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
	"context"
	"encoding/json"
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

	"github.com/sirupsen/logrus"
)

func TestPowerScaleHandler(t *testing.T) {
	t.Run("UpdateSystems", testPowerScaleUpdateSystems)
	t.Run("ServeHTTP", testPowerScaleServeHTTP)
}

func testPowerScaleServeHTTP(t *testing.T) {
	t.Run("it proxies requests", func(t *testing.T) {
		done := make(chan struct{})
		u := &powerscaleUtils{}

		sessionCookie := "isisessid=12345678-abcd-1234-abcd-1234567890ab;"
		csrf := "isicsrf=c36a3484-4079-48d1-89a8-c1e2585ba867;"
		fakePowerScale := u.fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Logf("fake powerscale received: %s %s", r.Method, r.URL)
			if r.Method == http.MethodPost && r.URL.Path == "/session/1/session" {
				w.Header().Add("Set-Cookie", sessionCookie)
				w.Header().Add("Set-Cookie", csrf)
				w.WriteHeader(http.StatusCreated)
				return
			} else if r.Method == http.MethodGet && r.URL.Path == "/session/1/session" {
				w.Write([]byte(`{
					"services": [
						"namespace",
						"platform"
					],
					"timeout_absolute": 14372,
					"timeout_inactive": 900,
					"username": "admin"
				}
				`))
			} else if r.URL.Path == "/" {
				done <- struct{}{}
			} else {
				t.Fatalf("Unexpected request sent to fake Powerscale at %v", r.URL)
			}
		}))

		sut := buildPowerScaleHandler(t)

		err := sut.UpdateSystems(context.Background(), strings.NewReader(u.systemJSON(fakePowerScale.URL)), logrus.New().WithContext(context.Background()))
		if err != nil {
			t.Fatal(err)
		}

		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Forwarded", "for=https://1.1.1.1;1234567890")
		w := httptest.NewRecorder()

		go func() {
			sut.ServeHTTP(w, r)
			done <- struct{}{}
		}()
		<-done
		<-done // we also need to wait for the HTTP request to fully complete.

		if got, want := w.Result().StatusCode, http.StatusOK; got != want {
			t.Errorf("got status code %d, want status code %d", got, want)
		}
	})
	t.Run("it returns 502 Bad Gateway on unknown system", func(t *testing.T) {
		sut := buildPowerScaleHandler(t)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Forwarded", "for=https://1.1.1.1;0000000000") // pass unknown system ID
		w := httptest.NewRecorder()

		sut.ServeHTTP(w, r)

		want := http.StatusBadGateway
		if got := w.Result().StatusCode; got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	})
	t.Run("it uses session based authentication with the array", func(t *testing.T) {
		var (
			u = &powerscaleUtils{}
			m = &powerscaleHandlerOptionManager{}
		)
		var gotSessionCookie string
		wantedSessionCookie := "isisessid=12345678-abcd-1234-abcd-1234567890ab"
		sessionCookieHeader := "isisessid=12345678-abcd-1234-abcd-1234567890ab;"
		csrf := "isicsrf=c36a3484-4079-48d1-89a8-c1e2585ba867;"
		fakePowerScale := u.fakeServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Logf("fake powerscale received: %s %s", r.Method, r.URL)
			if r.Method == http.MethodPost && r.URL.Path == "/session/1/session" {
				w.Header().Add("Set-Cookie", sessionCookieHeader)
				w.Header().Add("Set-Cookie", csrf)
				w.WriteHeader(http.StatusCreated)
				return
			} else if r.Method == http.MethodGet && r.URL.Path == "/test/endpoint" {
				// check for proper headers
				gotSessionCookie = r.Header.Get("Cookie")
				w.WriteHeader(http.StatusOK)
				return
			} else if r.Method == http.MethodGet && r.URL.Path == "/session/1/session" {
				w.WriteHeader(http.StatusUnauthorized)
			} else {
				t.Fatalf("Unexpected request sent to fake Powerscale at %v", r.URL)
			}
		}))
		sut := buildPowerScaleHandler(t,
			m.withOPAServer(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{ "result": { "allow": true } }`)
			}))

		err := sut.UpdateSystems(context.Background(), strings.NewReader(u.systemJSON(fakePowerScale.URL)), logrus.New().WithContext(context.Background()))
		if err != nil {
			t.Fatal(err)
		}

		r := httptest.NewRequest(http.MethodGet,
			"/test/endpoint",
			nil)
		r.Header.Set("Forwarded", "for=https://1.1.1.1;1234567890")
		addJWTToRequestHeader(t, r)
		w := httptest.NewRecorder()

		web.Adapt(sut, web.AuthMW(discardLogger(), jwx.NewTokenManager(jwx.HS256))).ServeHTTP(w, r)

		if wantedSessionCookie != gotSessionCookie {
			t.Errorf("SessionCookie: got %q, want %q",
				gotSessionCookie, wantedSessionCookie)
		}
	})
}

func testPowerScaleUpdateSystems(t *testing.T) {
	u := &powerscaleUtils{}
	tests := []struct {
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

type powerscaleHandlerOption func(*testing.T, *PowerScaleHandler)

type powerscaleHandlerOptionManager struct{}

func (m *powerscaleHandlerOptionManager) withPowerScaleServer(h http.HandlerFunc) powerscaleHandlerOption {
	return func(t *testing.T, psh *PowerScaleHandler) {
		fakePowerScale := fakeServer(t, h)
		u := &powerscaleUtils{}
		err := psh.UpdateSystems(context.Background(), strings.NewReader(u.systemJSON(fakePowerScale.URL)), logrus.New().WithContext(context.Background()))
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

func TestErr(t *testing.T) {
	w := httptest.NewRecorder()
	writeErrorPowerScale(w, "test error", http.StatusUnauthorized, logrus.NewEntry(&logrus.Logger{}))

	errBody := struct {
		Err []APIErr `json:"errors"`
	}{}

	if err := json.NewDecoder(w.Body).Decode(&errBody); err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", errBody)
	if errBody.Err[0].Message == "" {
		t.Log("empty error")
	}
}
