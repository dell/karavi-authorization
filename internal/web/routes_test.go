// Copyright © 2021-2023 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"karavi-authorization/internal/web"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestRouter(t *testing.T) {
	noopHandler := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {})
	sut := &web.Router{}
	sut.TokenHandler = noopHandler
	sut.AdminTokenHandler = noopHandler
	sut.RolesHandler = noopHandler
	sut.ProxyHandler = noopHandler
	sut.VolumesHandler = noopHandler
	sut.TenantHandler = noopHandler
	sut.StorageHandler = noopHandler

	defer func() {
		if err := recover(); err != nil {
			t.Errorf("missing handler assignment: %+v", err)
		}
	}()

	sut.Handler() // will panic if not all expected routes are configured.

	t.Run("proxy handler is a catch-all handler", func(t *testing.T) {
		var (
			called bool
			wg     sync.WaitGroup
		)
		wg.Add(1)
		sut.ProxyHandler = http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
			defer wg.Done()
			called = true
		})

		w := httptest.NewRecorder()
		r, err := http.NewRequest(http.MethodGet, "/api/version/", nil)
		if err != nil {
			t.Fatal(err)
		}

		go func() {
			sut.Handler().ServeHTTP(w, r)
		}()
		wg.Wait()

		if !called {
			t.Error("expected the handler to be called, but it wasn't")
		}
	})
}
