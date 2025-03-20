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

package proxy

import (
	"fmt"
	"karavi-authorization/internal/web"
	"net/http"
	"strings"

	"github.com/sirupsen/logrus"
)

// DispatchHandler is a wrapper around various backend system http handlers
type DispatchHandler struct {
	log            *logrus.Entry
	systemHandlers map[string]http.Handler
}

// NewDispatchHandler returns a new DispatchHandler from the supplied map of pluginIDs to their respective http handler
func NewDispatchHandler(log *logrus.Entry, m map[string]http.Handler) *DispatchHandler {
	return &DispatchHandler{
		systemHandlers: m,
		log:            log,
	}
}

func (h *DispatchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fwd := web.ForwardedHeader(r)
	pluginID := web.NormalizePluginID(fwd["by"])
	next, ok := h.systemHandlers[pluginID]
	if !ok {
		http.Error(w, "plugin id not found", http.StatusBadGateway)
		return
	}
	next.ServeHTTP(w, r)
}

// SplitEndpointSystemID split the endpoint to read systemID
func SplitEndpointSystemID(s string) (string, string) {
	v := strings.Split(s, ";")
	if len(v) == 1 {
		return v[0], ""
	}
	return v[0], v[1]
}
