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

package web

import (
	"net/http"
)

const (
	DebugPath             = "/debug/"
	PolicyPath            = "/policy/"
	ProxyRefreshTokenPath = "/proxy/refresh-token/"
	ProxyRolesPath        = "/proxy/roles/"
	ProxyPath             = "/"
)

type Router struct {
	PolicyHandler http.Handler
	TokenHandler  http.Handler
	RolesHandler  http.Handler
	ProxyHandler  http.Handler
}

func (rtr *Router) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle(PolicyPath, rtr.PolicyHandler)
	mux.Handle(ProxyRefreshTokenPath, rtr.TokenHandler)
	mux.Handle(ProxyRolesPath, rtr.RolesHandler)
	mux.Handle(ProxyPath, rtr.ProxyHandler)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r)
	})
}
