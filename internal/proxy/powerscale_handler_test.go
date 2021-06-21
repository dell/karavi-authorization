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
	"context"
	"fmt"
	"io"
	"karavi-authorization/internal/quota"
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

func TestPowerScaleHandler(t *testing.T) {
	t.Run("UpdateSystems", testPowerScaleUpdateSystems)
	t.Run("ServeHTTP", testPowerScaleServeHTTP)
}

func testPowerScaleServeHTTP(t *testing.T) {
	t.Run("it proxies requests", func(t *testing.T) {
		var gotRequestedPolicyPath string
		done := make(chan struct{})
		m := &powerscaleHandlerOptionManager{}
		sut := buildPowerScaleHandler(t,
			m.withPowerScaleServer(func(w http.ResponseWriter, r *http.Request) {
				done <- struct{}{}
			}),
			m.withOPAServer(func(w http.ResponseWriter, r *http.Request) {
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
		wantRequestedPolicyPath := "/v1/data/karavi/authz/powerscale/url"
		if gotRequestedPolicyPath != wantRequestedPolicyPath {
			t.Errorf("OPAPolicyPath: got %q, want %q",
				gotRequestedPolicyPath, wantRequestedPolicyPath)
		}
	})
	t.Run("it returns 502 Bad Gateway on unknown system", func(t *testing.T) {
		sut := buildPowerScaleHandler(t)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Forwarded", "for=https://10.0.0.1;0000000000") // pass unknown system ID
		w := httptest.NewRecorder()

		sut.ServeHTTP(w, r)

		want := http.StatusBadGateway
		if got := w.Result().StatusCode; got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	})
	t.Run("it intercepts volume create requests", func(t *testing.T) {
		var (
			gotExistsKey, gotExistsField string
			u                            = &powerscaleUtils{}
			m                            = &powerscaleHandlerOptionManager{}
		)
		fakePowerScale := u.fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Logf("fake powerscale received: %s %s", r.Method, r.URL)
			if r.Method == http.MethodPut && r.URL.Path == "/namespace/ifs/test/volume" {
				w.WriteHeader(http.StatusOK)
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
		sut := buildPowerScaleHandler(t,
			m.withOPAServer(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{ "result": { "allow": true } }`)
			}),
			m.withEnforcer(enf),
		)
		err := sut.UpdateSystems(context.Background(), strings.NewReader(u.systemJSON(fakePowerScale.URL)))
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodPut,
			"/namespace/ifs/test/volume",
			nil)
		r.Header.Set("Forwarded", "for=https://10.0.0.1;1234567890")
		r.Header.Set(HeaderPVName, "volume")
		addJWTToRequestHeader(t, r)
		w := httptest.NewRecorder()

		web.Adapt(sut, web.AuthMW(discardLogger(), jwx.NewTokenManager(jwx.HS256), "secret")).ServeHTTP(w, r)

		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("status: got %d, want 200", w.Result().StatusCode)
		}
		wantExistsKey := "quota:powerscale:1234567890:/ifs/test:karavi-tenant:data"
		if gotExistsKey != wantExistsKey {
			t.Errorf("exists key: got %q, want %q", gotExistsKey, wantExistsKey)
		}
		wantExistsField := "vol:volume:approved"
		if gotExistsField != wantExistsField {
			t.Errorf("exists field: got %q, want %q", gotExistsField, wantExistsField)
		}
	})
	t.Run("it intercepts volume delete requests", func(t *testing.T) {
		var (
			gotVolName, gotDeleteArg string
			u                        = &powerscaleUtils{}
			m                        = &powerscaleHandlerOptionManager{}
		)
		fakePowerScale := u.fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Logf("fake powerscale received: %s %s", r.Method, r.URL)
			if r.Method == http.MethodDelete && r.URL.Path == "/namespace/ifs/test/volume" {
				w.WriteHeader(http.StatusOK)
				return
			}
		}))
		evalIntfnCount := 0
		enf := quota.NewRedisEnforcement(context.Background(), quota.WithDB(&quota.FakeRedis{
			EvalIntFn: func(_ string, _ []string, args ...interface{}) (int, error) {
				if evalIntfnCount == 1 {
					gotVolName = args[6].(string)
					gotDeleteArg = args[10].(string)
				}
				evalIntfnCount++
				return 1, nil
			},
		}))
		opaReqCount := 0
		sut := buildPowerScaleHandler(t,
			m.withOPAServer(func(w http.ResponseWriter, r *http.Request) {
				switch opaReqCount {
				case 0:
					fmt.Fprintf(w, `{ "result": { "allow": true } }`)
				case 1:
					fmt.Fprintf(w, `{ "result": { "response": { "allowed": true } } }`)
				}
				opaReqCount++
			}),
			m.withEnforcer(enf),
		)
		err := sut.UpdateSystems(context.Background(), strings.NewReader(u.systemJSON(fakePowerScale.URL)))
		if err != nil {
			t.Fatal(err)
		}
		r := httptest.NewRequest(http.MethodDelete,
			"/namespace/ifs/test/volume",
			nil)
		r.Header.Set("Forwarded", "for=https://10.0.0.1;1234567890")
		r.Header.Set(HeaderPVName, "volume")
		addJWTToRequestHeader(t, r)
		w := httptest.NewRecorder()

		web.Adapt(sut, web.AuthMW(discardLogger(), jwx.NewTokenManager(jwx.HS256), "secret")).ServeHTTP(w, r)

		if w.Result().StatusCode != http.StatusOK {
			t.Errorf("status: got %d, want 200", w.Result().StatusCode)
		}
		wantVolName := "volume"
		if gotVolName != wantVolName {
			t.Errorf("exists key: got %q, want %q", gotVolName, wantVolName)
		}
		wantDeleteArg := "deleted"
		if gotDeleteArg != wantDeleteArg {
			t.Errorf("exists field: got %q, want %q", gotDeleteArg, wantDeleteArg)
		}
	})
}

func testPowerScaleUpdateSystems(t *testing.T) {
	u := &powerscaleUtils{}
	var tests = []struct {
		name                string
		given               io.Reader
		expectedErr         error
		expectedSystemCount int
	}{
		{"invalid json", strings.NewReader(""), io.EOF, 0},
		{"remove system", strings.NewReader("{}"), nil, 0},
		{"add system", strings.NewReader(u.systemJSON("test")), nil, 1},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sut := buildPowerScaleHandler(t)

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

type powerscaleHandlerOption func(*testing.T, *PowerScaleHandler)

type powerscaleHandlerOptionManager struct{}

func (m *powerscaleHandlerOptionManager) withPowerScaleServer(h http.HandlerFunc) powerscaleHandlerOption {
	return func(t *testing.T, psh *PowerScaleHandler) {
		fakePowerScale := fakeServer(t, h)
		u := &powerscaleUtils{}
		err := psh.UpdateSystems(context.Background(), strings.NewReader(u.systemJSON(fakePowerScale.URL)))
		if err != nil {
			t.Fatal(err)
		}
	}
}

func (m *powerscaleHandlerOptionManager) withOPAServer(h http.HandlerFunc) powerscaleHandlerOption {
	return func(t *testing.T, pmh *PowerScaleHandler) {
		m := &powerscaleUtils{}
		fakeOPA := m.fakeServer(t, h)
		pmh.opaHost = m.hostPortFromFakeServer(t, fakeOPA)
	}
}

func (m *powerscaleHandlerOptionManager) withEnforcer(v *quota.RedisEnforcement) powerscaleHandlerOption {
	return func(t *testing.T, pmh *PowerScaleHandler) {
		pmh.enforcer = v
	}
}

func (m *powerscaleHandlerOptionManager) withLogger(logger *logrus.Entry) powerscaleHandlerOption {
	return func(t *testing.T, pmh *PowerScaleHandler) {
		pmh.log = logger
	}
}

func buildPowerScaleHandler(t *testing.T, opts ...powerscaleHandlerOption) *PowerScaleHandler {
	m := &powerscaleHandlerOptionManager{}
	defaultOptions := []powerscaleHandlerOption{
		m.withLogger(testLogger()), // order matters for this one.
		m.withPowerScaleServer(func(w http.ResponseWriter, r *http.Request) {}),
		m.withOPAServer(func(w http.ResponseWriter, r *http.Request) {}),
	}

	ret := PowerScaleHandler{}

	for _, opt := range defaultOptions {
		opt(t, &ret)
	}
	for _, opt := range opts {
		opt(t, &ret)
	}

	return &ret
}

type powerscaleUtils struct{}

func (u *powerscaleUtils) testLogger() *logrus.Entry {
	logger := logrus.New().WithContext(context.Background())
	logger.Logger.SetOutput(os.Stdout)
	return logger
}

func (u *powerscaleUtils) fakeEnforcer() *quota.RedisEnforcement {
	return quota.NewRedisEnforcement(context.Background())
}

func (u *powerscaleUtils) hostPortFromFakeServer(t *testing.T, testServer *httptest.Server) string {
	parsedURL, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	return parsedURL.Host
}

func (u *powerscaleUtils) fakeServer(t *testing.T, h http.Handler) *httptest.Server {
	s := httptest.NewServer(h)
	t.Cleanup(func() {
		s.Close()
	})
	return s
}

func (u *powerscaleUtils) systemJSON(endpoint string) string {
	return fmt.Sprintf(`{
	  "powerscale": {
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

func (u *powerscaleUtils) systemObject(endpoint string) SystemConfig {
	return SystemConfig{
		"powerscale": Family{
			"1234567890": SystemEntry{
				Endpoint: endpoint,
				User:     "smc",
				Password: "smc",
				Insecure: true,
			},
		},
	}
}
