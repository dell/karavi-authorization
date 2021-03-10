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
	"karavi-authorization/internal/web"
	"log"
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
	"github.com/dgrijalva/jwt-go"
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
func (h *PowerFlexHandler) UpdateSystems(ctx context.Context, r io.Reader) error {
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
		if h.systems[k], err = buildSystem(ctx, v); err != nil {
			h.log.Errorf("proxy: powerflex failure: %+v", err)
		}
	}
	h.log.Printf("Updated systems: %+v", updated)
	return nil
}

func buildSystem(ctx context.Context, e SystemEntry) (*System, error) {
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
		Logger: logrus.New().WithContext(context.Background()),
	})
	// TODO(ian): How do we ensure this gets cleaned up?
	go func() {
		tk.Start(ctx)
	}()

	return &System{
		SystemEntry: e,
		log:         logrus.New().WithContext(context.Background()),
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
	h.log.Printf("Endpoint: %s, SystemID: %s", ep, systemID)
	r = r.WithContext(context.WithValue(r.Context(), web.SystemIDKey, systemID))

	v, ok := h.systems[systemID]
	if !ok {
		writeError(w, "system id not found", http.StatusBadGateway)
		return
	}

	// Use the authenticated session.
	token, err := v.tk.GetToken(r.Context())
	if err != nil {
		writeError(w, "failed to authenticate", http.StatusUnauthorized)
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

func (h *PowerFlexHandler) spoofLoginRequest(w http.ResponseWriter, r *http.Request) {
	_, span := trace.SpanFromContext(r.Context()).Tracer().Start(r.Context(), "spoofLoginRequest")
	defer span.End()
	w.Write([]byte("hellofromkaravi"))
}

func writeError(w http.ResponseWriter, msg string, code int) error {
	log.Printf("proxy: powerflex_handler: writing error:  %d: %s", code, msg)
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
		return err
	}
	return nil
}

func (s *System) volumeCreateHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := trace.SpanFromContext(r.Context()).Tracer().Start(r.Context(), "volumeCreateHandler")
		defer span.End()

		var systemID string
		if v := r.Context().Value(web.SystemIDKey); v != nil {
			var ok bool
			if systemID, ok = v.(string); !ok {
				writeError(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
		}

		// Read the body.
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeError(w, "failed to read body", http.StatusInternalServerError)
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
			s.log.Errorf("proxy: decoding create volume request: %+v", err)
			writeError(w, "failed to extract cap data", http.StatusBadRequest)
			return
		}
		body.VolumeSize, err = strconv.ParseInt(body.VolumeSizeInKb, 0, 64)
		if err != nil {
			writeError(w, "failed to parse capacity", http.StatusBadRequest)
			return
		}

		// Convert the StoragePoolID into more friendly Name.
		spName, err := s.spc.GetStoragePoolNameByID(ctx, body.StoragePoolID)
		if err != nil {
			writeError(w, "failed to query pool name from id", http.StatusBadRequest)
			return
		}
		log.Printf("Storagepool: %v -> %v", body.StoragePoolID, spName)

		// Get the remote host address.
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			writeError(w, "failed to parse remote host", http.StatusInternalServerError)
			return
		}
		s.log.Printf("RemoteAddr: %s", host)

		pvName := r.Header.Get(HeaderPVName)
		// Update metrics counter for volumes requested.
		//volReqCount.Add(pvName, 1)

		// Ask OPA to make a decision
		var requestBody map[string]json.RawMessage
		err = json.NewDecoder(bytes.NewReader(b)).Decode(&requestBody)
		if err != nil {
			writeError(w, "decoding request body", http.StatusInternalServerError)
			return
		}

		jwtValue := r.Context().Value(web.JWTKey)
		jwtToken, ok := jwtValue.(*jwt.Token)
		if !ok {
			panic("incorrect type for a jwt token")
		}

		s.log.Println("Asking OPA...")
		// Request policy decision from OPA
		ans, err := decision.Can(func() decision.Query {
			return decision.Query{
				Host: opaHost,
				// TODO(ian): This will need to be namespaced under "powerflex".
				Policy: "/karavi/volumes/create",
				Input: map[string]interface{}{
					"claims":          jwtToken.Claims,
					"request":         requestBody,
					"storagepool":     spName,
					"storagesystemid": systemID,
				},
			}
		})
		var opaResp OPAResponse
		err = json.NewDecoder(bytes.NewReader(ans)).Decode(&opaResp)
		if err != nil {
			writeError(w, "decoding opa request body", http.StatusInternalServerError)
			return
		}
		log.Printf("OPA Response: %v", string(ans))
		if resp := opaResp.Result; !resp.Response.Allowed {
			switch {
			case resp.Response.Status.Reason == "":
				writeError(w, "proxy is not configured", http.StatusInternalServerError)
			case resp.Claims.Group == "":
				writeError(w, "invalid token", http.StatusUnauthorized)
			default:
				writeError(w, fmt.Sprintf("request denied: %v", resp.Response.Status.Reason), http.StatusBadRequest)
			}
			return
		}

		// At this point, the request has been approved.
		qr := quota.Request{
			StoragePoolID: spName,
			Group:         opaResp.Result.Claims.Group,
			VolumeName:    pvName,
			Capacity:      body.VolumeSizeInKb,
		}

		s.log.Println("Approving request...")
		// Ask our quota enforcer if it approves the request.
		ok, err = enf.ApproveRequest(ctx, qr, opaResp.Result.Quota)
		if err != nil {
			s.log.Printf("failed to approve request: %+v", err)
			writeError(w, "failed to approve request", http.StatusInternalServerError)
			return
		}
		if !ok {
			s.log.Println("request was not approved")
			writeError(w, "request denied: not enough quota", http.StatusInsufficientStorage)
			return
		}

		// At this point, the request has been approved.

		// Reset the original request
		r.Body.Close()
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

func (s *System) volumeDeleteHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := trace.SpanFromContext(r.Context()).Tracer().Start(r.Context(), "volumeDeleteHandler")
		defer span.End()

		// Extract the volume ID from the request URI in order to get the
		// the name.
		// TODO(ian): have the CSI driver send both name and ID to remove
		// the need for us to figure it out.
		var id string
		z := strings.SplitN(r.URL.Path, "/", 5)
		if len(z) > 3 {
			id = z[3]
		}
		pvName, err := func() (*types.Volume, error) {
			c, err := goscaleio.NewClientWithArgs(s.Endpoint, "", false, false)
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
			s.log.Printf("ERROR: %v", err)
			writeError(w, "query name by volid", http.StatusInternalServerError)
			return
		}

		spName, err := s.spc.GetStoragePoolNameByID(ctx, pvName.StoragePoolID)
		if err != nil {
			writeError(w, "failed to query pool name from id", http.StatusBadRequest)
			return
		}

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeError(w, "failed to read body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		jwtValue := r.Context().Value(web.JWTKey)
		jwtToken, ok := jwtValue.(*jwt.Token)
		if !ok {
			panic("incorrect type for a jwt token")
		}

		var requestBody map[string]json.RawMessage
		err = json.NewDecoder(bytes.NewReader(b)).Decode(&requestBody)
		if err != nil {
			writeError(w, "decoding request body", http.StatusInternalServerError)
			return
		}
		// Request policy decision from OPA
		ans, err := decision.Can(func() decision.Query {
			return decision.Query{
				Host:   opaHost,
				Policy: "/karavi/volumes/delete",
				Input: map[string]interface{}{
					"claims": jwtToken.Claims,
				},
			}
		})

		var opaResp OPAResponse
		err = json.NewDecoder(bytes.NewReader(ans)).Decode(&opaResp)
		if err != nil {
			writeError(w, "decoding opa request body", http.StatusInternalServerError)
			return
		}
		log.Printf("OPA Response: %v", string(ans))
		if resp := opaResp.Result; !resp.Response.Allowed {
			switch {
			case resp.Claims.Group == "":
				writeError(w, "invalid token", http.StatusUnauthorized)
			default:
				writeError(w, fmt.Sprintf("request denied: %v", resp.Response.Status.Reason), http.StatusBadRequest)
			}
			return
		}

		qr := quota.Request{
			StoragePoolID: spName,
			Group:         opaResp.Result.Claims.Group,
			VolumeName:    pvName.Name,
		}
		ok, err = enf.DeleteRequest(r.Context(), qr)
		if err != nil {
			writeError(w, "delete request failed", http.StatusInternalServerError)
			return
		}
		if !ok {
			writeError(w, "request denied", http.StatusForbidden)
			return
		}

		// Reset the original request
		r.Body.Close()
		r.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		sw := &web.StatusWriter{
			ResponseWriter: w,
		}
		r = r.WithContext(ctx)
		next.ServeHTTP(sw, r)

		log.Printf("Resp: Code: %d", sw.Status)
		switch sw.Status {
		case http.StatusOK:
			log.Println("Publish deleted")
			ok, err := enf.PublishDeleted(r.Context(), qr)
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

func (s *System) volumeUnmapHandler(next http.Handler, enf *quota.RedisEnforcement, opaHost string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, span := trace.SpanFromContext(r.Context()).Tracer().Start(r.Context(), "volumeUnmapHandler")
		defer span.End()

		var id string
		z := strings.SplitN(r.URL.Path, "/", 5)
		if len(z) > 3 {
			id = z[3]
		}
		pvName, err := func() (*types.Volume, error) {
			c, err := goscaleio.NewClientWithArgs(s.Endpoint, "", false, false)
			if err != nil {
				return nil, err
			}
			token, err := s.tk.GetToken(ctx)
			c.SetToken(token)

			id = strings.TrimPrefix(id, "Volume::")
			s.log.Printf("Looking for volume to unmap: %v", id)
			vols, err := c.GetVolume("", id, "", "", false)
			s.log.Printf("Found volumes: %v", vols)
			if err != nil {
				return nil, err
			}

			if len(vols) == 0 {
				return nil, errors.New("No volume")
			}

			return vols[0], nil
		}()
		if err != nil {
			writeError(w, "query name by volid", http.StatusInternalServerError)
			return
		}

		spName, err := s.spc.GetStoragePoolNameByID(ctx, pvName.StoragePoolID)
		if err != nil {
			writeError(w, "failed to query pool name from id", http.StatusBadRequest)
			return
		}

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeError(w, "failed to read body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		jwtValue := r.Context().Value(web.JWTKey)
		jwtToken, ok := jwtValue.(*jwt.Token)
		if !ok {
			panic("incorrect type for a jwt token")
		}

		var requestBody map[string]json.RawMessage
		err = json.NewDecoder(bytes.NewReader(b)).Decode(&requestBody)
		if err != nil {
			s.log.Printf("decoding request body: %+v", err)
			writeError(w, "decoding request body", http.StatusInternalServerError)
			return
		}
		// Request policy decision from OPA
		ans, err := decision.Can(func() decision.Query {
			return decision.Query{
				Host:   opaHost,
				Policy: "/karavi/volumes/unmap",
				Input: map[string]interface{}{
					"claims": jwtToken.Claims,
				},
			}
		})

		var opaResp OPAResponse
		err = json.NewDecoder(bytes.NewReader(ans)).Decode(&opaResp)
		if err != nil {
			s.log.Printf("decoding opa request body: %+v", err)
			writeError(w, "decoding opa request body", http.StatusInternalServerError)
			return
		}
		log.Printf("OPA Response: %v", string(ans))
		if resp := opaResp.Result; !resp.Response.Allowed {
			switch {
			case resp.Claims.Group == "":
				writeError(w, "invalid token", http.StatusUnauthorized)
			default:
				writeError(w, fmt.Sprintf("request denied: %v", resp.Response.Status.Reason), http.StatusBadRequest)
			}
			return
		}

		qr := quota.Request{
			StoragePoolID: spName,
			Group:         opaResp.Result.Claims.Group,
			VolumeName:    pvName.Name,
		}
		ok, err = enf.ValidateOwnership(ctx, qr)
		if err != nil {
			writeError(w, "unmap request failed", http.StatusInternalServerError)
			return
		}
		if !ok {
			writeError(w, "unmap denied", http.StatusForbidden)
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
