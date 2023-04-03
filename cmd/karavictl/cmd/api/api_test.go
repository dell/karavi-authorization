// Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

type body struct {
	Key string `json:"key"`
}

func TestAPI(t *testing.T) {
	svr := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/get":
			w.Write([]byte(fmt.Sprintf(`{"key": "%s"}`, r.URL.Query().Get("key"))))
		case "/post":
			var b body
			err := json.NewDecoder(r.Body).Decode(&b)
			if err != nil {
				t.Fatal(err)
			}
			w.Write([]byte(fmt.Sprintf(`{"key": "%s"}`, b.Key)))
		case "/patch":
			var b body
			err := json.NewDecoder(r.Body).Decode(&b)
			if err != nil {
				t.Fatal(err)
			}
			w.Write([]byte(fmt.Sprintf(`{"key": "%s"}`, b.Key)))
		case "/delete":
			w.Write([]byte(fmt.Sprintf(`{"key": "%s"}`, r.URL.Query().Get("key"))))
		default:
			t.Fatalf("%s not supported", r.URL.Path)
		}
	}))
	defer svr.Close()

	insecureClient, err := New(context.Background(), svr.URL, ClientOptions{Insecure: true})
	if err != nil {
		t.Fatal(err)
	}

	t.Run("GET", func(t *testing.T) {
		var resp body
		values := url.Values{
			"key": []string{"value"},
		}
		err = insecureClient.Get(context.Background(), "/get", nil, values, &resp)
		if err != nil {
			t.Fatal(err)
		}

		if resp.Key != "value" {
			t.Errorf("expected %s, got %s", "value", resp.Key)
		}
	})

	t.Run("POST", func(t *testing.T) {
		b := body{
			Key: "value",
		}
		var resp body

		err = insecureClient.Patch(context.Background(), "/post", nil, nil, &b, &resp)
		if err != nil {
			t.Fatal(err)
		}

		if resp.Key != "value" {
			t.Errorf("expected %s, got %s", "value", resp.Key)
		}
	})

	t.Run("PATCH", func(t *testing.T) {
		b := body{
			Key: "value",
		}
		var resp body

		err = insecureClient.Patch(context.Background(), "/patch", nil, nil, &b, &resp)
		if err != nil {
			t.Fatal(err)
		}

		if resp.Key != "value" {
			t.Errorf("expected %s, got %s", "value", resp.Key)
		}
	})

	t.Run("DELETE", func(t *testing.T) {
		var resp body
		values := url.Values{
			"key": []string{"value"},
		}
		err = insecureClient.Delete(context.Background(), "/delete", nil, values, &resp)
		if err != nil {
			t.Fatal(err)
		}

		if resp.Key != "value" {
			t.Errorf("expected %s, got %s", "value", resp.Key)
		}
	})
}
