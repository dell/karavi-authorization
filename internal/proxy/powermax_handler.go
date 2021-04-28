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
	"io"
	"karavi-authorization/internal/decision"
	"karavi-authorization/internal/quota"
	"karavi-authorization/internal/web"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/trace"
)

// PowerMaxSystem holds a reverse proxy and utilites for a PowerMax storage system.
type PowerMaxSystem struct {
	SystemEntry
	log *logrus.Entry
	rp  *httputil.ReverseProxy
}

// PowerMaxHandler is the proxy handler for PowerMax systems.
type PowerMaxHandler struct {
	log      *logrus.Entry
	mu       sync.Mutex // guards systems map
	systems  map[string]*PowerMaxSystem
	enforcer *quota.RedisEnforcement
	opaHost  string
}

// NewPowerMaxHandler returns a new PowerMaxHandler.
func NewPowerMaxHandler(log *logrus.Entry, enforcer *quota.RedisEnforcement, opaHost string) *PowerMaxHandler {
	return &PowerMaxHandler{
		log:      log,
		systems:  make(map[string]*PowerMaxSystem),
		enforcer: enforcer,
		opaHost:  opaHost,
	}
}

// UpdateSystems updates the PowerMaxHandler via a SystemConfig
func (h *PowerMaxHandler) UpdateSystems(ctx context.Context, r io.Reader) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.systems == nil {
		h.systems = make(map[string]*PowerMaxSystem)
	}

	var updated SystemConfig
	if err := json.NewDecoder(r).Decode(&updated); err != nil {
		return err
	}

	powerMaxSystems := updated["powermax"]

	// Remove systems
	for k := range h.systems {
		if _, ok := powerMaxSystems[k]; !ok {
			// Removed
			delete(h.systems, k)
		}
	}
	// Update systems
	for k, v := range powerMaxSystems {
		var err error
		if h.systems[k], err = buildPowerMaxSystem(ctx, v); err != nil {
			h.log.Errorf("proxy: powermax failure: %+v", err)
		}
	}

	for _, id := range powerMaxSystems {
		h.log.Printf("Updated systems: %+v", id)
	}

	return nil
}

func buildPowerMaxSystem(ctx context.Context, e SystemEntry) (*PowerMaxSystem, error) {
	tgt, err := url.Parse(e.Endpoint)
	if err != nil {
		return nil, err
	}

	return &PowerMaxSystem{
		SystemEntry: e,
		log:         logrus.New().WithContext(context.Background()),
		rp:          httputil.NewSingleHostReverseProxy(tgt),
	}, nil
}

func (h *PowerMaxHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fwd := forwardedHeader(r)
	fwdFor := fwd["for"]

	ep, systemID := splitEndpointSystemID(fwdFor)
	h.log.Printf("Endpoint: %s, SystemID: %s", ep, systemID)
	r = r.WithContext(context.WithValue(r.Context(), web.SystemIDKey, systemID))

	v, ok := h.systems[systemID]
	if !ok {
		writeError(w, "system id not found", http.StatusBadGateway)
		return
	}

	// Add authentication headers.
	r.SetBasicAuth(v.User, v.Password)

	// Instrument the proxy
	attrs := trace.WithAttributes(label.String("powermax.endpoint", ep), label.String("powermax.systemid", systemID))
	opts := otelhttp.WithSpanOptions(attrs)
	proxyHandler := otelhttp.NewHandler(v.rp, "proxy", opts)

	mux := http.NewServeMux()
	mux.Handle("/", proxyHandler)

	// Request policy decision from OPA
	ans, err := decision.Can(func() decision.Query {
		return decision.Query{
			Host:   h.opaHost,
			Policy: "/karavi/authz/powermax/url",
			Input: map[string]interface{}{
				"method": r.Method,
				"url":    r.URL.Path,
			},
		}
	})
	if err != nil {
		log.Printf("opa: %v", err)
		writeError(w, err.Error(), http.StatusInternalServerError)
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
		writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !resp.Result.Allow {
		log.Println("Request denied")
		writeError(w, "request denied for path", http.StatusNotFound)
		return
	}

	mux.ServeHTTP(w, r)
}

func (h *PowerMaxHandler) spoofLoginRequest(w http.ResponseWriter, r *http.Request) {
	_, span := trace.SpanFromContext(r.Context()).Tracer().Start(r.Context(), "spoofLoginRequest")
	defer span.End()
	_, err := w.Write([]byte("hellofromkaravi"))
	if err != nil {
		h.log.Printf("failed to write response: %v", err)
	}
}

// TODO(ian): This will need to be updated to return errors in a format expected
// by the powermax client. Currently this is just the #writeError function that
// was written for the powerflex system.
func (h *PowerMaxHandler) writeError(w http.ResponseWriter, msg string, code int) {
	log.Printf("proxy: powermax_handler: writing error:  %d: %s", code, msg)
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

func (s *PowerMaxSystem) volumeCreateHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	})
}

func (s *PowerMaxSystem) volumeDeleteHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	})
}
