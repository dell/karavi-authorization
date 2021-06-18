// Copyright © 2021 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/trace"
)

// PowerScaleSystem holds a reverse proxy and utilites for a PowerScale storage system.
type PowerScaleSystem struct {
	SystemEntry
	log *logrus.Entry
	rp  *httputil.ReverseProxy
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

// UpdateSystems updates the PowerScaleHandler via a SystemConfig
func (h *PowerScaleHandler) UpdateSystems(ctx context.Context, r io.Reader) error {
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
		if h.systems[k], err = buildPowerScaleSystem(ctx, v); err != nil {
			h.log.Errorf("proxy: powerscale failure: %+v", err)
		}
	}

	for _, id := range powerScaleSystems {
		h.log.Printf("Updated systems: %+v", id)
	}

	return nil
}

func buildPowerScaleSystem(ctx context.Context, e SystemEntry) (*PowerScaleSystem, error) {
	tgt, err := url.Parse(e.Endpoint)
	if err != nil {
		return nil, err
	}

	return &PowerScaleSystem{
		SystemEntry: e,
		log:         logrus.New().WithContext(context.Background()),
		rp:          httputil.NewSingleHostReverseProxy(tgt),
	}, nil
}

func (h *PowerScaleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fwd := forwardedHeader(r)
	fwdFor := fwd["for"]

	ep, systemID := splitEndpointSystemID(fwdFor)
	h.log.Printf("Endpoint: %s, SystemID: %s", ep, systemID)
	r = r.WithContext(context.WithValue(r.Context(), web.SystemIDKey, systemID))

	v, ok := h.systems[systemID]
	if !ok {
		h.writeError(w, "system id not found", http.StatusBadGateway)
		return
	}

	// Add authentication headers.
	r.SetBasicAuth(v.User, v.Password)

	// Instrument the proxy
	attrs := trace.WithAttributes(label.String("powerscale.endpoint", ep), label.String("powerscale.systemid", systemID))
	opts := otelhttp.WithSpanOptions(attrs)
	proxyHandler := otelhttp.NewHandler(v.rp, "proxy", opts)

	mux := http.NewServeMux()
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

	// Request policy decision from OPA
	ans, err := decision.Can(func() decision.Query {
		return decision.Query{
			Host:   h.opaHost,
			Policy: "/karavi/authz/powerscale/url",
			Input: map[string]interface{}{
				"method": r.Method,
				"url":    r.URL.Path,
			},
		}
	})
	if err != nil {
		log.Printf("opa: %v", err)
		h.writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var resp struct {
		Result struct {
			Allow bool `json:"allow"`
		} `json:"result"`
	}
	err = json.NewDecoder(bytes.NewReader(ans)).Decode(&resp)
	if err != nil {
		log.Printf("decode json: %q: %v", string(ans), err)
		h.writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !resp.Result.Allow {
		log.Println("Request denied")
		h.writeError(w, "request denied for path", http.StatusNotFound)
		return
	}

	mux.ServeHTTP(w, r)
}

func (h *PowerScaleHandler) spoofLoginRequest(w http.ResponseWriter, r *http.Request) {
	_, span := trace.SpanFromContext(r.Context()).Tracer().Start(r.Context(), "spoofLoginRequest")
	defer span.End()
	_, err := w.Write([]byte("hellofromkaravi"))
	if err != nil {
		h.log.Printf("failed to write response: %v", err)
	}
}

// TODO(ian): This will need to be updated to return errors in a format expected
// by the powerscale client. Currently this is just the #writeError function that
// was written for the powerflex system.
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
		log.Println("Failed to encode error response", err)
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
	}
}

func (s *PowerScaleSystem) volumeCreateHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := trace.SpanFromContext(r.Context()).Tracer().Start(r.Context(), "volumeCreateHandler")
		defer span.End()

		var systemID string
		if v := r.Context().Value(web.SystemIDKey); v != nil {
			var ok bool
			if systemID, ok = v.(string); !ok {
				//TODO(aaron): update writeError for powerscale
				writeError(w, "powerscale", http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}

		isiPath := strings.TrimPrefix(filepath.Dir(r.URL.Path), "/namespace")

		// Read the body.
		// The body is nil but we use the resulting io.ReadCloser to reset the request later on.
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeError(w, "powerscale", "failed to read body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		// Get the remote host address.
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			writeError(w, "powerscale", "failed to parse remote host", http.StatusInternalServerError)
			return
		}
		s.log.Printf("RemoteAddr: %s", host)

		pvName := r.Header.Get(HeaderPVName)
		log.Printf("pvName: %s", pvName)
		// Update metrics counter for volumes requested.
		//volReqCount.Add(pvName, 1)

		// Ask OPA to make a decision

		jwtGroup := r.Context().Value(web.JWTTenantName)
		group, ok := jwtGroup.(string)
		if !ok {
			writeError(w, "powerscale", "incorrect type for JWT group", http.StatusInternalServerError)
			return
		}

		jwtValue := r.Context().Value(web.JWTKey)
		jwtToken, ok := jwtValue.(token.Token)
		if !ok {
			writeError(w, "powerscale", "incorrect type for JWT token", http.StatusInternalServerError)
			return
		}

		claims, err := jwtToken.Claims()
		if err != nil {
			writeError(w, "powerscale", "decoding token claims", http.StatusInternalServerError)
			return
		}

		s.log.Println("Asking OPA...")
		// Request policy decision from OPA
		// The driver does not send the volume request size so we set the volumeSizeInKb to 0
		ans, err := decision.Can(func() decision.Query {
			return decision.Query{
				Host:   opaHost,
				Policy: "/karavi/volumes/powerscale/create",
				Input: map[string]interface{}{
					"claims":          claims,
					"request":         map[string]interface{}{"volumeSizeInKb": 0},
					"storagepool":     isiPath,
					"storagesystemid": systemID,
					"systemtype":      "powerscale",
				},
			}
		})
		var opaResp CreateOPAResponse
		err = json.NewDecoder(bytes.NewReader(ans)).Decode(&opaResp)
		if err != nil {
			s.log.Printf("decoding opa response: %+v", err)
			writeError(w, "powerscale", "decoding opa request body", http.StatusInternalServerError)
			return
		}
		log.Printf("OPA Response: %+v", opaResp)
		if resp := opaResp.Result; !resp.Allow {
			reason := strings.Join(opaResp.Result.Deny, ",")
			s.log.Printf("request denied: %v", reason)
			writeError(w, "powerscale", fmt.Sprintf("request denied: %v", reason), http.StatusBadRequest)
			return
		}

		// In the scenario where multiple roles are allowing
		// this request, choose the one with the most quota.
		var maxQuotaInKb int
		for _, quota := range opaResp.Result.PermittedRoles {
			if quota >= maxQuotaInKb {
				maxQuotaInKb = quota
			}
		}

		// At this point, the request has been approved.
		// The driver does not send the volume request size so we set the Capacity to 0 to always approve the quota
		// TODO(aaron): Will PowerScale SmartQuota handle this for us?
		qr := quota.Request{
			SystemType:    "powerscale",
			SystemID:      systemID,
			StoragePoolID: isiPath,
			Group:         group,
			VolumeName:    pvName,
			Capacity:      "0",
		}

		s.log.Println("Approving request...")
		// Ask our quota enforcer if it approves the request.
		ok, err = enf.ApproveRequest(ctx, qr, int64(maxQuotaInKb))
		if err != nil {
			s.log.Printf("failed to approve request: %+v", err)
			writeError(w, "powerscale", "failed to approve request", http.StatusInternalServerError)
			return
		}
		if !ok {
			s.log.Println("request was not approved")
			writeError(w, "powerscale", "request denied: not enough quota", http.StatusInsufficientStorage)
			return
		}

		// At this point, the request has been approved.

		// Reset the original request
		err = r.Body.Close()
		if err != nil {
			s.log.Printf("Failed to close original request body: %v", err)
		}
		r.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		sw := &web.StatusWriter{
			ResponseWriter: w,
		}

		s.log.Println("Proxying request...")
		// Proxy the request to the backend powerflex.
		r = r.WithContext(ctx)
		next.ServeHTTP(sw, r)

		// TODO(ian): Determine if when the approved volume fails the volume is
		// cleaned up (releasing capacity).
		log.Printf("Resp: Code: %d", sw.Status)
		switch sw.Status {
		case http.StatusOK:
			log.Println("Publish created")
			ok, err := enf.PublishCreated(r.Context(), qr)
			if err != nil {
				log.Printf("publish failed: %+v", err)
				return
			}
			log.Println("Result of publish:", ok)
		default:
			log.Println("Non 200 response, nothing to publish")
		}
	})
}

func (s *PowerScaleSystem) volumeDeleteHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	})
}
