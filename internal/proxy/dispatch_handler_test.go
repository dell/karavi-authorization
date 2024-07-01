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

package proxy_test

import (
	"context"
	"karavi-authorization/internal/proxy"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestNewDispatchHandler(t *testing.T) {
	ctx := context.Background()
	log := logrus.New().WithContext(ctx)

	h := proxy.NewDispatchHandler(log,
		buildSystemRegistry(t))

	if h == nil {
		t.Fatal("expected non-nil")
	}
}

func TestDispatchHandler_ServeHTTP(t *testing.T) {
	t.Run("empty dispatch handler returns 502", testEmptyDispatchHandler)
	t.Run("configured dispatch handler proxies request", testConfiguredDispatchHandler)
	t.Run("configured dispatch handler proxies request with various headers", testForwardedHeaders)
}

func testEmptyDispatchHandler(t *testing.T) {
	t.Log("Given a new dispatch handler with no systems registered")
	ctx := context.Background()
	log := logrus.New().WithContext(ctx)
	h := proxy.NewDispatchHandler(log,
		buildSystemRegistry(t))

	t.Log("When I make a request")
	w := httptest.NewRecorder()
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	checkError(t, err)
	// r.Header.Set("x-csi-plugin-identifier", "omitted-or-blank")
	h.ServeHTTP(w, r)

	t.Log("Then I should get back an 502 response")
	if got := w.Result().StatusCode; got != http.StatusBadGateway {
		t.Fatalf("(%s): got status %d, want %d", "/", got, http.StatusBadGateway)
	}
}

func testConfiguredDispatchHandler(t *testing.T) {
	t.Log("Given a dispatch handler with a powerflex system registered")
	ctx := context.Background()
	log := logrus.New().WithContext(ctx)
	h := proxy.NewDispatchHandler(log,
		map[string]http.Handler{
			"powerflex": http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			}),
		})

	t.Log("When I make a request")
	w := httptest.NewRecorder()
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	checkError(t, err)
	r.Header.Set("Forwarded", "by=csm-authorization;powerflex")
	h.ServeHTTP(w, r)

	t.Log("Then I should get back a 200 response")
	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Errorf("got status %d, want %d", got, http.StatusOK)
	}
}

func testForwardedHeaders(t *testing.T) {
	t.Log("Given a dispatch handler with a powerflex system registered")
	ctx := context.Background()
	log := logrus.New().WithContext(ctx)
	h := proxy.NewDispatchHandler(log,
		map[string]http.Handler{
			"powerflex": http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			}),
		})

	type requestBuilder func(t *testing.T) *http.Request
	requestBuilders := []requestBuilder{
		func(t *testing.T) *http.Request {
			r, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			checkError(t, err)
			r.Header.Add("Forwarded", "by=csm-authorization;powerflex,for=csm-authorization;https://1.1.1.1;7045c4cc20dffc0f")
			return r
		},
		func(t *testing.T) *http.Request {
			r, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
			checkError(t, err)
			r.Header.Add("Forwarded", "for=csm-authorization;https://1.1.1.1;7045c4cc20dffc0f")
			r.Header.Add("Forwarded", "by=csm-authorization;powerflex")
			return r
		},
	}

	for _, builder := range requestBuilders {
		r := builder(t)
		t.Log("When I make a request")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		t.Log("Then I should get back a 200 response")
		if got := w.Result().StatusCode; got != http.StatusOK {
			t.Errorf("got status %d, want %d", got, http.StatusOK)
		}
	}
}

func buildSystemRegistry(_ *testing.T) map[string]http.Handler {
	return map[string]http.Handler{}
}
