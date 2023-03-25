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
	"encoding/json"
	"karavi-authorization/internal/web"
	"net/http"
	"path"
	"sync"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// RootHandler is the entrypoint handler of the proxy server
type RootHandler struct {
	log     *logrus.Entry
	next    http.Handler
	once    sync.Once
	meter   metric.Meter
	key     attribute.KeyValue
	counter metric.Float64Counter
}

// Handler returns a new RootHandler
func Handler(log *logrus.Entry, next http.Handler) *RootHandler {
	return &RootHandler{
		log:  log,
		next: next,
	}
}

func (h *RootHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	r.URL.Path = cleanPath(r.URL.Path)
	h.log.Printf("Serving %s %s %v", r.RemoteAddr, r.Method, r.URL.Path)
	h.next.ServeHTTP(w, r)
}

func cleanPath(pth string) string {
	pth = path.Clean("/" + pth)
	if pth[len(pth)-1] != '/' {
		pth = pth + "/"
	}
	return pth
}

func writeError(w http.ResponseWriter, storage string, msg string, code int, log *logrus.Entry) {
	log.WithFields(logrus.Fields{
		"storage": storage,
		"code":    code,
		"message": msg,
	}).Debug("proxy: writing error")
	w.WriteHeader(code)
	errBody := struct {
		Code       int    `json:"errorCode"`
		StatusCode int    `json:"httpStatusCode"`
		Message    string `json:"message"`
	}{
		Code:       code,
		StatusCode: code,
		Message:    msg,
	}
	err := json.NewEncoder(w).Encode(&errBody)
	if err != nil {
		log.WithError(err).Error("encoding error response")
		http.Error(w, "Failed to encode error response", http.StatusInternalServerError)
	}
}

func jsonErrorResponse(log *logrus.Entry, w http.ResponseWriter, code int, err error) {
	log.Error(err)
	w.WriteHeader(code)
	if err := web.JSONErrorResponse(w, err); err != nil {
		log.WithError(err).Error("writing json error response")
	}
}
