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
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"sync"

	pmax "github.com/dell/gopowermax/v2"

	"github.com/julienschmidt/httprouter"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Common Unisphere constants.
const (
	appName              = "CSM-Authz"
	limitBodySizeInBytes = 1024
	cylinderSizeInBytes  = 1966080
	SRPNONE              = "NONE"
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

// GetSystems returns the configured systems
func (h *PowerMaxHandler) GetSystems() map[string]*PowerMaxSystem {
	return h.systems
}

// UpdateSystems updates the PowerMaxHandler via a SystemConfig
func (h *PowerMaxHandler) UpdateSystems(ctx context.Context, r io.Reader, log *logrus.Entry) error {
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
		if h.systems[k], err = buildPowerMaxSystem(ctx, v, log); err != nil {
			h.log.WithError(err).Error("building powermax system")
		}
	}

	for k := range powerMaxSystems {
		h.log.WithField("updated_system", k).Info("Updated systems")
	}

	return nil
}

func buildPowerMaxSystem(ctx context.Context, e SystemEntry, log *logrus.Entry) (*PowerMaxSystem, error) {
	tgt, err := url.Parse(e.Endpoint)
	if err != nil {
		return nil, err
	}
	return &PowerMaxSystem{
		SystemEntry: e,
		log:         log,
		rp:          httputil.NewSingleHostReverseProxy(tgt),
	}, nil
}

func (h *PowerMaxHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fwd := ForwardedHeader(r)
	fwdFor := fwd["for"]

	ep, systemID := SplitEndpointSystemID(fwdFor)
	h.log.WithFields(logrus.Fields{
		"Endpoint": ep,
		"SystemID": systemID,
	}).Debug("Serving request")
	r = r.WithContext(context.WithValue(r.Context(), web.SystemIDKey, systemID))

	v, ok := h.systems[systemID]
	if !ok {
		writeError(w, "powermax", "system id not found", http.StatusBadGateway, h.log)
		return
	}

	// Add authentication headers.
	r.SetBasicAuth(v.User, v.Password)

	// Instrument the proxy
	attrs := trace.WithAttributes(attribute.String("powermax.endpoint", ep), attribute.String("powermax.systemid", systemID))
	opts := otelhttp.WithSpanOptions(attrs)
	proxyHandler := otelhttp.NewHandler(v.rp, "proxy", opts)

	router := httprouter.New()
	router.Handler(http.MethodPut,
		"/univmax/restapi/:version/sloprovisioning/symmetrix/:systemid/storagegroup/:storagegroup/",
		v.editStorageGroupHandler(proxyHandler, h.enforcer, h.opaHost))
	router.Handler(http.MethodPut,
		"/univmax/restapi/:version/sloprovisioning/symmetrix/:systemid/volume/:volumeid/",
		v.volumeModifyHandler(proxyHandler, h.enforcer, h.opaHost))
	router.NotFound = proxyHandler
	router.MethodNotAllowed = proxyHandler
	router.RedirectTrailingSlash = false

	// Save a copy of this request for debugging.
	requestDump, err := httputil.DumpRequest(r, true)
	if err != nil {
		h.log.Error(err)
	}

	h.log.Debug(string(requestDump))

	router.ServeHTTP(w, r)
}

// editStorageGroupHandler handles storage group update requests.
//
// The REST call is:
// PUT /univmax/restapi/91/sloprovisioning/symmetrix/:systemid/storagegroup/:storagegroupid
//
// The payload looks like:
// {
//
//	"editStorageGroupActionParam": {
//		"expandStorageGroupParam": {
//	   ...
//		}
//	},
//
// "executionOption": "SYNCHRONOUS"}
//
// The action ("expandStorageGroupParam" in the example) will be different depending on the
// intended edit operation. This handler will process the action and delegate to the appropriate
// handler.
func (s *PowerMaxSystem) editStorageGroupHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, span := trace.SpanFromContext(r.Context()).TracerProvider().Tracer("").Start(r.Context(), "powermaxEditStorageGroupHandler")
		defer span.End()

		params := httprouter.ParamsFromContext(r.Context())

		s.log.WithField("storage_group", params.ByName("storagegroup")).Debug("Edit StorageGroup")
		b, err := ioutil.ReadAll(io.LimitReader(r.Body, limitBodySizeInBytes))
		if err != nil {
			writeError(w, "powermax", "failure reading request body", http.StatusInternalServerError, s.log)
			return
		}

		type editAction struct {
			Editstoragegroupactionparam map[string]interface{} `json:"editStorageGroupActionParam"`
		}
		var action editAction
		err = json.Unmarshal(b, &action)
		if err != nil {
			writeError(w, "powermax", "failure decoding request body", http.StatusInternalServerError, s.log)
			return
		}
		r.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		switch {
		case hasKey(action.Editstoragegroupactionparam, "expandStorageGroupParam"):
			if m, ok := action.Editstoragegroupactionparam["expandStorageGroupParam"].(map[string]interface{}); ok {
				if _, ok := m["addSpecificVolumeParam"]; ok {
					next.ServeHTTP(w, r)
					return
				}
			}
			s.volumeCreateHandler(next, enf, opaHost).ServeHTTP(w, r)
			return
		default:
			next.ServeHTTP(w, r)
			return
		}
	})
}

// volumeCreateHandler handles a create volume request.
//
// The REST call is:
// PUT /univmax/restapi/91/sloprovisioning/symmetrix/:systemid/storagegroup/:storagegroupid
//
// The payload looks like:
// {
//
//	"editStorageGroupActionParam": {
//		"expandStorageGroupParam": {
//			"addVolumeParam": {
//				"emulation": "FBA",
//				"create_new_volumes": true,
//				"volumeAttributes": [
//				{
//					"num_of_vols": 1,
//					"volumeIdentifier": {
//						"volumeIdentifierChoice": "identifier_name",
//						"identifier_name": "csi-CSM-pmax-9c79d51b18"
//					},
//					"capacityUnit": "CYL",
//					"volume_size": "547"
//				}
//				],
//				"remoteSymmSGInfoParam": {
//					"force": true
//				}
//			}
//		}
//	},
//
// "executionOption": "SYNCHRONOUS"}
func (s *PowerMaxSystem) volumeCreateHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := trace.SpanFromContext(r.Context()).TracerProvider().Tracer("").Start(r.Context(), "powermaxVolumeCreateHandler")
		defer span.End()

		params := httprouter.ParamsFromContext(r.Context())

		s.log.WithField("storage_group", params.ByName("storagegroup")).Debug("Creating volume in StorageGroup")
		b, err := ioutil.ReadAll(io.LimitReader(r.Body, limitBodySizeInBytes))
		if err != nil {
			writeError(w, "powermax", "failed to read body", http.StatusInternalServerError, s.log)
			return
		}

		defer r.Body.Close()

		var payloadTemp map[string]interface{}
		if err := json.NewDecoder(bytes.NewReader(b)).Decode(&payloadTemp); err != nil {
			s.log.WithError(err).Error("proxy: decoding create volume request")
			writeError(w, "powermax", "failed to decode body to json", http.StatusInternalServerError, s.log)
			return
		}

		var op string
		if action, ok := payloadTemp["editStorageGroupActionParam"]; ok {
			v, ok := action.(map[string]interface{})
			if !ok {
				writeError(w, "powermax", "invalid payload", http.StatusBadRequest, s.log)
				return
			}
			if _, ok := v["expandStorageGroupParam"]; ok {
				op = "expandStorageGroupParam"
			}
		}

		// Other modification operations can pass through.
		if op != "expandStorageGroupParam" {
			next.ServeHTTP(w, r)
			return
		}

		var payload powermaxAddVolumeRequest
		if err := json.NewDecoder(bytes.NewReader(b)).Decode(&payload); err != nil {
			writeError(w, "powermax", "failed to decode addVolumeRequest body", http.StatusInternalServerError, s.log)
			return
		}

		if len(payload.Editstoragegroupactionparam.Expandstoragegroupparam.Addvolumeparam.Volumeattributes) == 0 {
			next.ServeHTTP(w, r)
			return
		}

		capAsInt, err := strconv.ParseInt(payload.Editstoragegroupactionparam.Expandstoragegroupparam.Addvolumeparam.Volumeattributes[0].VolumeSize, 0, 64)
		if err != nil {
			writeError(w, "powermax", "failed to parse capacity", http.StatusInternalServerError, s.log)
			return
		}

		volID := payload.Editstoragegroupactionparam.Expandstoragegroupparam.Addvolumeparam.Volumeattributes[0].Volumeidentifier.IdentifierName

		// Determine which pool this SG exists within, as it will form the quota key.
		client, err := pmax.NewClientWithArgs(s.Endpoint, appName, true, false)
		if err != nil {
			writeError(w, "powermax", "failed to build powermax client", http.StatusInternalServerError, s.log)
			return
		}
		if err := client.Authenticate(ctx, &pmax.ConfigConnect{
			Username: s.User,
			Password: s.Password,
		}); err != nil {
			writeError(w, "powermax", "failed to authenticate with unisphere", http.StatusInternalServerError, s.log)
			return
		}

		sg, err := client.GetStorageGroup(ctx, params.ByName("systemid"), params.ByName("storagegroup"))
		if err != nil {
			s.log.WithError(err).Error("getting storage group")
			return
		}

		jwtGroup := r.Context().Value(web.JWTTenantName)
		group, ok := jwtGroup.(string)
		if !ok {
			writeError(w, "powermax", "invalid JWT group", http.StatusInternalServerError, s.log)
			return
		}

		jwtValue := r.Context().Value(web.JWTKey)
		jwtToken, ok := jwtValue.(token.Token)
		if !ok {
			writeError(w, "powermax", "incorrect type for JWT token", http.StatusInternalServerError, s.log)
			return
		}

		jwtClaims, err := jwtToken.Claims()
		if err != nil {
			writeError(w, "powermax", "decoding token claims", http.StatusInternalServerError, s.log)
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
		}).Debug("Create volume request")

		// Ask OPA if this request is valid against the policy.
		s.log.Debugln("Asking OPA...")
		// Request policy decision from OPA
		ans, err := decision.Can(func() decision.Query {
			return decision.Query{
				Host:   opaHost,
				Policy: "/karavi/volumes/powermax/create",
				Input: map[string]interface{}{
					"claims":          jwtClaims,
					"request":         map[string]interface{}{"volumeSizeInKb": paramVolSizeInKb},
					"storagepool":     paramStoragePoolID,
					"storagesystemid": paramSystemID,
					"systemtype":      "powermax",
				},
			}
		})
		if err != nil {
			s.log.WithError(err).Error("asking OPA for volume create decision")
			writeError(w, "powermax", fmt.Sprintf("asking OPA for volume create decision: %v", err), http.StatusInternalServerError, s.log)
			return
		}

		var opaResp CreateOPAResponse
		s.log.WithField("opa_response", string(ans)).Debug()
		err = json.NewDecoder(bytes.NewReader(ans)).Decode(&opaResp)
		if err != nil {
			s.log.WithError(err).Error("decoding opa response")
			writeError(w, "powermax", "decoding opa request body", http.StatusInternalServerError, s.log)
			return
		}
		s.log.WithField("opa_response", opaResp).Debug()
		if resp := opaResp.Result; !resp.Allow {
			reason := strings.Join(opaResp.Result.Deny, ",")
			s.log.WithField("reason", reason).Debug("request denied")
			writeError(w, "powermax", fmt.Sprintf("request denied: %v", reason), http.StatusBadRequest, s.log)
			return
		}

		// In the scenario where multiple roles are allowing
		// this request, choose the one with the most quota.
		var maxQuotaInKb int
		for _, quota := range opaResp.Result.PermittedRoles {
			if quota == 0 {
				maxQuotaInKb = 0
				break
			}
			if quota >= maxQuotaInKb {
				maxQuotaInKb = quota
			}
		}

		// Ask Redis if this request is valid against existing volumes.
		qr := quota.Request{
			SystemType:    "powermax",
			SystemID:      paramSystemID,
			StoragePoolID: paramStoragePoolID,
			Group:         group,
			VolumeName:    volID,
			Capacity:      fmt.Sprintf("%d", paramVolSizeInKb),
		}

		s.log.Debugln("Approving request...")
		// Ask our quota enforcer if it approves the request.
		ok, err = enf.ApproveRequest(ctx, qr, int64(maxQuotaInKb))
		if err != nil {
			s.log.WithError(err).Error("approving request")
			writeError(w, "powermax", "failed to approve request", http.StatusInternalServerError, s.log)
			return
		}
		if !ok {
			s.log.Debugln("request was not approved")
			writeError(w, "powermax", "request denied: not enough quota", http.StatusInsufficientStorage, s.log)
			return
		}

		// Reset the original request
		if err = r.Body.Close(); err != nil {
			s.log.WithError(err).Error("closing original request body")
		}
		r.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		sw := &web.StatusWriter{
			ResponseWriter: w,
		}

		s.log.Debugln("Proxying request...")
		r = r.WithContext(ctx)
		next.ServeHTTP(sw, r)

		s.log.WithFields(logrus.Fields{
			"Response code": sw.Status,
		}).Debug()
		switch sw.Status {
		case http.StatusOK:
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

// volumeModifyHandler handles a modify volume request.
//
// The REST call is:
// PUT /univmax/restapi/91/sloprovisioning/symmetrix/1234567890/volume/003E4
//
// The payload looks like:
//
//	{"editVolumeActionParam":{
//	  "modifyVolumeIdentifierParam":{
//	    "volumeIdentifier":{"volumeIdentifierChoice":"identifier_name","identifier_name":"_DEL003E4"}
//	  }
//	},"executionOption":"SYNCHRONOUS"}
func (s *PowerMaxSystem) volumeModifyHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := trace.SpanFromContext(r.Context()).TracerProvider().Tracer("").Start(r.Context(), "powermaxVolumeModifyHandler")
		defer span.End()

		params := httprouter.ParamsFromContext(r.Context())

		s.log.WithFields(logrus.Fields{
			"system_id": params.ByName("systemid"),
			"volume_id": params.ByName("volumeid"),
		}).Debug("Modifying volume")
		b, err := ioutil.ReadAll(io.LimitReader(r.Body, limitBodySizeInBytes))
		if err != nil {
			writeError(w, "powermax", "failure reading request body", http.StatusInternalServerError, s.log)
			return
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(bytes.NewReader(b)).Decode(&payload); err != nil {
			writeError(w, "powermax", "failure decoding request body", http.StatusInternalServerError, s.log)
			return
		}
		defer r.Body.Close()

		var op string
		if action, ok := payload["editVolumeActionParam"]; ok {
			v, ok := action.(map[string]interface{})
			if !ok {
				writeError(w, "powermax", "invalid payload", http.StatusBadRequest, s.log)
				return
			}
			if _, ok := v["modifyVolumeIdentifierParam"]; ok {
				op = "modifyVolumeIdentifierParam"
			}
		}

		// Other modification operations can pass through.
		if op != "modifyVolumeIdentifierParam" {
			next.ServeHTTP(w, r)
			return
		}

		var modVolReq powermaxModifyVolumeRequest
		if err := json.Unmarshal(b, &modVolReq); err != nil {
			writeError(w, "powermax", err.Error(), http.StatusInternalServerError, s.log)
			return
		}

		// Determine which pool this SG exists within, as it will form the quota key.
		client, err := pmax.NewClientWithArgs(s.Endpoint, appName, true, false)
		if err != nil {
			writeError(w, "powermax", "failed to build powermax client", http.StatusInternalServerError, s.log)
			return
		}
		if err := client.Authenticate(ctx, &pmax.ConfigConnect{
			Username: s.User,
			Password: s.Password,
		}); err != nil {
			writeError(w, "powermax", "failed to authenticate with unisphere", http.StatusInternalServerError, s.log)
			return
		}

		vol, err := client.GetVolumeByID(ctx, params.ByName("systemid"), params.ByName("volumeid"))
		if err != nil {
			s.log.WithError(err).Error("getting volume by ID")
			return
		}

		jwtValue := r.Context().Value(web.JWTKey)
		jwtToken, ok := jwtValue.(token.Token)
		if !ok {
			writeError(w, "powermax", "incorrect type for JWT token", http.StatusInternalServerError, s.log)
			return
		}

		jwtClaims, err := jwtToken.Claims()
		if err != nil {
			writeError(w, "powermax", "decoding token claims", http.StatusInternalServerError, s.log)
			return
		}

		volID := vol.VolumeIdentifier
		var storagePoolID string
		// Find the first storage group that is associated with an SRP that
		// is not "NONE".
		for _, sgID := range vol.StorageGroupIDList {
			sg, err := client.GetStorageGroup(ctx, params.ByName("systemid"), sgID)
			if err != nil {
				writeError(w, "powermax", fmt.Sprintf("get storage group: %q", sgID), http.StatusInternalServerError, s.log)
				return
			}
			if sg.SRP != SRPNONE {
				storagePoolID = sg.SRP
				break
			}
		}
		if strings.TrimSpace(storagePoolID) == "" {
			writeError(w, "powermax", "no storage pool found", http.StatusBadRequest, s.log)
			return
		}

		qr := quota.Request{
			SystemType:    "powermax",
			SystemID:      params.ByName("systemid"),
			StoragePoolID: storagePoolID,
			Group:         jwtClaims.Group,
			VolumeName:    volID,
		}
		ok, err = enf.ValidateOwnership(ctx, qr)
		if err != nil {
			writeError(w, "powermax", "validating ownership failed", http.StatusInternalServerError, s.log)
			return
		}
		if !ok {
			writeError(w, "powermax", "request was denied", http.StatusBadRequest, s.log)
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(b))
		sw := &web.StatusWriter{
			ResponseWriter: w,
		}
		next.ServeHTTP(sw, r)

		// If the volume was renamed to _DEL, then we can mark this as deleted and remove capacity.
		if strings.HasPrefix(modVolReq.Editvolumeactionparam.Modifyvolumeidentifierparam.Volumeidentifier.IdentifierName, "_DEL") {
			ok, err = enf.PublishDeleted(ctx, qr)
			if err != nil {
				writeError(w, "powermax", "publish deleted", http.StatusInternalServerError, s.log)
				return
			}
			if !ok {
				writeError(w, "powermax", "request denied", http.StatusBadRequest, s.log)
				return
			}
		}
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

type powermaxModifyVolumeRequest struct {
	Editvolumeactionparam struct {
		Modifyvolumeidentifierparam struct {
			Volumeidentifier struct {
				Volumeidentifierchoice string `json:"volumeIdentifierChoice"`
				IdentifierName         string `json:"identifier_name"`
			} `json:"volumeIdentifier"`
		} `json:"modifyVolumeIdentifierParam"`
	} `json:"editVolumeActionParam"`
	Executionoption string `json:"executionOption"`
}

func hasKey(m map[string]interface{}, key string) bool {
	_, ok := m[key]
	return ok
}
