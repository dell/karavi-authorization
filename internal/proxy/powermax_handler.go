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
	"karavi-authorization/internal/web"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"

	pmax "github.com/dell/gopowermax"
	"github.com/dgrijalva/jwt-go"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/trace"
)

const (
	cylinderSizeInBytes = 1966080
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

	router := httprouter.New()

	router.Handler(http.MethodPut,
		"/univmax/restapi/91/sloprovisioning/symmetrix/:systemid/storagegroup/:storagegroup/",
		v.volumeCreateHandler(proxyHandler, h.enforcer, h.opaHost))
	router.NotFound = proxyHandler

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

	router.ServeHTTP(w, r)
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
		params := httprouter.ParamsFromContext(r.Context())

		log.Printf("Creating volume in SG %q", params.ByName("storagegroup"))
		b, err := ioutil.ReadAll(io.LimitReader(r.Body, 1024*1024))
		if err != nil {
			log.Printf("reading body: %+v", err)
			return
		}

		var payload powermaxAddVolumeRequest
		if err := json.NewDecoder(bytes.NewReader(b)).Decode(&payload); err != nil {
			log.Printf("decoding body: %+v", err)
			return
		}
		defer r.Body.Close()
		log.Printf("Size of the volume is %s %s",
			payload.Editstoragegroupactionparam.Expandstoragegroupparam.Addvolumeparam.Volumeattributes[0].VolumeSize,
			payload.Editstoragegroupactionparam.Expandstoragegroupparam.Addvolumeparam.Volumeattributes[0].Capacityunit)

		capAsInt, err := strconv.ParseInt(payload.Editstoragegroupactionparam.Expandstoragegroupparam.Addvolumeparam.Volumeattributes[0].VolumeSize, 0, 64)
		if err != nil {
			log.Printf("parsing int: %+v", err)
			return
		}

		capCylInKb := (capAsInt * cylinderSizeInBytes) / 1024

		log.Printf("Size of the volume in Kb is %d", capCylInKb)
		log.Printf("Identifier of the volume is %q",
			payload.Editstoragegroupactionparam.Expandstoragegroupparam.Addvolumeparam.Volumeattributes[0].Volumeidentifier.IdentifierName)

		pvName := r.Header.Get(HeaderPVName)
		log.Printf("PVName is %q", pvName)

		// Determine which pool this SG exists within, as it will form the quota key.
		client, err := pmax.NewClientWithArgs(s.Endpoint, pmax.APIVersion91, "CSMAuthz", true, false)
		if err != nil {
			log.Printf("building client: %+v", err)
			return
		}
		if err := client.Authenticate(&pmax.ConfigConnect{
			Username: s.User,
			Password: s.Password,
		}); err != nil {
			log.Printf("authn: %+v", err)
			return
		}

		sg, err := client.GetStorageGroup(params.ByName("systemid"), params.ByName("storagegroup"))
		if err != nil {
			log.Printf("getting SG: %+v", err)
			return
		}

		log.Printf("Storage group %q belongs to storage pool %q", sg.StorageGroupID, sg.SRP)

		jwtValue := r.Context().Value(web.JWTKey)
		jwtToken, ok := jwtValue.(*jwt.Token)
		if !ok {
			writeError(w, "incorrect type for JWT token", http.StatusInternalServerError)
			return
		}

		paramSystemID := params.ByName("systemid")
		paramStorageGroupID := params.ByName("storagegroup")
		paramStoragePoolID := sg.SRP
		paramVolSizeInKb := (capAsInt * cylinderSizeInBytes) / 1024
		paramVolID := payload.Editstoragegroupactionparam.Expandstoragegroupparam.Addvolumeparam.Volumeattributes[0].Volumeidentifier.IdentifierName
		paramPVName := r.Header.Get(HeaderPVName)

		s.log.WithFields(logrus.Fields{
			"systemID": paramSystemID,
			"sgID":     paramStorageGroupID,
			"spID":     paramStoragePoolID,
			"volSize":  paramVolSizeInKb,
			"volID":    paramVolID,
			"pvName":   paramPVName,
		}).Println("Proxy create volume request")

		// Ask OPA if this request is valid against the policy.
		s.log.Println("Asking OPA...")
		// Request policy decision from OPA
		ans, err := decision.Can(func() decision.Query {
			return decision.Query{
				Host: opaHost,
				// TODO(ian): This will need to be namespaced under "powermax".
				Policy: "/karavi/volumes/powermax/create",
				Input: map[string]interface{}{
					"claims":          jwtToken.Claims,
					"request":         map[string]interface{}{"volumeSizeInKb": paramVolSizeInKb},
					"storagepool":     paramStoragePoolID,
					"storagesystemid": paramSystemID,
					"systemtype":      "powermax",
				},
			}
		})
		var opaResp CreateOPAResponse
		log.Printf("OPA REsponse: %s", string(ans))
		err = json.NewDecoder(bytes.NewReader(ans)).Decode(&opaResp)
		if err != nil {
			s.log.Printf("decoding opa response: %+v", err)
			writeError(w, "decoding opa request body", http.StatusInternalServerError)
			return
		}
		log.Printf("OPA Response: %+v", opaResp)
		if resp := opaResp.Result; !resp.Allow {
			reason := strings.Join(opaResp.Result.Deny, ",")
			s.log.Printf("request denied: %v", reason)
			writeError(w, fmt.Sprintf("request denied: %v", reason), http.StatusBadRequest)
			return
		}

		// Ask Redis if this request is valid against existing volumes.

		r.Body = io.NopCloser(bytes.NewReader(b))
		next.ServeHTTP(w, r)
	})
}

func (s *PowerMaxSystem) volumeDeleteHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	})
}

type powermaxAddVolumeRequest struct {
	Editstoragegroupactionparam struct {
		Expandstoragegroupparam struct {
			Addvolumeparam struct {
				Emulation        string `json:"emulation"`
				CreateNewVolumes bool   `json:"create_new_volumes"`
				Volumeattributes []struct {
					NumOfVols        int `json:"num_of_vols"`
					Volumeidentifier struct {
						Volumeidentifierchoice string `json:"volumeIdentifierChoice"`
						IdentifierName         string `json:"identifier_name"`
					} `json:"volumeIdentifier"`
					Capacityunit string `json:"capacityUnit"`
					VolumeSize   string `json:"volume_size"`
				} `json:"volumeAttributes"`
				Remotesymmsginfoparam struct {
					Force bool `json:"force"`
				} `json:"remoteSymmSGInfoParam"`
			} `json:"addVolumeParam"`
		} `json:"expandStorageGroupParam"`
	} `json:"editStorageGroupActionParam"`
	Executionoption string `json:"executionOption"`
}
