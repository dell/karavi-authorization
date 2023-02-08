// Copyright © 2021-2022 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"karavi-authorization/internal/decision"
	"karavi-authorization/internal/quota"
	"karavi-authorization/internal/token"
	"karavi-authorization/internal/web"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// PowerScaleSystem holds a reverse proxy and utilites for a PowerScale storage system.
type PowerScaleSystem struct {
	SystemEntry
	sessionCookie string
	csrfToken     string
	log           *logrus.Entry
	rp            *httputil.ReverseProxy
}

// PowerScaleHandler is the proxy handler for PowerScale systems.
type PowerScaleHandler struct {
	log      *logrus.Entry
	mu       sync.Mutex // guards systems map
	systems  map[string]*PowerScaleSystem
	enforcer *quota.RedisEnforcement
	opaHost  string
}

// NewPowerScaleHandler returns a new PowerScaleHandler.
func NewPowerScaleHandler(log *logrus.Entry, enforcer *quota.RedisEnforcement, opaHost string) *PowerScaleHandler {
	return &PowerScaleHandler{
		log:      log,
		systems:  make(map[string]*PowerScaleSystem),
		enforcer: enforcer,
		opaHost:  opaHost,
	}
}

// GetSystems returns the configured systems
func (h *PowerScaleHandler) GetSystems() map[string]*PowerScaleSystem {
	return h.systems
}

// UpdateSystems updates the PowerScaleHandler via a SystemConfig
func (h *PowerScaleHandler) UpdateSystems(ctx context.Context, r io.Reader, log *logrus.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.systems == nil {
		h.systems = make(map[string]*PowerScaleSystem)
	}

	var updated SystemConfig
	if err := json.NewDecoder(r).Decode(&updated); err != nil {
		return err
	}

	powerScaleSystems := updated["powerscale"]

	// Remove systems
	for k := range h.systems {
		if _, ok := powerScaleSystems[k]; !ok {
			// Removed
			delete(h.systems, k)
		}
	}
	// Update systems
	for k, v := range powerScaleSystems {
		var err error
		if h.systems[k], err = buildPowerScaleSystem(ctx, v, log); err != nil {
			h.log.WithError(err).Error("building powerscale system")
		}
	}

	for _, id := range powerScaleSystems {
		h.log.WithField("updated_systems", id).Debug()
	}

	return nil
}

func buildPowerScaleSystem(ctx context.Context, e SystemEntry, log *logrus.Entry) (*PowerScaleSystem, error) {
	tgt, err := url.Parse(e.Endpoint)
	if err != nil {
		return nil, err
	}

	return &PowerScaleSystem{
		SystemEntry: e,
		log:         log,
		rp:          httputil.NewSingleHostReverseProxy(tgt),
	}, nil
}

func (h *PowerScaleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fwd := ForwardedHeader(r)
	fwdFor := fwd["for"]

	ep, systemID := SplitEndpointSystemID(fwdFor)
	h.log.WithFields(logrus.Fields{
		"endpoint":  ep,
		"system_id": systemID,
	}).Debug("Serving request")
	r = r.WithContext(context.WithValue(r.Context(), web.SystemIDKey, systemID))

	v, ok := h.systems[systemID]
	if !ok {
		writeErrorPowerScale(w, "system id not found", http.StatusBadGateway, h.log)
		return
	}

	// Strip uneeded headers
	r.Header.Del("Cookie")
	r.Header.Del("X-Csrf-Token")
	r.Header.Del("Referer")
	r.Header.Del("Authorization")
	r.Header.Del("X-Forwarded-For")
	r.Header.Del("X-Forwarded-Host")
	r.Header.Del("X-Forwarded-Port")
	r.Header.Del("X-Forwarded-Proto")
	r.Header.Del("X-Forwarded-Server")

	host, err := url.Parse(v.Endpoint)
	if err != nil {
		writeErrorPowerScale(w, "cannot parse host header from system endpoint", http.StatusBadGateway, h.log)
		return
	}
	r.Host = host.Host

	// Add authentication headers.
	err = h.addSessionHeaders(r, v)
	if err != nil {
		h.log.Errorf("adding session headers: %v", err)
		writeErrorPowerScale(w, err.Error(), http.StatusInternalServerError, h.log)
		return
	}

	// Instrument the proxy
	attrs := trace.WithAttributes(attribute.String("powerscale.endpoint", ep), attribute.String("powerscale.systemid", systemID))
	opts := otelhttp.WithSpanOptions(attrs)
	proxyHandler := otelhttp.NewHandler(v.rp, "proxy", opts)

	mux := http.NewServeMux()
	mux.Handle("/session/1/session/", http.HandlerFunc(h.spoofSession))
	mux.Handle("/namespace/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			v.volumeCreateHandler(proxyHandler, h.enforcer, h.opaHost).ServeHTTP(w, r)
		case http.MethodDelete:
			v.volumeDeleteHandler(proxyHandler, h.enforcer, h.opaHost).ServeHTTP(w, r)
		default:
			proxyHandler.ServeHTTP(w, r)
		}
	}))
	mux.Handle("/", proxyHandler)

	// Save a copy of this request for debugging.
	requestDump, err := httputil.DumpRequest(r, true)
	if err != nil {
		h.log.Error(err)
	}

	h.log.Debug("Dumping request...")
	h.log.Debug(string(requestDump))

	mux.ServeHTTP(w, r)
}

func (h *PowerScaleHandler) spoofSession(w http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		h.log.Error("Could not read session request body")
		w.WriteHeader(http.StatusInternalServerError)
	}
	h.log.Infof("Spoofing session for %v request at %v: %v", r.Method, r.URL.RawPath, string(b))
	_, span := trace.SpanFromContext(r.Context()).TracerProvider().Tracer("").Start(r.Context(), "spoofSessionCheck")
	defer span.End()

	type sessionStatusResponseBody struct {
		Services        []string `json:"services"`
		TimeoutAbsolute int      `json:"timeout_absolute"`
		TimeoutInactive int      `json:"timeout_inactive"`
		Username        string   `json:"username"`
	}
	resp := sessionStatusResponseBody{
		Services:        []string{"platform", "namespace"},
		TimeoutAbsolute: 12345,
		TimeoutInactive: 900,
		Username:        "-",
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(resp)
		if err != nil {
			h.writeError(w, err.Error(), http.StatusInternalServerError)
		}
	case http.MethodPost:
		w.Header().Add("Set-Cookie", "isisessid=12345678-abcd-1234-abcd-1234567890ab;")
		w.Header().Add("Set-Cookie", "isicsrf=12345678-abcd-1234-abcd-1234567890ab;")
		w.WriteHeader(http.StatusCreated)
		err := json.NewEncoder(w).Encode(resp)
		if err != nil {
			h.writeError(w, err.Error(), http.StatusInternalServerError)
		}
	default:
		h.log.Errorf("unexpected http request method for spoofing session: %v", r.Method)
	}
}

func (h *PowerScaleHandler) writeError(w http.ResponseWriter, msg string, code int) {
	h.log.Printf("proxy: powerscale_handler: writing error:  %d: %s", code, msg)
	w.WriteHeader(code)
	errBody := struct {
		Code       int    `json:"errorCode"`
		StatusCode int    `json:"httpStatusCode"`
		Message    string `json:"message"`
	}{
		Code:       code,
		StatusCode: code,
		Message:    msg,
	}
	err := json.NewEncoder(w).Encode(&errBody)
	if err != nil {
		h.log.Println("Failed to encode error response", err)
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
	}
}

func (h *PowerScaleHandler) addSessionHeaders(r *http.Request, v *PowerScaleSystem) error {
	// Check if current session cookie is valid
	client := &http.Client{}
	sessionStatusReq, err := http.NewRequest("GET", v.Endpoint+"/session/1/session", nil)
	if err != nil {
		return fmt.Errorf("could not create request for session cookie status: %e", err)
	}
	sessionStatusReq.Header.Add("Cookie", v.sessionCookie)
	sessionStatusResp, err := client.Do(sessionStatusReq)
	if err != nil {
		return fmt.Errorf("error requesting session cookie status for PowerScale %v: %e", v.Endpoint, err)
	}
	sessionStatusRespBody, err := ioutil.ReadAll(sessionStatusResp.Body)
	if err != nil {
		return fmt.Errorf("error reading session status response body: %e", err)
	}
	h.log.Debugf("get session status response: (%v) %v", sessionStatusResp.StatusCode, string(sessionStatusRespBody))

	// If not valid, get a new session cookie
	if sessionStatusResp.StatusCode == http.StatusUnauthorized {
		h.log.Info("Authintication session is expired. Requesting a new session...")
		type newSessionRequestBody struct {
			Username string   `json:"username"`
			Password string   `json:"password"`
			Services []string `json:"services"`
		}
		req := newSessionRequestBody{
			Username: v.User,
			Password: v.Password,
			Services: []string{"platform", "namespace"},
		}
		reqBody, err := json.Marshal(req)
		if err != nil {
			return fmt.Errorf("failed to marshal session request body: %e", err)
		}
		h.log.Debugf("New session request body: %v", string(reqBody))
		newSessionResp, err := http.Post(v.Endpoint+"/session/1/session", "application/json", bytes.NewBuffer(reqBody))
		if err != nil {
			return fmt.Errorf("error requesting new session: %e", err)
		}
		defer newSessionResp.Body.Close()

		respBody, err := ioutil.ReadAll(newSessionResp.Body)
		if err != nil {
			return fmt.Errorf("reading response body from new session request: %e", err)
		}
		if newSessionResp.StatusCode != http.StatusCreated {
			return fmt.Errorf("in response when requesting session token: %v", string(respBody))
		}
		h.log.Debugf("New session response: (%v) %v", newSessionResp.StatusCode, string(respBody))

		headerRes := strings.Join(newSessionResp.Header.Values("Set-Cookie"), " ")

		startIndex, endIndex, matchStrLen := fetchValueIndexForKey(headerRes, "isisessid=", ";")
		v.sessionCookie = headerRes[startIndex : startIndex+matchStrLen+endIndex]
		if startIndex < 0 || endIndex < 0 {
			return fmt.Errorf("could not extract isisessid from new session response: %v", headerRes)
		}

		startIndex, endIndex, matchStrLen = fetchValueIndexForKey(headerRes, "isicsrf=", ";")
		v.csrfToken = headerRes[startIndex+matchStrLen : startIndex+matchStrLen+endIndex]
		if startIndex < 0 || endIndex < 0 {
			h.log.Errorf("Could not extract isisessid from new session response: %v", headerRes)
		}
	}

	// Add the session cookie to the request's headers
	r.Header.Add("Cookie", v.sessionCookie)
	h.log.Debugf("added session cookie to request header: %v", v.sessionCookie)
	r.Header.Add("X-CSRF-Token", v.csrfToken)
	h.log.Debugf("added CSRF token to request header: %v", v.csrfToken)

	// Add referrer header
	r.Header.Add("Referer", v.Endpoint)
	return nil
}

func fetchValueIndexForKey(l string, match string, sep string) (int, int, int) {

	if i := strings.Index(l, match); i != -1 {
		if j := strings.Index(l[i+len(match):], sep); j != -1 {
			return i, j, len(match)
		}
	}
	return -1, -1, len(match)
}

func (s *PowerScaleSystem) volumeCreateHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := trace.SpanFromContext(r.Context()).TracerProvider().Tracer("").Start(r.Context(), "volumeCreateHandler")
		defer span.End()

		// Read the body.
		// The body is nil but we use the resulting io.ReadCloser to reset the request later on.
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeErrorPowerScale(w, "failed to read body", http.StatusInternalServerError, s.log)
			return
		}
		defer r.Body.Close()

		// Get the remote host address.
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			writeErrorPowerScale(w, "failed to parse remote host", http.StatusInternalServerError, s.log)
			return
		}
		s.log.WithField("remote_address", host).Debug()

		pvName := r.Header.Get(HeaderPVName)
		s.log.WithField("pv_name", pvName).Debug()

		// Ask OPA to make a decision

		jwtGroup := r.Context().Value(web.JWTTenantName)
		_, ok := jwtGroup.(string)
		if !ok {
			writeErrorPowerScale(w, "incorrect type for JWT group", http.StatusInternalServerError, s.log)
			return
		}

		jwtValue := r.Context().Value(web.JWTKey)
		jwtToken, ok := jwtValue.(token.Token)
		if !ok {
			writeErrorPowerScale(w, "incorrect type for JWT token", http.StatusInternalServerError, s.log)
			return
		}

		claims, err := jwtToken.Claims()
		if err != nil {
			writeErrorPowerScale(w, "decoding token claims", http.StatusInternalServerError, s.log)
			return
		}

		s.log.Debugln("Asking OPA...")
		// Request policy decision from OPA
		// The driver does not send the volume request size so we set the volumeSizeInKb to 0
		ans, err := decision.Can(func() decision.Query {
			return decision.Query{
				Host:   opaHost,
				Policy: "/karavi/volumes/powerscale/create",
				Input: map[string]interface{}{
					"claims":  claims,
					"request": map[string]interface{}{"volumeSizeInKb": 0},
				},
			}
		})
		if err != nil {
			s.log.WithError(err).Error("asking OPA for volume create decision")
			writeErrorPowerScale(w, fmt.Sprintf("asking OPA for volume create decision: %v", err), http.StatusInternalServerError, s.log)
			return
		}

		var opaResp CreateOPAResponse
		err = json.NewDecoder(bytes.NewReader(ans)).Decode(&opaResp)
		if err != nil {
			s.log.Printf("decoding opa response: %+v", err)
			writeErrorPowerScale(w, "decoding opa request body", http.StatusInternalServerError, s.log)
			return
		}
		s.log.WithField("opa_response", opaResp).Debug()
		if resp := opaResp.Result; !resp.Allow {
			reason := strings.Join(opaResp.Result.Deny, ",")
			s.log.Printf("request denied: %v", reason)
			writeErrorPowerScale(w, fmt.Sprintf("request denied: %v", reason), http.StatusBadRequest, s.log)
			return
		}

		// Reset the original request
		err = r.Body.Close()
		if err != nil {
			s.log.WithError(err).Error("closing original request body")
		}
		r.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		sw := &web.StatusWriter{
			ResponseWriter: w,
		}

		s.log.Println("Proxying request...")
		// Proxy the request to the backend powerscale.
		r = r.WithContext(ctx)
		next.ServeHTTP(sw, r)
	})
}

func (s *PowerScaleSystem) volumeDeleteHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := trace.SpanFromContext(r.Context()).TracerProvider().Tracer("").Start(r.Context(), "volumeDeleteHandler")
		defer span.End()

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeErrorPowerScale(w, "failed to read body", http.StatusInternalServerError, s.log)
			return
		}
		defer r.Body.Close()

		jwtValue := r.Context().Value(web.JWTKey)
		jwtToken, ok := jwtValue.(token.Token)
		if !ok {
			writeErrorPowerScale(w, "incorrect type for JWT token", http.StatusInternalServerError, s.log)
			return
		}

		claims, err := jwtToken.Claims()
		if err != nil {
			writeErrorPowerScale(w, "decoding token claims", http.StatusInternalServerError, s.log)
			return
		}

		// Request policy decision from OPA
		ans, err := decision.Can(func() decision.Query {
			return decision.Query{
				Host:   opaHost,
				Policy: "/karavi/volumes/delete",
				Input: map[string]interface{}{
					"claims": claims,
				},
			}
		})
		if err != nil {
			s.log.WithError(err).Error("asking OPA for volume delete decision")
			writeErrorPowerScale(w, fmt.Sprintf("asking OPA for volume delete decision: %v", err), http.StatusInternalServerError, s.log)
			return
		}

		var opaResp OPAResponse
		err = json.NewDecoder(bytes.NewReader(ans)).Decode(&opaResp)
		if err != nil {
			writeErrorPowerScale(w, "decoding opa request body", http.StatusInternalServerError, s.log)
			return
		}
		s.log.WithField("opa_response", string(ans)).Debug()
		if resp := opaResp.Result; !resp.Response.Allowed {
			switch {
			case resp.Claims.Group == "":
				writeErrorPowerScale(w, "invalid token", http.StatusUnauthorized, s.log)
			default:
				writeErrorPowerScale(w, fmt.Sprintf("request denied: %v", resp.Response.Status.Reason), http.StatusBadRequest, s.log)
			}
			return
		}

		// Reset the original request
		err = r.Body.Close()
		if err != nil {
			s.log.WithError(err).Error("closing original request body")
		}
		r.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		sw := &web.StatusWriter{
			ResponseWriter: w,
		}
		r = r.WithContext(ctx)
		next.ServeHTTP(sw, r)
	})
}

// APIErr is the error format returned from PowerScale
type APIErr struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeErrorPowerScale(w http.ResponseWriter, msg string, code int, log *logrus.Entry) {
	log.Printf("proxy: powerscale_handler: writing error:  %d: %s", code, msg)
	w.WriteHeader(code)

	errBody := struct {
		Err []APIErr `json:"errors"`
	}{
		Err: []APIErr{
			{
				Code:    strconv.Itoa(code),
				Message: msg,
			},
		},
	}
	b, err := json.Marshal(errBody)
	if err == nil {
		log.Println(string(b))
	}

	err = json.NewEncoder(w).Encode(&errBody)
	if err != nil {
		log.Println("Failed to encode error response", err)
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
	}
}
