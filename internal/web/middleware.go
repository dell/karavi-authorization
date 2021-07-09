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
	"context"
	"karavi-authorization/internal/token"
	"net/http"
	"net/http/httputil"
	"path"
	"strings"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/trace"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// CtxKey wraps the int type and is meant for context values
type CtxKey int

// Common JWT values to be store inside the request context.
const (
	JWTKey        CtxKey = iota // JWTKey is the context key for the json web token
	JWTTenantName               // TenantName is the name of the Tenant.
	JWTRoles                    // Roles is the list of claimed roles.
	SystemIDKey                 // SystemIDKey is the context key for a system ID
)

var (
	// JWTSigningSecret is the secret string used to sign JWT tokens
	JWTSigningSecret = "secret"
)

// Middleware is a function that accepts an http Handler and returns an http Handler following the middleware pattern
type Middleware func(http.Handler) http.Handler

// Adapt applies the middlewares to the supplied http handler and returns said handler
func Adapt(h http.Handler, mws ...Middleware) http.Handler {
	for _, mw := range mws {
		h = mw(h)
	}
	return h
}

// OtelMW configures OpenTelemetry http instrumentation
func OtelMW(tp trace.TracerProvider, op string, opts ...otelhttp.Option) Middleware {
	return func(next http.Handler) http.Handler {
		switch t := tp.(type) {
		case *sdktrace.TracerProvider:
			if t == nil {
				return next
			}
		}
		opts = append(opts, otelhttp.WithTracerProvider(tp))
		return otelhttp.NewHandler(next, op, opts...)
	}
}

// LoggingMW configures logging incoming requests
func LoggingMW(log *logrus.Entry, showHTTPDump bool) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("Serving %s %s %v", r.RemoteAddr, r.Method, r.URL.Path)
			if showHTTPDump {
				b, err := httputil.DumpRequest(r, true)
				if err != nil {
					log.Printf("web: http dump request: %v", err)
					return
				}
				log.Println(string(b))
			}
			next.ServeHTTP(w, r)
		})
	}
}

// CleanMW configures formatting incoming request paths
func CleanMW() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = cleanPath(r.URL.Path)
			next.ServeHTTP(w, r)
		})
	}
}

func cleanPath(pth string) string {
	pth = path.Clean("/" + pth)
	if pth[len(pth)-1] != '/' {
		pth = pth + "/"
	}
	return pth
}

// AuthMW configures validating the json web token from the request
func AuthMW(log *logrus.Entry, tm token.Manager) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := r.Header.Get("Authorization")
			parts := strings.Split(authz, " ")
			if len(parts) != 2 {
				log.Println("invalid authz header")
				return
			}
			scheme, tkn := parts[0], parts[1]

			switch scheme {
			case "Bearer":
				var claims token.Claims
				parsedToken, err := tm.ParseWithClaims(tkn, JWTSigningSecret, &claims)
				if err != nil {
					w.WriteHeader(http.StatusUnauthorized)
					fwd := forwardedHeader(r)
					pluginID := normalizePluginID(fwd["by"])
					if pluginID == "powerscale" {
						if err := PowerScaleJSONErrorResponse(w, http.StatusUnauthorized, err); err != nil {
							log.WithError(err).Println("sending json response")
						}
						return
					}
					if err := JSONErrorResponse(w, err); err != nil {
						log.WithError(err).Println("sending json response")
					}
					return
				}

				ctx := context.WithValue(r.Context(), JWTKey, parsedToken)
				ctx = context.WithValue(ctx, JWTTenantName, claims.Group)
				ctx = context.WithValue(ctx, JWTRoles, claims.Roles)
				r = r.WithContext(ctx)
			case "Basic":
				log.Println("Basic authentication used")
			}

			next.ServeHTTP(w, r)
		})
	}
}

func forwardedHeader(r *http.Request) map[string]string {
	// Forwarded: for=foo by=bar -> map[for] = foo
	fwd := r.Header["Forwarded"]

	if len(fwd) > 0 {
		if strings.Contains(fwd[0], ",for") {
			fwd = strings.Split(fwd[0], ",")
		}
	}
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
			"powermax": {
				"powermax":     struct{}{},
				"csi-powermax": struct{}{},
			},
			"powerscale": {
				"powerscale":     struct{}{},
				"csi-powerscale": struct{}{},
				"isilon":         struct{}{},
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
