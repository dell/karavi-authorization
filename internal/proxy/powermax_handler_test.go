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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"karavi-authorization/internal/quota"
	"karavi-authorization/internal/token"
	"karavi-authorization/internal/token/jwx"
	"karavi-authorization/internal/web"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

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
		r.Header.Set("Forwarded", "for=https://10.0.0.1;1234567890")
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
		r.Header.Set("Forwarded", "for=https://10.0.0.1;0000000000") // pass unknown system ID
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

		err := sut.UpdateSystems(context.Background(), strings.NewReader(systemJSON(fakeUni.URL)))
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodGet,
			"/univmax/restapi/91/sloprovisioning/symmetrix/1234567890/storagegroup/csi-CSM-Bronze-SRP_1-SG/",
			nil)
		r.Header.Set("Forwarded", "for=https://10.0.0.1;1234567890")
		addJWTToRequestHeader(t, r)
		w := httptest.NewRecorder()

		web.Adapt(sut, web.AuthMW(discardLogger(), jwx.NewTokenManager(jwx.HS256), "secret")).ServeHTTP(w, r)

		if !gotCalled {
			t.Errorf("wanted fake unisphere to be called, but it wasn't")
		}
		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("status: got %d, want 200", w.Result().StatusCode)
		}
	})
	t.Run("it intercepts volume create requests", func(t *testing.T) {
		var (
			gotExistsKey, gotExistsField string
		)
		fakeUni := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Logf("fake unisphere received: %s %s", r.Method, r.URL)
			if r.URL.Path == "/univmax/restapi/91/sloprovisioning/symmetrix/1234567890/storagegroup/csi-CSM-Bronze-SRP_1-SG" {
				b, err := ioutil.ReadFile("testdata/powermax_create_volume_response.json")
				if err != nil {
					t.Fatal(err)
				}
				w.Write(b)
				return
			}
		}))
		enf := quota.NewRedisEnforcement(context.Background(), quota.WithDB(&quota.FakeRedis{
			HExistsFn: func(key, field string) (bool, error) {
				gotExistsKey, gotExistsField = key, field
				return true, nil
			},
			EvalIntFn: func(_ string, _ []string, _ ...interface{}) (int, error) {
				return 1, nil
			},
		}))
		sut := buildPowerMaxHandler(t,
			withOPAServer(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{ "result": { "allow": true } }`)
			}),
			withEnforcer(enf),
		)
		err := sut.UpdateSystems(context.Background(), strings.NewReader(systemJSON(fakeUni.URL)))
		if err != nil {
			t.Fatal(err)
		}
		payloadBytes, err := ioutil.ReadFile("testdata/powermax_create_volume_payload.json")
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPut,
			"/univmax/restapi/91/sloprovisioning/symmetrix/1234567890/storagegroup/csi-CSM-Bronze-SRP_1-SG/",
			bytes.NewReader(payloadBytes))
		r.Header.Set("Forwarded", "for=https://10.0.0.1;1234567890")
		addJWTToRequestHeader(t, r)
		w := httptest.NewRecorder()

		web.Adapt(sut, web.AuthMW(discardLogger(), jwx.NewTokenManager(jwx.HS256), "secret")).ServeHTTP(w, r)

		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("status: got %d, want 200", w.Result().StatusCode)
		}
		wantExistsKey := "quota:powermax:1234567890:SRP_1:karavi-tenant:data"
		if gotExistsKey != wantExistsKey {
			t.Errorf("exists key: got %q, want %q", gotExistsKey, wantExistsKey)
		}
		wantExistsField := "vol:csi-CSM-pmax-9c79d51b18:approved"
		if gotExistsField != wantExistsField {
			t.Errorf("exists field: got %q, want %q", gotExistsField, wantExistsField)
		}
	})
	t.Run("it intercepts volume modify requests", func(t *testing.T) {
		var (
			gotExistsKey, gotExistsField string
		)
		fakeUni := fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Logf("fake unisphere received: %s %s", r.Method, r.URL)
			switch r.URL.Path {
			case "/univmax/restapi/91/sloprovisioning/symmetrix/1234567890/volume/003E4":
				b, err := ioutil.ReadFile("testdata/powermax_getvolumebyid_response.json")
				if err != nil {
					t.Fatal(err)
				}
				w.Write(b)
				return
			case "/univmax/restapi/91/sloprovisioning/symmetrix/1234567890/storagegroup/csi-CSM-Bronze-SRP_1-SG":
				b, err := ioutil.ReadFile("testdata/powermax_getstoragegroup_response.json")
				if err != nil {
					t.Fatal(err)
				}
				w.Write(b)
				return
			}
		}))
		enf := quota.NewRedisEnforcement(context.Background(), quota.WithDB(&quota.FakeRedis{
			HExistsFn: func(key, field string) (bool, error) {
				gotExistsKey, gotExistsField = key, field
				return true, nil
			},
			EvalIntFn: func(_ string, _ []string, _ ...interface{}) (int, error) {
				return 1, nil
			},
		}))
		sut := buildPowerMaxHandler(t,
			withOPAServer(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{ "result": { "allow": true } }`)
			}),
			withEnforcer(enf),
		)
		err := sut.UpdateSystems(context.Background(), strings.NewReader(systemJSON(fakeUni.URL)))
		if err != nil {
			t.Fatal(err)
		}
		payloadBytes, err := ioutil.ReadFile("testdata/powermax_modify_volume.json")
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPut,
			"/univmax/restapi/91/sloprovisioning/symmetrix/1234567890/volume/003E4/",
			bytes.NewReader(payloadBytes))
		r.Header.Set("Forwarded", "for=https://10.0.0.1;1234567890")
		addJWTToRequestHeader(t, r)
		w := httptest.NewRecorder()

		web.Adapt(sut, web.AuthMW(discardLogger(), jwx.NewTokenManager(jwx.HS256), "secret")).ServeHTTP(w, r)

		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("status: got %d, want 200", w.Result().StatusCode)
		}
		wantExistsKey := "quota:powermax:1234567890:SRP_1:karavi-tenant:data"
		if gotExistsKey != wantExistsKey {
			t.Errorf("exists key: got %q, want %q", gotExistsKey, wantExistsKey)
		}
		wantExistsField := "vol:csi-CSM-pmax-9c79d51b18:created"
		if gotExistsField != wantExistsField {
			t.Errorf("exists field: got %q, want %q", gotExistsField, wantExistsField)
		}
	})
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

			err := sut.UpdateSystems(context.Background(), tt.given)

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

type powermaxHandlerOption func(*testing.T, *PowerMaxHandler)

func withUnisphereServer(h http.HandlerFunc) powermaxHandlerOption {
	return func(t *testing.T, pmh *PowerMaxHandler) {
		fakeUnisphere := fakeServer(t, h)
		err := pmh.UpdateSystems(context.Background(), strings.NewReader(systemJSON(fakeUnisphere.URL)))
		if err != nil {
			t.Fatal(err)
		}
	}
}

func withOPAServer(h http.HandlerFunc) powermaxHandlerOption {
	return func(t *testing.T, pmh *PowerMaxHandler) {
		fakeOPA := fakeServer(t, h)
		pmh.opaHost = hostPortFromFakeServer(t, fakeOPA)
	}
}

func withEnforcer(v *quota.RedisEnforcement) powermaxHandlerOption {
	return func(t *testing.T, pmh *PowerMaxHandler) {
		pmh.enforcer = v
	}
}

func withLogger(logger *logrus.Entry) powermaxHandlerOption {
	return func(t *testing.T, pmh *PowerMaxHandler) {
		pmh.log = logger
	}
}

func withSystem(s *PowerMaxSystem) powermaxHandlerOption {
	return func(t *testing.T, pmh *PowerMaxHandler) {
		pmh.systems["1234567890"] = s
	}
}

func buildPowerMaxHandler(t *testing.T, opts ...powermaxHandlerOption) *PowerMaxHandler {
	defaultOptions := []powermaxHandlerOption{
		withLogger(testLogger()), // order matters for this one.
		withUnisphereServer(func(w http.ResponseWriter, r *http.Request) {}),
		withOPAServer(func(w http.ResponseWriter, r *http.Request) {}),
	}

	ret := PowerMaxHandler{}

	for _, opt := range defaultOptions {
		opt(t, &ret)
	}
	for _, opt := range opts {
		opt(t, &ret)
	}

	return &ret
}

func testLogger() *logrus.Entry {
	logger := logrus.New().WithContext(context.Background())
	logger.Logger.SetOutput(os.Stdout)
	return logger
}

func hostPortFromFakeServer(t *testing.T, testServer *httptest.Server) string {
	parsedURL, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	return parsedURL.Host
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

func systemObject(endpoint string) SystemConfig {
	return SystemConfig{
		"powermax": Family{
			"1234567890": SystemEntry{
				Endpoint: endpoint,
				User:     "smc",
				Password: "smc",
				Insecure: true,
			},
		},
	}
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
