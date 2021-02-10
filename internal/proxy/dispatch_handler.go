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
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

type DispatchHandler struct {
	log            *logrus.Entry
	systemHandlers map[string]http.Handler
}

func NewDispatchHandler(log *logrus.Entry, m map[string]http.Handler) *DispatchHandler {
	return &DispatchHandler{
		systemHandlers: m,
		log:            log,
	}
}

func (h *DispatchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fwd := forwardedHeader(r)
	pluginID := normalizePluginID(fwd["by"])
	next, ok := h.systemHandlers[pluginID]
	if !ok {
		http.Error(w, "plugin id not found", http.StatusBadGateway)
		return
	}
	next.ServeHTTP(w, r)
}

func forwardedHeader(r *http.Request) map[string]string {
	// Forwarded: for=foo by=bar -> map[for] = foo
	fwd := r.Header["Forwarded"]
	m := make(map[string]string, len(fwd))
	for _, e := range fwd {
		split := strings.Split(e, "=")
		m[split[0]] = split[1]
	}
	return m
}

func normalizePluginID(s string) string {
	l := []map[string]map[string]struct{}{
		{
			"powerflex": {
				"powerflex":    struct{}{},
				"csi-vxflexos": struct{}{},
				"vxflexos":     struct{}{},
			},
		},
	}

	for _, e := range l {
		for k, v := range e {
			if _, ok := v[s]; ok {
				return k
			}
		}
	}
	return ""
}
