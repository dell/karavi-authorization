// Copyright © 2024 Dell Inc., or its subsidiaries. All Rights Reserved.
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

package main

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestProxyInstanceHandler(t *testing.T) {
	t.Run("it adds to the Forwarded header", func(t *testing.T) {
		fakeProxyServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer fakeProxyServer.Close()

		u, err := url.Parse(fakeProxyServer.URL)
		if err != nil {
			t.Fatal(err)
		}

		rp := httputil.NewSingleHostReverseProxy(u)
		rp.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}

		pi := &ProxyInstance{
			log:              logrus.NewEntry(logrus.New()),
			PluginID:         "powerflex",
			IntendedEndpoint: "https://powerflex.com",
			SystemID:         "542a2d5f5122210f",
			rp:               rp,
		}

		handler := pi.Handler(*u, "access", "refresh")

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)

		fwd := r.Header.Values("Forwarded")

		fwdFor := fwd[0]
		want := "for=csm-authorization;https://powerflex.com;542a2d5f5122210f"
		if fwdFor != want {
			t.Errorf("got %s, want %s", fwdFor, want)
		}

		fwdBy := fwd[1]
		want = "by=csm-authorization;powerflex"
		if fwdBy != want {
			t.Errorf("got %s, want %s", fwdFor, want)
		}
	})
}
