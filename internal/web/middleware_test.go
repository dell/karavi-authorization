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

package web_test

import (
	"context"
	"errors"
	"karavi-authorization/internal/web"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestTelemetryMW(t *testing.T) {
	t.Run("it sets an error in the span", func(t *testing.T) {
		errHandler := func(w http.ResponseWriter, r *http.Request) error {
			return errors.New("test error")
		}

		exporter := tracetest.NewInMemoryExporter()
		h := web.Adapt(web.HandlerWithError(errHandler), web.TelemetryMW("", logrus.NewEntry(logrus.New())), web.OtelMW(sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter)), "test"))

		w := httptest.NewRecorder()
		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://10.0.0.0", nil)
		if err != nil {
			t.Fatal(err)
		}

		h.ServeHTTP(w, r)

		status := exporter.GetSpans()[0].Status

		if status.Code != codes.Error {
			t.Errorf("expected code %d, got %d", codes.Error, status.Code)
		}

		if status.Description != "test error" {
			t.Errorf("expected description test error, got %s", status.Description)
		}
	})

	t.Run("it executes the next handler if next is wrong type", func(t *testing.T) {
		var gotCalled bool
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotCalled = true
		})

		h := web.Adapt(handler, web.TelemetryMW("", logrus.NewEntry(logrus.New())))

		w := httptest.NewRecorder()
		r, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://10.0.0.0", nil)
		if err != nil {
			t.Fatal(err)
		}

		h.ServeHTTP(w, r)

		if !gotCalled {
			t.Errorf("expected next handler to be executed")
		}
	})
}
