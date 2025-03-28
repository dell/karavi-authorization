// Copyright © 2021-2024 Dell Inc., or its subsidiaries. All Rights Reserved.
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

package web_test

import (
	"context"
	"errors"
	"karavi-authorization/internal/token/jwx"
	"karavi-authorization/internal/web"
	"karavi-authorization/pb"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"gopkg.in/yaml.v2"
)

func TestTelemetryMW(t *testing.T) {
	t.Run("it sets an error in the span", func(t *testing.T) {
		errHandler := func(_ http.ResponseWriter, _ *http.Request) error {
			return errors.New("test error")
		}

		exporter := tracetest.NewInMemoryExporter()
		h := web.Adapt(web.HandlerWithError(errHandler), web.TelemetryMW("", logrus.NewEntry(logrus.New())), web.OtelMW(sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter)), "test"))

		w := httptest.NewRecorder()
		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://10.0.0.0", nil)
		if err != nil {
			t.Fatal(err)
		}

		h.ServeHTTP(w, r)

		status := exporter.GetSpans()[0].Status

		if status.Code != codes.Error {
			t.Errorf("expected code %d, got %d", codes.Error, status.Code)
		}

		if status.Description != "test error" {
			t.Errorf("expected description test error, got %s", status.Description)
		}
	})

	t.Run("it executes the next handler if next is wrong type", func(t *testing.T) {
		var gotCalled bool
		handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			gotCalled = true
		})

		h := web.Adapt(handler, web.TelemetryMW("", logrus.NewEntry(logrus.New())))

		w := httptest.NewRecorder()
		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://10.0.0.0", nil)
		if err != nil {
			t.Fatal(err)
		}

		h.ServeHTTP(w, r)

		if !gotCalled {
			t.Errorf("expected next handler to be executed")
		}
	})
}

func TestAuthMW(t *testing.T) {
	t.Run("it validates a token", func(t *testing.T) {
		handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})
		h := web.Adapt(handler, web.AuthMW(discardLogger(), jwx.NewTokenManager(jwx.HS256)))

		tkn, err := jwx.GenerateAdminToken(context.Background(), &pb.GenerateAdminTokenRequest{
			AdminName:        "admin",
			JWTSigningSecret: "secret",
		})
		checkError(t, err)
		if len(tkn.Token) == 0 {
			t.Errorf("got %q, want non-empty", tkn.Token)
		}

		tknData := tkn.Token
		var tokenData struct {
			Access string `yaml:"Access"`
		}

		err = yaml.Unmarshal([]byte(tknData), &tokenData)
		checkError(t, err)

		w := httptest.NewRecorder()
		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		checkError(t, err)

		r.Header.Add("Authorization", "Bearer "+string(tokenData.Access))

		h.ServeHTTP(w, r)
		if status := w.Code; status != http.StatusOK {
			t.Errorf("got %v, want %v", status, http.StatusOK)
		}
	})

	t.Run("it writes an error with an invalid token", func(t *testing.T) {
		handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})
		h := web.Adapt(handler, web.AuthMW(discardLogger(), jwx.NewTokenManager(jwx.HS256)))

		// test token
		tokens := make(map[string]interface{})
		credFile, err := os.ReadFile("../../tokens.yaml")
		if err != nil {
			t.Errorf("unable to read token: %v", err)
		}
		err = yaml.Unmarshal(credFile, &tokens)
		if err != nil {
			t.Errorf("unable to unmarshal token: %v", err)
		}
		tokenString := tokens["tokenString"].(string)

		w := httptest.NewRecorder()
		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		checkError(t, err)

		r.Header.Set("Authorization", tokenString)
		h.ServeHTTP(w, r)
		if status := w.Code; status != http.StatusUnauthorized {
			t.Errorf("got %v, want %v", status, http.StatusUnauthorized)
		}
	})

	t.Run("it writes an error with an invalid token to csi-powerscale", func(t *testing.T) {
		handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})
		h := web.Adapt(handler, web.AuthMW(discardLogger(), jwx.NewTokenManager(jwx.HS256)))

		// test token
		tokens := make(map[string]interface{})
		credFile, err := os.ReadFile("../../tokens.yaml")
		if err != nil {
			t.Errorf("unable to read token: %v", err)
		}
		err = yaml.Unmarshal(credFile, &tokens)
		if err != nil {
			t.Errorf("unable to unmarshal token: %v", err)
		}
		tokenString := tokens["tokenString"].(string)

		w := httptest.NewRecorder()
		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		checkError(t, err)

		r.Header = http.Header{
			"Forwarded": []string{"by=powerscale"},
		}

		r.Header.Set("Authorization", tokenString)
		h.ServeHTTP(w, r)
		if status := w.Code; status != http.StatusUnauthorized {
			t.Errorf("got %v, want %v", status, http.StatusUnauthorized)
		}
	})

	t.Run("it executes the next handler if next is wrong type", func(t *testing.T) {
		var gotCalled bool
		handler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			gotCalled = true
		})
		h := web.Adapt(handler, web.AuthMW(discardLogger(), jwx.NewTokenManager(jwx.HS256)))

		tkn, err := jwx.GenerateAdminToken(context.Background(), &pb.GenerateAdminTokenRequest{
			AdminName:        "admin",
			JWTSigningSecret: "secret",
		})
		checkError(t, err)
		if len(tkn.Token) == 0 {
			t.Errorf("got %q, want non-empty", tkn.Token)
		}

		tknData := tkn.Token
		var tokenData struct {
			Access string `yaml:"Access"`
		}

		err = yaml.Unmarshal([]byte(tknData), &tokenData)
		checkError(t, err)

		w := httptest.NewRecorder()
		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
		checkError(t, err)

		r.Header.Add("Authorization", "Bearer "+string(tokenData.Access))

		h.ServeHTTP(w, r)
		if status := w.Code; status != http.StatusOK {
			t.Errorf("got %v, want %v", status, http.StatusOK)
		}

		if gotCalled == false {
			t.Errorf("expected next handler to be executed")
		}
	})
}

func TestFowardedHeader(t *testing.T) {
	tests := []struct {
		name    string
		request *http.Request
		want    map[string]string
	}{
		{
			name: "it parses the csm-authorization values",
			request: &http.Request{
				Header: http.Header{
					"Forwarded": []string{"for=csm-authorization;https://10.0.0.1;12345", "by=csm-authorization;powerflex"},
				},
			},
			want: map[string]string{
				"for": "https://10.0.0.1;12345",
				"by":  "powerflex",
			},
		},
		{
			name: "it parses the csm-authorization values with another for value",
			request: &http.Request{
				Header: http.Header{
					"Forwarded": []string{"for=10.0.0.1;host=ingress.com", "for=csm-authorization;https://10.0.0.1;12345", "by=csm-authorization;powerflex"},
				},
			},
			want: map[string]string{
				"for": "https://10.0.0.1;12345",
				"by":  "powerflex",
			},
		},
		{
			name: "it parses without csm-authorization values",
			request: &http.Request{
				Header: http.Header{
					"Forwarded": []string{"for=https://10.0.0.1;12345", "by=powerflex"},
				},
			},
			want: map[string]string{
				"for": "https://10.0.0.1;12345",
				"by":  "powerflex",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := web.ForwardedHeader(test.request)
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func discardLogger() *logrus.Entry {
	logger := logrus.New()
	return logger.WithContext(context.Background())
}

func checkError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
