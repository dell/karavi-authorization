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
	"crypto/tls"
	"fmt"
	"karavi-authorization/internal/proxy"
	"karavi-authorization/internal/web"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
)

func init() {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
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
		fakePowerFlex := buildFakePowerFlex(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		}))
		// Add headers that the sidecar-proxy would add, in order to identify
		// the request as intended for a PowerFlex with the given systemID.
		r.Header.Add("Forwarded", "by=csi-vxflexos")
		r.Header.Add("Forwarded", fmt.Sprintf("for=https://%s;542a2d5f5122210f", fakePowerFlex.URL))
		// Create the router and assign the appropriate handlers.
		rtr := newTestRouter()
		// Create the PowerFlex handler and configure it with a system
		// where the endpoint is our test server.
		powerFlexHandler := proxy.NewPowerFlexHandler(log, nil, "")
		powerFlexHandler.UpdateSystems(strings.NewReader(fmt.Sprintf(`
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
`, fakePowerFlex.URL)))
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
		fakePowerFlex := buildFakePowerFlex(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Logf("Test request path is %q", r.URL.Path)
			if r.URL.Path == "/api/version/" {
				done <- struct{}{}
			}
		}))
		// Add headers that the sidecar-proxy would add, in order to identify
		// the request as intended for a PowerFlex with the given systemID.
		r.Header.Add("Forwarded", "by=csi-vxflexos")
		r.Header.Add("Forwarded", fmt.Sprintf("for=https://%s;542a2d5f5122210f", fakePowerFlex.URL))
		// Create the router and assign the appropriate handlers.
		rtr := newTestRouter()
		// Create the PowerFlex handler and configure it with a system
		// where the endpoint is our test server.
		powerFlexHandler := proxy.NewPowerFlexHandler(log, nil, "")
		powerFlexHandler.UpdateSystems(strings.NewReader(fmt.Sprintf(`
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
`, fakePowerFlex.URL)))
		systemHandlers := map[string]http.Handler{
			"powerflex": web.Adapt(powerFlexHandler),
		}
		dh := proxy.NewDispatchHandler(log, systemHandlers)
		rtr.ProxyHandler = dh
		h := web.Adapt(rtr.Handler(), web.CleanMW())

		go func() {
			h.ServeHTTP(w, r)
		}()
		<-done

		if got, want := w.Result().StatusCode, http.StatusOK; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
}

func newTestRouter() *web.Router {
	noopHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	return &web.Router{
		PolicyHandler: noopHandler,
		ProxyHandler:  noopHandler,
		RolesHandler:  noopHandler,
		TokenHandler:  noopHandler,
	}
}

func buildFakePowerFlex(t *testing.T, h ...http.Handler) *httptest.Server {
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
