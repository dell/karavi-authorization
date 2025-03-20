// Copyright © 2021-2024 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"fmt"
	"karavi-authorization/internal/token"
	"net/http"
	"net/http/httputil"
	"path"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// CtxKey wraps the int type and is meant for context values
type CtxKey int

// Common JWT values to be store inside the request context.
const (
	JWTKey        CtxKey = iota // JWTKey is the context key for the json web token
	JWTTenantName               // TenantName is the name of the Tenant.
	JWTAdminName                // AdminName is the name of the admin.
	JWTRoles                    // Roles is the list of claimed roles.
	SystemIDKey                 // SystemIDKey is the context key for a system ID
)

// JWTSigningSecret is the secret string used to sign JWT tokens
var JWTSigningSecret = "secret"

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

// AuthMW configures validating the admin or the tenant json web token from the request
func AuthMW(log *logrus.Entry, tm token.Manager) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// let tenant refresh token go through
			if r.URL.Path == "/proxy/refresh-token/" {
				next.ServeHTTP(w, r)
				return
			}

			log.Info("Validating token!")
			authz := r.Header.Get("Authorization")
			parts := strings.Split(authz, " ")
			if len(parts) != 2 {
				if err := JSONErrorResponse(w, http.StatusUnauthorized, fmt.Errorf("invalid authz header")); err != nil {
					log.WithError(err).Println("error creating json response")
				}
				log.Errorf("invalid authz header: %v", parts)
				return
			}

			scheme, tkn := parts[0], parts[1]

			switch scheme {
			case "Bearer":
				var claims token.Claims
				parsedToken, err := tm.ParseWithClaims(tkn, JWTSigningSecret, &claims)
				if err != nil {
					log.Debugf("validating token: %v", err)

					fwd := ForwardedHeader(r)
					pluginID := NormalizePluginID(fwd["by"])

					if pluginID == "powerscale" {
						// if the pluginID is powerscale, we must write an error response specific for csi-powerscale
						// otherwise, we can write the standard JSONErrorResponse to the driver or karavictl/dellctl
						if err := PowerScaleJSONErrorResponse(w, http.StatusUnauthorized, err); err != nil {
							log.WithError(err).Println("sending json response")
						}
						return
					}

					if err := JSONErrorResponse(w, http.StatusUnauthorized, err); err != nil {
						log.WithError(err).Println("sending json response")
					}
					return
				}

				if claims.Subject == "csm-admin" {
					ctx := context.WithValue(r.Context(), JWTKey, parsedToken)
					ctx = context.WithValue(ctx, JWTAdminName, claims.Group)
					r = r.WithContext(ctx)
				} else {
					ctx := context.WithValue(r.Context(), JWTKey, parsedToken)
					ctx = context.WithValue(ctx, JWTTenantName, claims.Group)
					ctx = context.WithValue(ctx, JWTRoles, claims.Roles)
					r = r.WithContext(ctx)
				}
			case "Basic":
				log.Println("Basic authentication used")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// HandlerWithError is a http HandlerFunc that returns an error
type HandlerWithError func(w http.ResponseWriter, r *http.Request) error

// ServeHTTP implements the http.Handler interface
// This is a noop because the underlying HandlerWithError should be executed explicitly
func (h HandlerWithError) ServeHTTP(_ http.ResponseWriter, _ *http.Request) {}

// TelemetryMW logs the time for the next handler and records the error from the next handler in the span
// The next handler must be the HandlerWithError type for logging and error recording
func TelemetryMW(name string, log *logrus.Entry) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h, ok := next.(HandlerWithError)
			if !ok {
				next.ServeHTTP(w, r)
				return
			}

			now := time.Now()
			defer timeSince(now, name, log)

			span := trace.SpanFromContext(r.Context())
			err := h(w, r)
			if err != nil {
				span.SetStatus(codes.Error, err.Error())
				span.RecordError(err)
			}
		})
	}
}

func timeSince(start time.Time, fName string, log *logrus.Entry) {
	log.WithFields(logrus.Fields{
		"function": fName,
		"duration": fmt.Sprintf("%v", time.Since(start)),
	}).Debug()
}

// ForwardedHeader splits forward headers for verification
func ForwardedHeader(r *http.Request) map[string]string {
	// Forwarded header can either be
	// Forwarded: for=https://10.0.0.1;12345 by=powerflex
	// Or
	// Forwarded: for=10.0.0.1;host=ingress.com for=csm-authorization;https://10.0.0.1;12345 by=csm-authorization;powerflex
	// -> map[for] = https://10.0.0.1;12345; map[by] = powerflex
	fwd := r.Header["Forwarded"]

	m := make(map[string]string)
	for _, e := range fwd {
		if strings.Contains(e, "csm-authorization;") {
			split := strings.Split(strings.ReplaceAll(e, "csm-authorization;", ""), "=")
			if len(split) >= 2 {
				m[split[0]] = split[1]
			}
		} else {
			split := strings.Split(e, "=")
			if len(split) >= 2 {
				m[split[0]] = split[1]
			}
		}
	}
	return m
}

// NormalizePluginID returns an array identifier to the forwarded header
func NormalizePluginID(s string) string {
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
