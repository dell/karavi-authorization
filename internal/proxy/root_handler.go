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
	"path"
	"sync"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/label"
	"go.opentelemetry.io/otel/metric"
)

type RootHandler struct {
	log     *logrus.Entry
	next    http.Handler
	once    sync.Once
	meter   metric.Meter
	key     label.KeyValue
	counter metric.Float64Counter
}

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
	//	h.once.Do(func() {
	//		h.meter = otel.Meter("karavi/count")
	//		h.key = label.Key("path").String("/")
	//		h.counter = metric.Must(h.meter).NewFloat64Counter("hits")
	//	})
	//	h.counter.Add(ctx, 1, h.key)
	//	fmt.Fprintf(w, "hey")
}

func cleanPath(pth string) string {
	pth = path.Clean("/" + pth)
	if pth[len(pth)-1] != '/' {
		pth = pth + "/"
	}
	return pth
}
