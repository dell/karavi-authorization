package web_test

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

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"karavi-authorization/internal/proxy"
	"karavi-authorization/internal/quota"
	"karavi-authorization/internal/token"
	"karavi-authorization/internal/token/jwx"
	"karavi-authorization/internal/web"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redis"
	redisclient "github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
)

// SystemConfig is a map of string keys to a Family of backend storage systems
type SystemConfig map[string]Family

// Family is map of string keys to a SystemEntry
type Family map[string]SystemEntry

// SystemEntry holds information for a backend storage system
type SystemEntry struct {
	Endpoint string `json:"endpoint"`
	User     string `json:"user"`
	Password string `json:"password"`
	Insecure bool   `json:"insecure"`
}

func init() {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
}

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

func TestPowerFlex(t *testing.T) {
	t.Run("it spoofs login requests from clients", func(t *testing.T) {
		// Logging.
		log := logrus.New().WithContext(context.Background())
		log.Logger.SetOutput(os.Stdout)
		// Prepare the login request.
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/login", nil)
		// Build a fake powerflex backend, since it will try to login for real.
		// We'll use the URL of this test server as part of the systems config.
		fakePowerFlex := buildTestTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		}))
		fakeOPA := buildTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"result": {"allow": true}}`))
		}))
		// Add headers that the sidecar-proxy would add, in order to identify
		// the request as intended for a PowerFlex with the given systemID.
		r.Header.Add("Forwarded", "by=csi-vxflexos")
		r.Header.Add("Forwarded", fmt.Sprintf("for=%s;542a2d5f5122210f", fakePowerFlex.URL))
		// Create the router and assign the appropriate handlers.
		rtr := newTestRouter()
		// Create the PowerFlex handler and configure it with a system
		// where the endpoint is our test server.
		powerFlexHandler := proxy.NewPowerFlexHandler(log, nil, hostPort(t, fakeOPA.URL))
		powerFlexHandler.UpdateSystems(context.Background(), strings.NewReader(fmt.Sprintf(`
	{
	  "powerflex": {
	    "542a2d5f5122210f": {
	      "endpoint": "%s",
	      "user": "admin",
	      "pass": "Password123",
	      "insecure": true
	    }
	  }
	}
	`, fakePowerFlex.URL)), logrus.New().WithContext(context.Background()))
		systemHandlers := map[string]http.Handler{
			"powerflex": web.Adapt(powerFlexHandler),
		}
		dh := proxy.NewDispatchHandler(log, systemHandlers)
		rtr.ProxyHandler = dh
		h := web.Adapt(rtr.Handler(), web.CleanMW())

		h.ServeHTTP(w, r)

		if got, want := w.Result().StatusCode, http.StatusOK; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		// This response should come from our PowerFlex handler, NOT the (fake)
		// PowerFlex itself.
		got := string(w.Body.Bytes())
		want := "hellofromkaravi"
		if !strings.Contains(got, want) {
			t.Errorf("got %q, expected response body to contain %q", got, want)
		}
	})

	t.Run("it proxies immutable requests to the PowerFlex", func(t *testing.T) {
		// Logging.
		log := logrus.New().WithContext(context.Background())
		log.Logger.SetOutput(os.Stdout)
		// Prepare the login request.
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/version/", nil)
		// Build a fake powerflex backend, since it will try to login for real.
		// We'll use the URL of this test server as part of the systems config.
		done := make(chan struct{})
		fakePowerFlex := buildTestTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Logf("Test request path is %q", r.URL.Path)
			if r.URL.Path == "/api/version/" {
				done <- struct{}{}
			}
		}))
		fakeOPA := buildTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"result": {"allow": true}}`))
		}))
		// Add headers that the sidecar-proxy would add, in order to identify
		// the request as intended for a PowerFlex with the given systemID.
		r.Header.Add("Forwarded", "by=csi-vxflexos")
		r.Header.Add("Forwarded", fmt.Sprintf("for=https://%s;542a2d5f5122210f", fakePowerFlex.URL))
		// Create the router and assign the appropriate handlers.
		rtr := newTestRouter()
		// Create the PowerFlex handler and configure it with a system
		// where the endpoint is our test server.
		powerFlexHandler := proxy.NewPowerFlexHandler(log, nil, hostPort(t, fakeOPA.URL))
		powerFlexHandler.UpdateSystems(context.Background(), strings.NewReader(fmt.Sprintf(`
		{
		  "powerflex": {
		    "542a2d5f5122210f": {
		      "endpoint": "%s",
		      "user": "admin",
		      "pass": "Password123",
		      "insecure": true
		    }
		  }
		}
		`, fakePowerFlex.URL)), logrus.New().WithContext(context.Background()))
		systemHandlers := map[string]http.Handler{
			"powerflex": web.Adapt(powerFlexHandler),
		}
		dh := proxy.NewDispatchHandler(log, systemHandlers)
		rtr.ProxyHandler = dh
		h := web.Adapt(rtr.Handler(), web.CleanMW())

		go func() {
			h.ServeHTTP(w, r)
			done <- struct{}{}
		}()
		<-done
		<-done // we also need to wait for the HTTP request to fully complete.

		if got, want := w.Result().StatusCode, http.StatusOK; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("it denies tenant request to remove volume that tenant does not own", func(t *testing.T) {
		// Logging.
		log := logrus.New().WithContext(context.Background())
		log.Logger.SetOutput(os.Stdout)

		// Token manager
		tm := jwx.NewTokenManager(jwx.HS256)

		// Prepare tenant A's token
		// Create the claims
		claimsA := token.Claims{
			Issuer:    "com.dell.karavi",
			ExpiresAt: time.Now().Add(30 * time.Second).Unix(),
			Audience:  "karavi",
			Subject:   "Alice",
			Roles:     "DevTesting",
			Group:     "TestingGroup",
		}

		tokenA, err := tm.NewWithClaims(claimsA)
		if err != nil {
			t.Fatal(err)
		}

		// Prepare tenant B's token
		// Create the claims
		claimsB := token.Claims{
			Issuer:    "com.dell.karavi",
			ExpiresAt: time.Now().Add(30 * time.Second).Unix(),
			Audience:  "karavi",
			Subject:   "Bob",
			Roles:     "DevTesting",
			Group:     "TestingGroup",
		}

		tokenB, err := tm.NewWithClaims(claimsB)
		if err != nil {
			t.Fatal(err)
		}

		// Prepare the create volume request.
		createBody := struct {
			VolumeSize     int64
			VolumeSizeInKb string `json:"volumeSizeInKb"`
			StoragePoolID  string `json:"storagePoolId"`
			Name           string `json:"name"`
		}{
			VolumeSize:     10,
			VolumeSizeInKb: "10",
			StoragePoolID:  "3df6b86600000000",
			Name:           "TestVolume",
		}
		data, err := json.Marshal(createBody)
		if err != nil {
			t.Fatal(err)
		}
		payload := bytes.NewBuffer(data)

		wVolCreate := httptest.NewRecorder()
		rVolCreate := httptest.NewRequest(http.MethodPost, "/api/types/Volume/instances", payload)
		rVolCreateContext := context.WithValue(context.Background(), web.JWTKey, tokenA)
		rVolCreateContext = context.WithValue(rVolCreateContext, web.JWTTenantName, "TestingGroup")
		rVolCreate = rVolCreate.WithContext(rVolCreateContext)

		// Prepare the remove volume request.
		removeBody := struct {
			RemoveMode string `json:"removeMode"`
		}{
			RemoveMode: "ONLY_ME",
		}
		data, err = json.Marshal(removeBody)
		if err != nil {
			t.Fatal(err)
		}
		payload = bytes.NewBuffer(data)
		wVolDel := httptest.NewRecorder()
		rVolDel := httptest.NewRequest(http.MethodPost, "/api/instances/Volume::000000000000001/action/removeVolume", payload)
		rVolDelContext := context.WithValue(context.Background(), web.JWTKey, tokenB)
		rVolDelContext = context.WithValue(rVolDelContext, web.JWTTenantName, "TestingGroup")
		rVolDel = rVolDel.WithContext(rVolDelContext)

		// Build a fake powerflex backend, since it will try to create and delete volumes for real.
		// We'll use the URL of this test server as part of the systems config.
		fakePowerFlex := buildTestTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/types/Volume/instances/":
				w.Write([]byte(`{"id":"000000000000001", "name": "TestVolume"}`))
			case "/api/instances/Volume::000000000000001":
				w.Write([]byte(`{"sizeInKb":10, "storagePoolId":"3df6b86600000000", "name": "TestVolume"}`))
			case "/api/login":
				w.Write([]byte("token"))
			case "/api/version":
				w.Write([]byte("3.5"))
			case "/api/types/StoragePool/instances":
				w.Write([]byte(`[{"protectionDomainId": "75b661b400000000", "mediaType": "HDD", "id": "3df6b86600000000", "name": "TestPool"}]`))
			default:
				t.Errorf("Unexpected api call to fake PowerFlex: %v", r.URL.Path)
			}
		}))
		fakeOPA := buildTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Logf("Incoming OPA request: %v", r.URL.Path)
			switch r.URL.Path {
			case "/v1/data/karavi/authz/url":
				w.Write([]byte(`{"result": {"allow": true}}`))
			case "/v1/data/karavi/volumes/create":
				w.Write([]byte(`{"result": {"allow": true, "permitted_roles": {"role": 9999999}}}`))
			case "/v1/data/karavi/volumes/delete":
				w.Write([]byte(`{"result": { "response": {"allowed": true, "status": {"reason": "ok"}}, "token": {"group": "TestingGroup"}, "quota": 99999}}`))
			}
		}))

		// Add headers that the sidecar-proxy would add, in order to identify
		// the request as intended for a PowerFlex with the given systemID.
		rVolCreate.Header.Add("Forwarded", "by=csi-vxflexos")
		rVolCreate.Header.Add("Forwarded", fmt.Sprintf("for=%s;542a2d5f5122210f", fakePowerFlex.URL))

		// Add headers that the sidecar-proxy would add, in order to identify
		// the request as intended for a PowerFlex with the given systemID.
		rVolDel.Header.Add("Forwarded", "by=csi-vxflexos")
		rVolDel.Header.Add("Forwarded", fmt.Sprintf("for=%s;542a2d5f5122210f", fakePowerFlex.URL))

		// Create the router and assign the appropriate handlers.
		rtr := newTestRouter()
		// Create a redis enforcer
		rdb := testCreateRedisInstance(t)
		enf := quota.NewRedisEnforcement(context.Background(), quota.WithRedis(rdb))

		// Create the PowerFlex handler and configure it with a system
		// where the endpoint is our test server.
		powerFlexHandler := proxy.NewPowerFlexHandler(log, enf, hostPort(t, fakeOPA.URL))
		powerFlexHandler.UpdateSystems(context.Background(), strings.NewReader(fmt.Sprintf(`
		{
		  "powerflex": {
		    "542a2d5f5122210f": {
		      "endpoint": "%s",
		      "user": "admin",
		      "pass": "Password123",
		      "insecure": true
		    }
		  }
		}
		`, fakePowerFlex.URL)), logrus.New().WithContext(context.Background()))
		systemHandlers := map[string]http.Handler{
			"powerflex": web.Adapt(powerFlexHandler),
		}
		dh := proxy.NewDispatchHandler(log, systemHandlers)
		rtr.ProxyHandler = dh
		h := web.Adapt(rtr.Handler(), web.CleanMW())

		h.ServeHTTP(wVolCreate, rVolCreate)
		h.ServeHTTP(wVolDel, rVolDel)

		if got, want := wVolCreate.Result().StatusCode, http.StatusOK; got != want {
			fmt.Printf("Create request: %v\n", *rVolCreate)
			fmt.Printf("Create response: %v\n", string(wVolCreate.Body.Bytes()))
			t.Errorf("got %v, want %v", got, want)
		}

		if got, want := wVolDel.Result().StatusCode, http.StatusForbidden; got != want {
			fmt.Printf("Remove request: %v\n", *rVolDel)
			fmt.Printf("Remove response: %v\n", string(wVolDel.Body.Bytes()))
			t.Errorf("got %v, want %v", got, want)
		}

		// This response should come from our PowerFlex handler, NOT the (fake)
		// PowerFlex itself.
		type DeleteRequestResponse struct {
			ErrorCode      int    `json:"errorCode"`
			HttpStatusCode int    `json:"httpStatusCode"`
			Message        string `json:"message"`
		}
		got := DeleteRequestResponse{}
		err = json.Unmarshal(wVolDel.Body.Bytes(), &got)
		if err != nil {
			t.Errorf("error demarshalling volume delete request response: %v", err)
		}
		want := DeleteRequestResponse{
			ErrorCode:      403,
			HttpStatusCode: 403,
			Message:        "request denied",
		}
		if !strings.Contains(got.Message, want.Message) || got.ErrorCode != want.ErrorCode || got.HttpStatusCode != want.HttpStatusCode {
			t.Errorf("got %q, expected response body to contain %q", got, want)
		}
	})
}

func Test_handleError(t *testing.T) {
	var tests = []struct {
		name string
		fn   func() bool
		want bool
	}{
		{"nil error returns false", func() bool {
			return handleError(nil, nil, 0, nil, "")
		}, false},
		{"non-nil error, nil logger", func() bool {
			return handleError(nil, httptest.NewRecorder(), 0, errors.New("test"), "")
		}, true},
		{"non-nil logger", func() bool {
			return handleError(logrus.NewEntry(logrus.New()), httptest.NewRecorder(), 0, errors.New("test"), "")
		}, true},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := tt.fn()
			if got != tt.want {
				t.Errorf("(%s): got %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}

func TestPowerMaxHandler(t *testing.T) {
	t.Run("UpdateSystems", testPowerMaxUpdateSystems)
	t.Run("ServeHTTP", testPowerMaxServeHTTP)
}

func testPowerMaxServeHTTP(t *testing.T) {
	t.Run("it proxies requests", func(t *testing.T) {
		var gotRequestedPolicyPath string
		done := make(chan struct{})
		sut := buildPowerMaxHandler(t,
			withUnisphereServer(func(w http.ResponseWriter, r *http.Request) {
				done <- struct{}{}
			}),
			withOPAServer(func(w http.ResponseWriter, r *http.Request) {
				gotRequestedPolicyPath = r.URL.Path
				fmt.Fprintf(w, `{ "result": { "allow": true } }`)
			}),
		)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Forwarded", "for=https://1.1.1.1;1234567890")
		w := httptest.NewRecorder()

		go sut.ServeHTTP(w, r)

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for proxied request")
		}
		wantRequestedPolicyPath := "/v1/data/karavi/authz/powermax/url"
		if gotRequestedPolicyPath != wantRequestedPolicyPath {
			t.Errorf("OPAPolicyPath: got %q, want %q",
				gotRequestedPolicyPath, wantRequestedPolicyPath)
		}
	})
	t.Run("it returns 502 Bad Gateway on unknown system", func(t *testing.T) {
		sut := buildPowerMaxHandler(t)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Forwarded", "for=https://1.1.1.1;0000000000") // pass unknown system ID
		w := httptest.NewRecorder()

		sut.ServeHTTP(w, r)

		want := http.StatusBadGateway
		if got := w.Result().StatusCode; got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	})
	t.Run("it allows storage group queries", func(t *testing.T) {
		// This test case uses the same API endpoint as volume create, only
		// the difference is that it uses a GET method.
		// This test will ensure that httprouter handles both GET and PUT methods.
		var gotCalled bool
		fakeUni := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Logf("fake unisphere received: %s %s", r.Method, r.URL)
			gotCalled = true
		}))
		sut := buildPowerMaxHandler(t, withOPAServer(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `{ "result": { "allow": true } }`)
		}))

		err := sut.UpdateSystems(context.Background(), strings.NewReader(systemJSON(fakeUni.URL)), logrus.New().WithContext(context.Background()))
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodGet,
			"/univmax/restapi/91/sloprovisioning/symmetrix/1234567890/storagegroup/csi-CSM-Bronze-SRP_1-SG/",
			nil)
		r.Header.Set("Forwarded", "for=https://1.1.1.1;1234567890")
		addJWTToRequestHeader(t, r)
		w := httptest.NewRecorder()

		web.Adapt(sut, web.AuthMW(discardLogger(), jwx.NewTokenManager(jwx.HS256))).ServeHTTP(w, r)

		if !gotCalled {
			t.Errorf("wanted fake unisphere to be called, but it wasn't")
		}
		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("status: got %d, want 200", w.Result().StatusCode)
		}
	})
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

func newTestRouter() *web.Router {
	noopHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	return &web.Router{
		ProxyHandler:               noopHandler,
		RolesHandler:               noopHandler,
		TokenHandler:               noopHandler,
		ClientInstallScriptHandler: noopHandler,
	}
}

func buildTestTLSServer(t *testing.T, h ...http.Handler) *httptest.Server {
	var handler http.Handler
	switch len(h) {
	case 0:
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	case 1:
		handler = h[0]
	}
	ts := httptest.NewTLSServer(handler)
	t.Cleanup(func() {
		ts.Close()
	})
	return ts
}

func testPowerMaxUpdateSystems(t *testing.T) {
	var tests = []struct {
		name                string
		given               io.Reader
		expectedErr         error
		expectedSystemCount int
	}{
		{"invalid json", strings.NewReader(""), io.EOF, 0},
		{"remove system", strings.NewReader("{}"), nil, 0},
		{"add system", strings.NewReader(systemJSON("test")), nil, 1},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sut := buildPowerMaxHandler(t)

			err := sut.UpdateSystems(context.Background(), tt.given, logrus.New().WithContext(context.Background()))

			if tt.expectedErr != nil {
				if err != tt.expectedErr {
					t.Fatalf("UpdateSystems: got err = %v, want = %v", err, tt.expectedErr)
				}
				return
			}
			want := tt.expectedSystemCount
			if got := len(sut.systems); got != want {
				t.Errorf("%s: got system count %d, want %d", tt.name, got, want)
			}
		})
	}
}

func buildPowerMaxHandler(t *testing.T, opts ...powermaxHandlerOption) *proxy.PowerMaxHandler {
	defaultOptions := []powermaxHandlerOption{
		withLogger(testLogger()), // order matters for this one.
		withUnisphereServer(func(w http.ResponseWriter, r *http.Request) {}),
		withOPAServer(func(w http.ResponseWriter, r *http.Request) {}),
	}
	pmLog := logrus.New().WithContext(context.Background())
	pmLog.Logger.SetOutput(os.Stdout)

	pmRdb := testCreateRedisInstance(t)
	pmEnf := quota.NewRedisEnforcement(context.Background(), quota.WithRedis(pmRdb))

	// fakeOPA := buildTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	// 	t.Logf("Incoming OPA request: %v", r.URL.Path)
	// 	switch r.URL.Path {
	// 	case "/v1/data/karavi/authz/url":
	// 		w.Write([]byte(`{"result": {"allow": true}}`))
	// 	case "/v1/data/karavi/volumes/create":
	// 		w.Write([]byte(`{"result": {"allow": true, "permitted_roles": {"role": 9999999}}}`))
	// 	case "/v1/data/karavi/volumes/delete":
	// 		w.Write([]byte(`{"result": { "response": {"allowed": true, "status": {"reason": "ok"}}, "token": {"group": "TestingGroup"}, "quota": 99999}}`))
	// 	}
	// }))
	fakeOPA := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{ "result": { "allow": true } }`)
	}))
	//	pmh.opaHost = hostPortFromFakeServer(t, fakeOPA)

	//ret := proxy.NewPowerMaxHandler(pmLog, pmEnf, hostPort(t, fakeOPA.URL))
	ret := proxy.NewPowerMaxHandler(pmLog, pmEnf, hostPortFromFakeServer(t, fakeOPA))

	for _, opt := range defaultOptions {
		opt(t, ret)
	}
	for _, opt := range opts {
		opt(t, ret)
	}

	return ret
}

type powermaxHandlerOption func(*testing.T, *proxy.PowerMaxHandler)

func withUnisphereServer(h http.HandlerFunc) powermaxHandlerOption {
	return func(t *testing.T, pmh *proxy.PowerMaxHandler) {
		fakeUnisphere := fakeServer(t, h)
		err := pmh.UpdateSystems(context.Background(), strings.NewReader(systemJSON(fakeUnisphere.URL)), logrus.New().WithContext(context.Background()))
		if err != nil {
			t.Fatal(err)
		}
	}
}

func withOPAServer(h http.HandlerFunc) powermaxHandlerOption {
	return func(t *testing.T, pmh *proxy.PowerMaxHandler) {
		fakeOPA := fakeServer(t, h)
		//pmh.opaHost = hostPortFromFakeServer(t, fakeOPA)
	}
}

func withLogger(logger *logrus.Entry) powermaxHandlerOption {
	return func(t *testing.T, pmh *proxy.PowerMaxHandler) {
		pmh.log = logger
	}
}

func testLogger() *logrus.Entry {
	logger := logrus.New().WithContext(context.Background())
	logger.Logger.SetOutput(os.Stdout)
	return logger
}

func fakeServer(t *testing.T, h http.Handler) *httptest.Server {
	s := httptest.NewServer(h)
	t.Cleanup(func() {
		s.Close()
	})
	return s
}

func systemJSON(endpoint string) string {
	return fmt.Sprintf(`{
	  "powermax": {
	    "1234567890": {
	      "endpoint": "%s",
	      "user": "smc",
	      "pass": "smc",
	      "insecure": true
	    }
	  }
	}
	`, endpoint)
}

func addJWTToRequestHeader(t *testing.T, r *http.Request) {
	p, err := token.Create(jwx.NewTokenManager(jwx.HS256), token.Config{
		Tenant:            "karavi-tenant",
		Roles:             []string{"us-east-1"},
		JWTSigningSecret:  "secret",
		RefreshExpiration: 999 * time.Minute,
		AccessExpiration:  999 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}

	r.Header.Set("Authorization", "Bearer "+p.Access)
}

func discardLogger() *logrus.Entry {
	logger := logrus.New()
	return logger.WithContext(context.Background())
}

func buildTestServer(t *testing.T, h ...http.Handler) *httptest.Server {
	var handler http.Handler
	switch len(h) {
	case 0:
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	case 1:
		handler = h[0]
	}
	ts := httptest.NewServer(handler)
	t.Cleanup(func() {
		ts.Close()
	})
	return ts
}

func hostPort(t *testing.T, u string) string {
	t.Helper()
	p, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	return p.Host
}

func hostPortFromFakeServer(t *testing.T, testServer *httptest.Server) string {
	parsedURL, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	return parsedURL.Host
}

type tb interface {
	testing.TB
}

func testCreateRedisInstance(t tb) *redis.Client {
	var rdb *redisclient.Client

	redisHost := os.Getenv("REDIS_HOST")
	redistPort := os.Getenv("REDIS_PORT")

	if redisHost != "" && redistPort != "" {
		rdb = redis.NewClient(&redis.Options{
			Addr: fmt.Sprintf("%s:%s", redisHost, redistPort),
		})
	} else {
		var retries int
		for {
			cmd := exec.Command("docker", "run",
				"--rm",
				"--name", "test-redis",
				"--net", "host",
				"--detach",
				"redis")
			b, err := cmd.CombinedOutput()
			if err != nil {
				retries++
				if retries >= 3 {
					t.Fatalf("starting redis in docker: %s, %v", string(b), err)
				}
				time.Sleep(time.Second)
				continue
			}
			break
		}

		t.Cleanup(func() {
			err := exec.Command("docker", "stop", "test-redis").Start()
			if err != nil {
				t.Fatal(err)
			}
		})

		rdb = redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
		})
	}

	// Wait for a PING before returning, or fail with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		_, err := rdb.Ping().Result()
		if err != nil {
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			default:
				time.Sleep(time.Nanosecond)
				continue
			}
		}

		break
	}

	return rdb
}
