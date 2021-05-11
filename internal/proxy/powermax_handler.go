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
	"karavi-authorization/internal/quota"
	"karavi-authorization/internal/token"
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
		h.handleError(w, http.StatusBadGateway, errors.New("system id not found"))
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
		v.editStorageGroupHandler(proxyHandler, h.enforcer, h.opaHost))
	router.Handler(http.MethodPut,
		"/univmax/restapi/91/sloprovisioning/symmetrix/:systemid/volume/:volumeid/",
		v.volumeModifyHandler(proxyHandler, h.enforcer, h.opaHost))
	router.NotFound = proxyHandler
	router.MethodNotAllowed = proxyHandler
	router.RedirectTrailingSlash = false

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
	if h.handleErrorf(w, http.StatusInternalServerError, err, "opa decision failed") {
		return
	}
	var resp struct {
		Result struct {
			Allow bool `json:"allow"`
		} `json:"result"`
	}
	err = json.NewDecoder(bytes.NewReader(ans)).Decode(&resp)
	if h.handleErrorf(w, http.StatusInternalServerError, err, "decoding body") {
		return
	}
	if !resp.Result.Allow {
		h.handleError(w, http.StatusBadRequest, errors.New("request denied"))
		return
	}
	router.ServeHTTP(w, r)
}

// editStorageGroupHandler handles storage group update requests.
//
// The REST call is:
// PUT /univmax/restapi/91/sloprovisioning/symmetrix/:systemid/storagegroup/:storagegroupid
//
// The payload looks like:
// {
// "editStorageGroupActionParam": {
// 	"expandStorageGroupParam": {
//    ...
// 	}
// },
// "executionOption": "SYNCHRONOUS"}
//
// The action ("expandStorageGroupParam" in the example) will be different depending on the
// intended edit operation. This handler will process the action and delegate to the appropriate
// handler.
//
func (s *PowerMaxSystem) editStorageGroupHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, span := trace.SpanFromContext(r.Context()).Tracer().Start(r.Context(), "powermaxEditStorageGroupHandler")
		defer span.End()

		params := httprouter.ParamsFromContext(r.Context())

		log.Printf("Edit SG %q", params.ByName("storagegroup"))
		b, err := ioutil.ReadAll(io.LimitReader(r.Body, limitBodySizeInBytes))
		if s.handleErrorf(w, http.StatusInternalServerError, err, "reading body") {
			return
		}

		type editAction struct {
			Editstoragegroupactionparam map[string]interface{} `json:"editStorageGroupActionParam"`
		}
		var action editAction
		err = json.Unmarshal(b, &action)
		if s.handleErrorf(w, http.StatusInternalServerError, err, "reading body") {
			return
		}
		r.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		switch {
		case hasKey(action.Editstoragegroupactionparam, "expandStorageGroupParam"):
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
// "editStorageGroupActionParam": {
// 	"expandStorageGroupParam": {
// 		"addVolumeParam": {
// 			"emulation": "FBA",
// 			"create_new_volumes": true,
// 			"volumeAttributes": [
// 			{
// 				"num_of_vols": 1,
// 				"volumeIdentifier": {
// 					"volumeIdentifierChoice": "identifier_name",
// 					"identifier_name": "csi-CSM-pmax-9c79d51b18"
// 				},
// 				"capacityUnit": "CYL",
// 				"volume_size": "547"
// 			}
// 			],
// 			"remoteSymmSGInfoParam": {
// 				"force": true
// 			}
// 		}
// 	}
// },
// "executionOption": "SYNCHRONOUS"}
//
func (s *PowerMaxSystem) volumeCreateHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := trace.SpanFromContext(r.Context()).Tracer().Start(r.Context(), "powermaxVolumeCreateHandler")
		defer span.End()

		params := httprouter.ParamsFromContext(r.Context())

		s.log.Printf("Creating volume in SG %q", params.ByName("storagegroup"))
		b, err := ioutil.ReadAll(io.LimitReader(r.Body, limitBodySizeInBytes))
		if s.handleErrorf(w, http.StatusInternalServerError, err, "reading body") {
			return
		}

		var payload powermaxAddVolumeRequest
		if err := json.NewDecoder(bytes.NewReader(b)).Decode(&payload); err != nil {
			s.handleErrorf(w, http.StatusInternalServerError, err, "decoding body")
			return
		}
		defer r.Body.Close()

		capAsInt, err := strconv.ParseInt(payload.Editstoragegroupactionparam.Expandstoragegroupparam.Addvolumeparam.Volumeattributes[0].VolumeSize, 0, 64)
		if s.handleErrorf(w, http.StatusInternalServerError, err, "parsing int") {
			return
		}

		volID := payload.Editstoragegroupactionparam.Expandstoragegroupparam.Addvolumeparam.Volumeattributes[0].Volumeidentifier.IdentifierName

		// Determine which pool this SG exists within, as it will form the quota key.
		client, err := pmax.NewClientWithArgs(s.Endpoint, pmax.APIVersion91, "CSMAuthz", true, false)
		if s.handleErrorf(w, http.StatusInternalServerError, err, "building client") {
			return
		}
		if err := client.Authenticate(&pmax.ConfigConnect{
			Username: s.User,
			Password: s.Password,
		}); s.handleErrorf(w, http.StatusInternalServerError, err, "unisphere authn") {
			return
		}

		sg, err := client.GetStorageGroup(params.ByName("systemid"), params.ByName("storagegroup"))
		if err != nil {
			s.log.Printf("getting SG: %+v", err)
			return
		}

		jwtGroup := r.Context().Value(web.JWTTenantName)
		group, ok := jwtGroup.(string)
		if !ok {
			s.handleErrorf(w, http.StatusInternalServerError, err, "invalid JWT group")
			return
		}

		jwtValue := r.Context().Value(web.JWTKey)
		jwtToken, ok := jwtValue.(*jwt.Token)
		if !ok {
			s.handleErrorf(w, http.StatusInternalServerError, err, "invalid JWT token")
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
				Host:   opaHost,
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
		s.log.Printf("OPA Response: %s", string(ans))
		err = json.NewDecoder(bytes.NewReader(ans)).Decode(&opaResp)
		if s.handleErrorf(w, http.StatusInternalServerError, err, "decoding OPA response") {
			return
		}
		s.log.Printf("OPA Response: %+v", opaResp)
		if resp := opaResp.Result; !resp.Allow {
			reason := strings.Join(opaResp.Result.Deny, ",")
			s.handleErrorf(w, http.StatusBadRequest, err, "request denied: %v", reason)
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

		// Ask Redis if this request is valid against existing volumes.
		qr := quota.Request{
			SystemType:    "powermax",
			SystemID:      paramSystemID,
			StoragePoolID: paramStoragePoolID,
			Group:         group,
			VolumeName:    volID,
			Capacity:      fmt.Sprintf("%d", paramVolSizeInKb),
		}

		s.log.Println("Approving request...")
		// Ask our quota enforcer if it approves the request.
		ok, err = enf.ApproveRequest(ctx, qr, int64(maxQuotaInKb))
		if s.handleErrorf(w, http.StatusInternalServerError, err, "failed to approve request") {
			return
		}
		if !ok {
			s.handleErrorf(w, http.StatusInsufficientStorage, err, "request denied: not enough quota")
			return
		}

		// Reset the original request
		if err = r.Body.Close(); err != nil {
			s.log.Printf("Failed to close original request body: %v", err)
		}
		r.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		sw := &web.StatusWriter{
			ResponseWriter: w,
		}

		s.log.Println("Proxying request...")
		r = r.WithContext(ctx)
		next.ServeHTTP(sw, r)

		s.log.Printf("Resp: Code: %d", sw.Status)
		switch sw.Status {
		case http.StatusOK:
			s.log.Println("Publish created")
			ok, err := enf.PublishCreated(r.Context(), qr)
			if s.handleErrorf(w, http.StatusInternalServerError, err, "creation publish failed") {
				return
			}
			s.log.Println("Result of publish:", ok)
		default:
			s.log.Println("Non 200 response, nothing to publish")
		}
	})
}

// volumeModifyHandler handles a modify volume request.
//
// The REST call is:
// PUT /univmax/restapi/91/sloprovisioning/symmetrix/1234567890/volume/003E4
//
// The payload looks like:
// {"editVolumeActionParam":{
//   "modifyVolumeIdentifierParam":{
//     "volumeIdentifier":{"volumeIdentifierChoice":"identifier_name","identifier_name":"_DEL003E4"}
//   }
// },"executionOption":"SYNCHRONOUS"}
func (s *PowerMaxSystem) volumeModifyHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := trace.SpanFromContext(r.Context()).Tracer().Start(r.Context(), "powermaxVolumeModifyHandler")
		defer span.End()

		params := httprouter.ParamsFromContext(r.Context())

		log.Printf("Modifying volume %s/%s", params.ByName("systemid"), params.ByName("volumeid"))
		b, err := ioutil.ReadAll(io.LimitReader(r.Body, limitBodySizeInBytes))
		if s.handleErrorf(w, http.StatusInternalServerError, err, "reading body") {
			return
		}

		var payload map[string]interface{}
		if err := json.NewDecoder(bytes.NewReader(b)).Decode(&payload); err != nil {
			s.handleErrorf(w, http.StatusInternalServerError, err, "decoding body")
			return
		}
		defer r.Body.Close()

		var op string
		if action, ok := payload["editVolumeActionParam"]; ok {
			v, ok := action.(map[string]interface{})
			if !ok && s.handleError(w, http.StatusInternalServerError, errors.New("invalid payload")) {
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
		if err := json.Unmarshal(b, &modVolReq); s.handleError(w, http.StatusInternalServerError, err) {
			return
		}

		// Determine which pool this SG exists within, as it will form the quota key.
		client, err := pmax.NewClientWithArgs(s.Endpoint, pmax.APIVersion91, appName, true, false)
		if s.handleErrorf(w, http.StatusInternalServerError, err, "building client") {
			return
		}
		if err := client.Authenticate(&pmax.ConfigConnect{
			Username: s.User,
			Password: s.Password,
		}); s.handleErrorf(w, http.StatusInternalServerError, err, "unisphere authn") {
			return
		}

		vol, err := client.GetVolumeByID(params.ByName("systemid"), params.ByName("volumeid"))
		if s.handleErrorf(w, http.StatusInternalServerError, err, "get volume by ID") {
			return
		}

		jwtValue := r.Context().Value(web.JWTKey)
		jwtToken, ok := jwtValue.(*jwt.Token)
		if !ok {
			panic("incorrect type for a jwt token")
		}

		jwtClaims, ok := jwtToken.Claims.(*token.Claims)
		if !ok {
			log.Printf("JWT claims: %+v", jwtToken.Claims)
			panic("incorrect type for jwt claims")
		}

		volID := vol.VolumeIdentifier
		var storagePoolID string
		// Find the first storage group that is associated with an SRP that
		// is not "NONE".
		for _, sgID := range vol.StorageGroupIDList {
			sg, err := client.GetStorageGroup(params.ByName("systemid"), sgID)
			if s.handleErrorf(w, http.StatusInternalServerError, err, "get storage group: %q", sgID) {
				return
			}
			if sg.SRP != SRPNONE {
				storagePoolID = sg.SRP
				break
			}
		}
		if strings.TrimSpace(storagePoolID) == "" {
			s.handleError(w, http.StatusBadRequest, errors.New("no storage pool"))
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
		if s.handleErrorf(w, http.StatusInternalServerError, err, "validating ownership failed") {
			return
		}
		if !ok {
			s.handleError(w, http.StatusBadRequest, errors.New("request was denied"))
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
			if s.handleErrorf(w, http.StatusInternalServerError, err, "publish deleted") {
				return
			}
			if !ok {
				s.handleErrorf(w, http.StatusBadRequest, err, "request denied")
				return
			}
		}
	})
}

func (h *PowerMaxHandler) handleErrorf(w http.ResponseWriter, statusCode int, err error, format string, args ...interface{}) bool {
	return handleError(h.log, w, statusCode, err, format, args...)
}

func (h *PowerMaxHandler) handleError(w http.ResponseWriter, statusCode int, err error) bool {
	return handleError(h.log, w, statusCode, err, "")
}

func (s *PowerMaxSystem) handleErrorf(w http.ResponseWriter, statusCode int, err error, format string, args ...interface{}) bool {
	return handleError(s.log, w, statusCode, err, format, args...)
}

func (s *PowerMaxSystem) handleError(w http.ResponseWriter, statusCode int, err error) bool {
	return handleError(s.log, w, statusCode, err, "")
}

func handleError(logger *logrus.Entry, w http.ResponseWriter, statusCode int, err error, format string, args ...interface{}) bool {
	if err == nil {
		return false
	}
	if logger != nil {
		logger.WithError(err).Errorf(format, args...)
	}
	w.WriteHeader(statusCode)
	return true
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
