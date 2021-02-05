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

func TestHandler(t *testing.T) {
	ctx := context.Background()
	log := logrus.New().WithContext(ctx)
	h := proxy.Handler(log,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	w := httptest.NewRecorder()
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	checkError(t, err)

	h.ServeHTTP(w, r)

	if got := w.Result().StatusCode; got != http.StatusOK {
		t.Errorf("got %d, want %d", got, http.StatusOK)
	}
}

func checkError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
