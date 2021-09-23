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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"karavi-authorization/internal/decision"
	"karavi-authorization/internal/powerflex"
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
	"time"

	types "github.com/dell/goscaleio/types/v1"

	"github.com/dell/goscaleio"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/trace"
)

const (
	// HeaderPVName is the header key for k8s persistent volume
	HeaderPVName = "x-csi-pv-name"
	// HeaderPVClaimName is the header key for the k8s persistent volume claim
	HeaderPVClaimName = "x-csi-pv-claimname"
	// HeaderPVNamespace is the header key for the k8s persistent volume namespace
	HeaderPVNamespace = "x-csi-pv-namespace"
)

// System holds a reverse proxy and utilites for a PowerFlex storage system
type System struct {
	SystemEntry
	log *logrus.Entry
	rp  *httputil.ReverseProxy
	tk  interface {
		GetToken(context.Context) (string, error)
	}
	spc *powerflex.StoragePoolCache
}

// PowerFlexHandler is the proxy handler for PowerFlex systems
type PowerFlexHandler struct {
	log      *logrus.Entry
	mu       sync.Mutex // guards systems map
	systems  map[string]*System
	enforcer *quota.RedisEnforcement
	opaHost  string
}

// NewPowerFlexHandler returns a new PowerFlexHandler
func NewPowerFlexHandler(log *logrus.Entry, enforcer *quota.RedisEnforcement, opaHost string) *PowerFlexHandler {
	return &PowerFlexHandler{
		log:      log,
		systems:  make(map[string]*System),
		enforcer: enforcer,
		opaHost:  opaHost,
	}
}

// UpdateSystems updates the PowerFlexHandler via a SystemConfig
func (h *PowerFlexHandler) UpdateSystems(ctx context.Context, r io.Reader, log *logrus.Entry) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var updated SystemConfig
	if err := json.NewDecoder(r).Decode(&updated); err != nil {
		return err
	}

	powerFlexSystems := updated["powerflex"]

	// Remove systems
	for k := range h.systems {
		if _, ok := powerFlexSystems[k]; !ok {
			// Removed
			delete(h.systems, k)
		}
	}
	// Update systems
	for k, v := range powerFlexSystems {
		var err error
		if h.systems[k], err = buildSystem(ctx, v, log); err != nil {
			h.log.WithError(err).Error("building powerflex system")
		}
	}

	for _, arr := range updated {
		for id := range arr {
			h.log.WithField("updated_systems", id).Debug()
		}
	}

	return nil
}

func buildSystem(ctx context.Context, e SystemEntry, log *logrus.Entry) (*System, error) {
	tgt, err := url.Parse(e.Endpoint)
	if err != nil {
		return nil, err
	}
	c, err := goscaleio.NewClientWithArgs(tgt.String(), "", true, false)
	if err != nil {
		return nil, err
	}

	spc, err := powerflex.NewStoragePoolCache(c, 100)
	if err != nil {
		return nil, err
	}

	tk := powerflex.NewTokenGetter(powerflex.Config{
		PowerFlexClient:      c,
		TokenRefreshInterval: 5 * time.Minute,
		ConfigConnect: &goscaleio.ConfigConnect{
			Endpoint: e.Endpoint,
			Username: e.User,
			Password: e.Password,
		},
		Logger: log,
	})
	// TODO(ian): How do we ensure this gets cleaned up?
	go func() {
		err := tk.Start(ctx)
		if err != nil {
			log.Printf("token cache stopped for %s: %v", e.Endpoint, err)
			log.WithError(err).WithField("endpoint", e.Endpoint).Error("token cached stopped")
		}
	}()

	return &System{
		SystemEntry: e,
		log:         log,
		rp:          httputil.NewSingleHostReverseProxy(tgt),
		spc:         spc,
		tk:          tk,
	}, nil
}

func splitEndpointSystemID(s string) (string, string) {
	v := strings.Split(s, ";")
	if len(v) == 1 {
		return v[0], ""
	}
	return v[0], v[1]
}

func (h *PowerFlexHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fwd := forwardedHeader(r)
	fwdFor := fwd["for"]

	ep, systemID := splitEndpointSystemID(fwdFor)
	h.log.WithFields(logrus.Fields{
		"endpoint":  ep,
		"system_id": systemID,
	}).Debug("Serving request")
	r = r.WithContext(context.WithValue(r.Context(), web.SystemIDKey, systemID))
	r = r.WithContext(context.WithValue(r.Context(), web.SystemIDKey, systemID))

	v, ok := h.systems[systemID]
	if !ok {
		writeError(w, "powerflex", "system id not found", http.StatusBadGateway, h.log)
		return
	}

	// Use the authenticated session.
	token, err := v.tk.GetToken(r.Context())
	if err != nil {
		writeError(w, "powerflex", "failed to authenticate", http.StatusUnauthorized, h.log)
		return
	}
	r.SetBasicAuth("", token)

	// Instrument the proxy
	attrs := trace.WithAttributes(label.String("powerflex.endpoint", ep), label.String("powerflex.systemid", systemID))
	opts := otelhttp.WithSpanOptions(attrs)
	proxyHandler := otelhttp.NewHandler(v.rp, "proxy", opts)

	// TODO(ian): Probably shouldn't be building a servemux all the time :)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login/", h.spoofLoginRequest)
	mux.Handle("/api/types/Volume/instances/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/action/queryIdByKey/"):
			proxyHandler.ServeHTTP(w, r)
		default:
			v.volumeCreateHandler(proxyHandler, h.enforcer, h.opaHost).ServeHTTP(w, r)
		}
	}))
	mux.Handle("/api/instances/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/action/removeVolume/"):
			v.volumeDeleteHandler(proxyHandler, h.enforcer, h.opaHost).ServeHTTP(w, r)
		case strings.HasSuffix(r.URL.Path, "/action/addMappedSdc/"):
			v.volumeMapHandler(proxyHandler, h.enforcer, h.opaHost).ServeHTTP(w, r)
		case strings.HasSuffix(r.URL.Path, "/action/removeMappedSdc/"):
			v.volumeUnmapHandler(proxyHandler, h.enforcer, h.opaHost).ServeHTTP(w, r)
		default:
			proxyHandler.ServeHTTP(w, r)
		}
	}))
	mux.Handle("/", proxyHandler)

	// Request policy decision from OPA
	ans, err := decision.Can(func() decision.Query {
		return decision.Query{
			Host:   h.opaHost,
			Policy: "/karavi/authz/url",
			Input: map[string]interface{}{
				"method": r.Method,
				"url":    r.URL.Path,
			},
		}
	})
	if err != nil {
		h.log.WithError(err).Error("requesting policy decision from OPA")
		writeError(w, "powerflex", err.Error(), http.StatusInternalServerError, h.log)
		return
	}
	var resp struct {
		Result struct {
			Allow bool `json:"allow"`
		} `json:"result"`
	}
	err = json.NewDecoder(bytes.NewReader(ans)).Decode(&resp)
	if err != nil {
		h.log.WithError(err).WithField("opa_policy_decision", string(ans)).Error("decoding json")
		writeError(w, "powerflex", err.Error(), http.StatusInternalServerError, h.log)
		return
	}
	if !resp.Result.Allow {
		h.log.Debug("Request denied")
		writeError(w, "powerflex", "request denied for path", http.StatusNotFound, h.log)
		return
	}

	mux.ServeHTTP(w, r)
}

func (h *PowerFlexHandler) spoofLoginRequest(w http.ResponseWriter, r *http.Request) {
	_, span := trace.SpanFromContext(r.Context()).Tracer().Start(r.Context(), "spoofLoginRequest")
	defer span.End()
	_, err := w.Write([]byte("hellofromkaravi"))
	if err != nil {
		h.log.WithError(err).Error("writing spoofed login response")
	}
}

func writeError(w http.ResponseWriter, storage string, msg string, code int, log *logrus.Entry) {
	log.WithFields(logrus.Fields{
		"storage": storage,
		"code":    code,
		"message": msg,
	}).Debug("proxy: writing error")
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
		log.WithError(err).Error("encoding error response")
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
	}
}

func (s *System) volumeCreateHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := trace.SpanFromContext(r.Context()).Tracer().Start(r.Context(), "volumeCreateHandler")
		defer span.End()

		var systemID string
		if v := r.Context().Value(web.SystemIDKey); v != nil {
			var ok bool
			if systemID, ok = v.(string); !ok {
				writeError(w, "powerflex", http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError, s.log)
				return
			}
		}

		// Read the body.
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeError(w, "powerflex", "failed to read body", http.StatusInternalServerError, s.log)
			return
		}
		defer r.Body.Close()

		// Decode the body into a known structure.
		body := struct {
			VolumeSize     int64
			VolumeSizeInKb string `json:"volumeSizeInKb"`
			StoragePoolID  string `json:"storagePoolId"`
		}{}
		err = json.NewDecoder(bytes.NewBuffer(b)).Decode(&body)
		if err != nil {
			s.log.WithError(err).Error("proxy: decoding create volume request")
			writeError(w, "powerflex", "failed to extract cap data", http.StatusBadRequest, s.log)
			return
		}
		body.VolumeSize, err = strconv.ParseInt(body.VolumeSizeInKb, 0, 64)
		if err != nil {
			writeError(w, "powerflex", "failed to parse capacity", http.StatusBadRequest, s.log)
			return
		}

		// Convert the StoragePoolID into more friendly Name.
		spName, err := s.spc.GetStoragePoolNameByID(ctx, body.StoragePoolID)
		if err != nil {
			writeError(w, "powerflex", "failed to query pool name from id", http.StatusBadRequest, s.log)
			return
		}
		s.log.WithFields(logrus.Fields{
			"storage_pool_name": spName,
			"storage_pool_id":   body.StoragePoolID,
		}).Debug()

		// Get the remote host address.
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			writeError(w, "powerflex", "failed to parse remote host", http.StatusInternalServerError, s.log)
			return
		}
		s.log.WithField("remote_address", host).Debug()

		pvName := r.Header.Get(HeaderPVName)
		// Update metrics counter for volumes requested.
		//volReqCount.Add(pvName, 1)

		// Ask OPA to make a decision
		var requestBody map[string]json.RawMessage
		err = json.NewDecoder(bytes.NewReader(b)).Decode(&requestBody)
		if err != nil {
			writeError(w, "powerflex", "decoding request body", http.StatusInternalServerError, s.log)
			return
		}

		jwtGroup := r.Context().Value(web.JWTTenantName)
		group, ok := jwtGroup.(string)
		if !ok {
			writeError(w, "powerflex", "incorrect type for JWT group", http.StatusInternalServerError, s.log)
			return
		}

		jwtValue := r.Context().Value(web.JWTKey)
		jwtToken, ok := jwtValue.(token.Token)
		if !ok {
			writeError(w, "powerflex", "incorrect type for JWT token", http.StatusInternalServerError, s.log)
			return
		}

		claims, err := jwtToken.Claims()
		if err != nil {
			writeError(w, "powerflex", "decoding token claims", http.StatusInternalServerError, s.log)
			return
		}

		s.log.Debugln("Asking OPA...")
		// Request policy decision from OPA
		ans, err := decision.Can(func() decision.Query {
			return decision.Query{
				Host: opaHost,
				// TODO(ian): This will need to be namespaced under "powerflex".
				Policy: "/karavi/volumes/create",
				Input: map[string]interface{}{
					"claims":          claims,
					"request":         requestBody,
					"storagepool":     spName,
					"storagesystemid": systemID,
					"systemtype":      "powerflex",
				},
			}
		})
		var opaResp CreateOPAResponse
		err = json.NewDecoder(bytes.NewReader(ans)).Decode(&opaResp)
		if err != nil {
			s.log.WithError(err).Error("decoding opa response")
			writeError(w, "powerflex", "decoding opa request body", http.StatusInternalServerError, s.log)
			return
		}
		s.log.WithField("opa_response", opaResp).Debug()
		if resp := opaResp.Result; !resp.Allow {
			reason := strings.Join(opaResp.Result.Deny, ",")
			s.log.WithField("reason", reason).Debug("request denied")
			writeError(w, "powerflex", fmt.Sprintf("request denied: %v", reason), http.StatusBadRequest, s.log)
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
		qr := quota.Request{
			SystemType:    "powerflex",
			SystemID:      systemID,
			StoragePoolID: spName,
			Group:         group,
			VolumeName:    pvName,
			Capacity:      body.VolumeSizeInKb,
		}

		s.log.Debugln("Approving request...")
		// Ask our quota enforcer if it approves the request.
		ok, err = enf.ApproveRequest(ctx, qr, int64(maxQuotaInKb))
		if err != nil {
			s.log.WithError(err).Error("approving request")
			writeError(w, "powerflex", "failed to approve request", http.StatusInternalServerError, s.log)
			return
		}
		if !ok {
			s.log.Debugln("request was not approved")
			writeError(w, "powerflex", "request denied: not enough quota", http.StatusInsufficientStorage, s.log)
			return
		}

		// At this point, the request has been approved.

		// Reset the original request
		err = r.Body.Close()
		if err != nil {
			s.log.WithError(err).Error("closing original request body")
		}
		r.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		sw := &web.StatusWriter{
			ResponseWriter: w,
		}

		s.log.Debugln("Proxying request...")
		// Proxy the request to the backend powerflex.
		r = r.WithContext(ctx)
		next.ServeHTTP(sw, r)

		// TODO(ian): Determine if when the approved volume fails the volume is
		// cleaned up (releasing capacity).
		s.log.WithFields(logrus.Fields{
			"Response code": sw.Status,
		}).Debug()
		switch sw.Status {
		case http.StatusOK:
			s.log.Debugln("Publish created")
			ok, err := enf.PublishCreated(r.Context(), qr)
			if err != nil {
				s.log.WithError(err).Error("publishing volume created")
				return
			}
			s.log.WithField("publish_result", ok).Debug("Publish volume created")
		default:
			s.log.Debugln("Non 200 response, nothing to publish")
		}
	})
}

func (s *System) volumeDeleteHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := trace.SpanFromContext(r.Context()).Tracer().Start(r.Context(), "volumeDeleteHandler")
		defer span.End()

		var systemID string
		if v := r.Context().Value(web.SystemIDKey); v != nil {
			var ok bool
			if systemID, ok = v.(string); !ok {
				writeError(w, "powerflex", http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError, s.log)
				return
			}
		}

		// Extract the volume ID from the request URI in order to get the
		// the name.
		var id string
		z := strings.SplitN(r.URL.Path, "/", 5)
		if len(z) > 3 {
			id = z[3]
		}
		pvName, err := func() (*types.Volume, error) {
			c, err := goscaleio.NewClientWithArgs(s.Endpoint, "", true, false)
			if err != nil {
				return nil, err
			}
			token, err := s.tk.GetToken(ctx)
			c.SetToken(token)

			id = strings.TrimPrefix(id, "Volume::")
			vols, err := c.GetVolume("", id, "", "", false)
			if err != nil {
				return nil, err
			}

			if len(vols) == 0 {
				return nil, errors.New("No volume")
			}

			return vols[0], nil
		}()
		if err != nil {
			s.log.WithError(err).Error("querying volume name by id")
			writeError(w, "powerflex", "query volume name by volid", http.StatusInternalServerError, s.log)
			return
		}

		spName, err := s.spc.GetStoragePoolNameByID(ctx, pvName.StoragePoolID)
		if err != nil {
			writeError(w, "powerflex", "failed to query pool name from id", http.StatusBadRequest, s.log)
			return
		}

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeError(w, "powerflex", "failed to read body", http.StatusInternalServerError, s.log)
			return
		}
		defer r.Body.Close()

		jwtValue := r.Context().Value(web.JWTKey)
		jwtToken, ok := jwtValue.(token.Token)
		if !ok {
			writeError(w, "powerflex", "incorrect type for JWT token", http.StatusInternalServerError, s.log)
			return
		}

		claims, err := jwtToken.Claims()
		if err != nil {
			writeError(w, "powerflex", "decoding token claims", http.StatusInternalServerError, s.log)
			return
		}

		var requestBody map[string]json.RawMessage
		err = json.NewDecoder(bytes.NewReader(b)).Decode(&requestBody)
		if err != nil {
			writeError(w, "powerflex", "decoding request body", http.StatusInternalServerError, s.log)
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

		var opaResp OPAResponse
		err = json.NewDecoder(bytes.NewReader(ans)).Decode(&opaResp)
		if err != nil {
			writeError(w, "powerflex", "decoding opa request body", http.StatusInternalServerError, s.log)
			return
		}
		s.log.WithField("opa_response", string(ans)).Debug("OPA Response")
		if resp := opaResp.Result; !resp.Response.Allowed {
			switch {
			case resp.Claims.Group == "":
				writeError(w, "powerflex", "invalid token", http.StatusUnauthorized, s.log)
			default:
				writeError(w, "powerflex", fmt.Sprintf("request denied: %v", resp.Response.Status.Reason), http.StatusBadRequest, s.log)
			}
			return
		}

		qr := quota.Request{
			SystemType:    "powerflex",
			SystemID:      systemID,
			StoragePoolID: spName,
			Group:         opaResp.Result.Claims.Group,
			VolumeName:    pvName.Name,
		}
		ok, err = enf.DeleteRequest(r.Context(), qr)
		if err != nil {
			writeError(w, "powerflex", "delete request failed", http.StatusInternalServerError, s.log)
			return
		}
		if !ok {
			writeError(w, "powerflex", "request denied", http.StatusForbidden, s.log)
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

		s.log.WithFields(logrus.Fields{
			"Response code": sw.Status,
		}).Debug()
		switch sw.Status {
		case http.StatusOK:
			s.log.Debugln("Publish deleted")
			ok, err := enf.PublishDeleted(r.Context(), qr)
			if err != nil {
				s.log.WithError(err).Error("publishing volume deleted")
				return
			}
			s.log.WithField("publish_result", ok).Debug("Publish volume created")
		default:
			s.log.Debugln("Non 200 response, nothing to publish")
		}
	})
}

func (s *System) volumeMapHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := trace.SpanFromContext(r.Context()).Tracer().Start(r.Context(), "volumeMapHandler")
		defer span.End()

		var systemID string
		if v := r.Context().Value(web.SystemIDKey); v != nil {
			var ok bool
			if systemID, ok = v.(string); !ok {
				writeError(w, "powerflex", http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError, s.log)
				return
			}
		}

		var id string
		z := strings.SplitN(r.URL.Path, "/", 5)
		if len(z) > 3 {
			id = z[3]
		} else {
			writeError(w, "powerflex", "incomplete request", http.StatusInternalServerError, s.log)
			return
		}
		pvName, err := func() (*types.Volume, error) {
			c, err := goscaleio.NewClientWithArgs(s.Endpoint, "", true, false)
			if err != nil {
				return nil, err
			}
			token, err := s.tk.GetToken(ctx)
			c.SetToken(token)

			id = strings.TrimPrefix(id, "Volume::")
			s.log.WithField("volume", id).Debug("Looking for volume to map")
			vols, err := c.GetVolume("", id, "", "", false)
			s.log.WithField("volumes", vols).Debug("Found volumes")
			if err != nil {
				return nil, err
			}

			if len(vols) == 0 {
				return nil, errors.New("No volume")
			}

			return vols[0], nil
		}()
		if err != nil {
			writeError(w, "powerflex", fmt.Sprintf("query name by volid: %v", err), http.StatusInternalServerError, s.log)
			return
		}

		spName, err := s.spc.GetStoragePoolNameByID(ctx, pvName.StoragePoolID)
		if err != nil {
			writeError(w, "powerflex", "failed to query pool name from id", http.StatusBadRequest, s.log)
			return
		}

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeError(w, "powerflex", "failed to read body", http.StatusInternalServerError, s.log)
			return
		}
		defer r.Body.Close()

		jwtValue := r.Context().Value(web.JWTKey)
		jwtToken, ok := jwtValue.(token.Token)
		if !ok {
			writeError(w, "powerflex", "incorrect type for JWT token", http.StatusInternalServerError, s.log)
			return
		}

		claims, err := jwtToken.Claims()
		if err != nil {
			writeError(w, "powerflex", "decoding token claims", http.StatusInternalServerError, s.log)
			return
		}

		var requestBody map[string]json.RawMessage
		err = json.NewDecoder(bytes.NewReader(b)).Decode(&requestBody)
		if err != nil {
			s.log.WithError(err).Error("decoding request body")
			writeError(w, "powerflex", "decoding request body", http.StatusInternalServerError, s.log)
			return
		}
		// Request policy decision from OPA
		ans, err := decision.Can(func() decision.Query {
			return decision.Query{
				Host:   opaHost,
				Policy: "/karavi/volumes/map",
				Input: map[string]interface{}{
					"claims": claims,
				},
			}
		})

		var opaResp OPAResponse
		err = json.NewDecoder(bytes.NewReader(ans)).Decode(&opaResp)
		if err != nil {
			s.log.Printf("decoding opa response: %+v", err)
			writeError(w, "powerflex", "decoding opa request body", http.StatusInternalServerError, s.log)
			return
		}
		s.log.WithField("opa_response", opaResp).Debug()
		if resp := opaResp.Result; !resp.Response.Allowed {
			s.log.Printf("request denied: %v", resp.Response.Status.Reason)
			writeError(w, "powerflex", fmt.Sprintf("request denied: %v", resp.Response.Status.Reason), http.StatusBadRequest, s.log)
			return
		}

		qr := quota.Request{
			SystemType:    "powerflex",
			SystemID:      systemID,
			StoragePoolID: spName,
			Group:         opaResp.Result.Claims.Group,
			VolumeName:    pvName.Name,
		}
		ok, err = enf.ValidateOwnership(ctx, qr)
		if err != nil {
			writeError(w, "powerflex", "map request failed", http.StatusInternalServerError, s.log)
			return
		}
		if !ok {
			writeError(w, "powerflex", "map denied", http.StatusForbidden, s.log)
			return
		}

		// Reset the original request
		r.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

func (s *System) volumeUnmapHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := trace.SpanFromContext(r.Context()).Tracer().Start(r.Context(), "volumeUnmapHandler")
		defer span.End()

		var systemID string
		if v := r.Context().Value(web.SystemIDKey); v != nil {
			var ok bool
			if systemID, ok = v.(string); !ok {
				writeError(w, "powerflex", http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError, s.log)
				return
			}
		}

		var id string
		z := strings.SplitN(r.URL.Path, "/", 5)
		if len(z) > 3 {
			id = z[3]
		} else {
			writeError(w, "powerflex", "incomplete request", http.StatusInternalServerError, s.log)
			return
		}
		pvName, err := func() (*types.Volume, error) {
			c, err := goscaleio.NewClientWithArgs(s.Endpoint, "", true, false)
			if err != nil {
				return nil, err
			}
			token, err := s.tk.GetToken(ctx)
			c.SetToken(token)

			id = strings.TrimPrefix(id, "Volume::")
			s.log.WithField("volume", id).Debug("Looking for volume to map")
			vols, err := c.GetVolume("", id, "", "", false)
			s.log.WithField("volumes", vols).Debug("Found volumes")
			if err != nil {
				return nil, err
			}

			if len(vols) == 0 {
				return nil, errors.New("No volume")
			}

			return vols[0], nil
		}()
		if err != nil {
			writeError(w, "powerflex", fmt.Sprintf("query name by volid: %v", err), http.StatusInternalServerError, s.log)
			return
		}

		spName, err := s.spc.GetStoragePoolNameByID(ctx, pvName.StoragePoolID)
		if err != nil {
			writeError(w, "powerflex", "failed to query pool name from id", http.StatusBadRequest, s.log)
			return
		}

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeError(w, "powerflex", "failed to read body", http.StatusInternalServerError, s.log)
			return
		}
		defer r.Body.Close()

		jwtValue := r.Context().Value(web.JWTKey)
		jwtToken, ok := jwtValue.(token.Token)
		if !ok {
			writeError(w, "powerflex", "incorrect type for JWT token", http.StatusInternalServerError, s.log)
			return
		}

		claims, err := jwtToken.Claims()
		if err != nil {
			writeError(w, "powerflex", "decoding token claims", http.StatusInternalServerError, s.log)
			return
		}

		var requestBody map[string]json.RawMessage
		err = json.NewDecoder(bytes.NewReader(b)).Decode(&requestBody)
		if err != nil {
			s.log.WithError(err).Error("decoding request body")
			writeError(w, "powerflex", "decoding request body", http.StatusInternalServerError, s.log)
			return
		}
		// Request policy decision from OPA
		ans, err := decision.Can(func() decision.Query {
			return decision.Query{
				Host:   opaHost,
				Policy: "/karavi/volumes/unmap",
				Input: map[string]interface{}{
					"claims": claims,
				},
			}
		})

		var opaResp OPAResponse
		err = json.NewDecoder(bytes.NewReader(ans)).Decode(&opaResp)
		if err != nil {
			s.log.Printf("decoding opa request body: %+v", err)
			writeError(w, "powerflex", "decoding opa request body", http.StatusInternalServerError, s.log)
			return
		}
		s.log.WithField("opa_response", opaResp).Debug()
		if resp := opaResp.Result; !resp.Response.Allowed {
			switch {
			case resp.Claims.Group == "":
				writeError(w, "powerflex", "invalid token", http.StatusUnauthorized, s.log)
			default:
				writeError(w, "powerflex", fmt.Sprintf("request denied: %v", resp.Response.Status.Reason), http.StatusBadRequest, s.log)
			}
			return
		}

		qr := quota.Request{
			SystemType:    "powerflex",
			SystemID:      systemID,
			StoragePoolID: spName,
			Group:         opaResp.Result.Claims.Group,
			VolumeName:    pvName.Name,
		}
		ok, err = enf.ValidateOwnership(ctx, qr)
		if err != nil {
			writeError(w, "powerflex", "unmap request failed", http.StatusInternalServerError, s.log)
			return
		}
		if !ok {
			writeError(w, "powerflex", "unmap denied", http.StatusForbidden, s.log)
			return
		}

		// Reset the original request
		r.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

// OPAResponse is the respone payload from OPA
type OPAResponse struct {
	Result struct {
		Response struct {
			Allowed bool `json:"allowed"`
			Status  struct {
				Reason string `json:"reason"`
			} `json:"status"`
		} `json:"response"`
		Claims struct {
			Group string `json:"group"`
		} `json:"claims"`
		Quota int64 `json:"quota"`
	} `json:"result"`
}

// CreateOPAResponse is the response payload from OPA
// when performing a volume create operation.
// The permitted_roles field shall contain a map of
// permitted role names to the appropriate storage
// pool quota.
type CreateOPAResponse struct {
	Result struct {
		Allow          bool           `json:"allow"`
		Deny           []string       `json:"deny"`
		PermittedRoles map[string]int `json:"permitted_roles"`
	} `json:"result"`
}
