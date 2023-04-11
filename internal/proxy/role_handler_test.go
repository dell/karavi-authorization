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

package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"karavi-authorization/internal/role-service/mocks"
	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/pb"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func TestRoleHandler(t *testing.T) {
	t.Run("it handles role create", func(t *testing.T) {
		t.Run("successfully creates a role", func(t *testing.T) {
			client := &mocks.FakeRoleServiceClient{}

			sut := NewRoleHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&CreateRoleBody{
				Name:        "test",
				StorageType: "powerflex",
				SystemID:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "10",
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPost, "/proxy/roles/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusCreated {
				t.Errorf("expected status code %d, got %d", http.StatusCreated, code)
			}
		})
		t.Run("handles malformed request body", func(t *testing.T) {
			client := &mocks.FakeRoleServiceClient{}

			sut := NewRoleHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodPost, "/proxy/roles/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, code)
			}
		})
		t.Run("handles error from role service", func(t *testing.T) {
			client := &mocks.FakeRoleServiceClient{
				CreateRoleFn: func(ctx context.Context, ctr *pb.RoleCreateRequest, co ...grpc.CallOption) (*pb.RoleCreateResponse, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewRoleHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&CreateRoleBody{
				Name:        "test",
				StorageType: "powerflex",
				SystemID:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "10",
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPost, "/proxy/roles/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
	})
	t.Run("it handles role update", func(t *testing.T) {
		t.Run("successfully updates a role", func(t *testing.T) {
			client := &mocks.FakeRoleServiceClient{}

			sut := NewRoleHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&CreateRoleBody{
				Name:        "test",
				StorageType: "powerflex",
				SystemID:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "10",
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPatch, "/proxy/roles/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusNoContent {
				t.Errorf("expected status code %d, got %d", http.StatusNoContent, code)
			}
		})
		t.Run("handles malformed request body", func(t *testing.T) {
			client := &mocks.FakeRoleServiceClient{}

			sut := NewRoleHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodPatch, "/proxy/roles/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, code)
			}
		})
		t.Run("handles error from role service", func(t *testing.T) {
			client := &mocks.FakeRoleServiceClient{
				UpdateRoleFn: func(ctx context.Context, ctr *pb.RoleUpdateRequest, co ...grpc.CallOption) (*pb.RoleUpdateResponse, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewRoleHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&CreateRoleBody{
				Name:        "test",
				StorageType: "powerflex",
				SystemID:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "10",
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPatch, "/proxy/roles/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
	})
	t.Run("it handles role get", func(t *testing.T) {
		t.Run("successfully gets a role", func(t *testing.T) {
			client := &mocks.FakeRoleServiceClient{
				GetRoleFn: func(ctx context.Context, ctr *pb.RoleGetRequest, co ...grpc.CallOption) (*pb.RoleGetResponse, error) {
					return &pb.RoleGetResponse{
						Role: []byte("{\"test\":{\"system_types\":{\"powerflex\":{\"system_ids\":{\"542a2d5f5122210f\":{\"pool_quotas\":{\"bronze\":10}}}}}}}"),
					}, nil
				},
			}

			sut := NewRoleHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodGet, "/proxy/roles/", nil)
			w := httptest.NewRecorder()

			q := r.URL.Query()
			q.Add("name", "test")
			r.URL.RawQuery = q.Encode()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusOK {
				t.Errorf("expected status code %d, got %d", http.StatusOK, code)
			}

			type resp struct {
				Role []byte
			}
			want := resp{
				Role: []byte("{\"test\":{\"system_types\":{\"powerflex\":{\"system_ids\":{\"542a2d5f5122210f\":{\"pool_quotas\":{\"bronze\":10}}}}}}}"),
			}

			var got resp
			err := json.NewDecoder(w.Result().Body).Decode(&got)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(want, got) {
				t.Errorf("expectecd %v, got %v", want, got)
			}

		})
		t.Run("handles error from role service get", func(t *testing.T) {
			client := &mocks.FakeRoleServiceClient{
				GetRoleFn: func(ctx context.Context, ctr *pb.RoleGetRequest, co ...grpc.CallOption) (*pb.RoleGetResponse, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewRoleHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodGet, "/proxy/roles/", nil)
			w := httptest.NewRecorder()

			q := r.URL.Query()
			q.Add("name", "test")
			r.URL.RawQuery = q.Encode()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
		t.Run("successfully lists roles", func(t *testing.T) {
			fakeRoles := roles.NewJSON()
			fakeRoles.Add(&roles.Instance{
				Quota: 10,
				RoleKey: roles.RoleKey{
					Name:       "test",
					SystemType: "powerflex",
					SystemID:   "542a2d5f5122210f",
					Pool:       "bronze",
				},
			})

			b, err := fakeRoles.MarshalJSON()
			if err != nil {
				t.Fatal(err)
			}
			client := &mocks.FakeRoleServiceClient{
				ListRoleFn: func(ctx context.Context, ctr *pb.RoleListRequest, co ...grpc.CallOption) (*pb.RoleListResponse, error) {
					return &pb.RoleListResponse{
						Roles: []byte(b),
					}, nil
				},
			}

			sut := NewRoleHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodGet, "/proxy/roles/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusOK {
				t.Errorf("expected status code %d, got %d", http.StatusOK, code)
			}

			type resp struct {
				Roles []byte `json:"roles,omitempty"`
			}

			want := resp{
				Roles: []byte(b),
			}

			var got resp
			err = json.NewDecoder(w.Result().Body).Decode(&got)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(want, got) {
				t.Errorf("expectecd %v, got %v", want, got)
			}

		})
		t.Run("handles error from tenant service list", func(t *testing.T) {
			client := &mocks.FakeRoleServiceClient{
				ListRoleFn: func(ctx context.Context, ctr *pb.RoleListRequest, co ...grpc.CallOption) (*pb.RoleListResponse, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewRoleHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodGet, "/proxy/roles/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
	})
	t.Run("it handles role delete", func(t *testing.T) {
		t.Run("successfully deletes a Role", func(t *testing.T) {
			client := &mocks.FakeRoleServiceClient{}

			sut := NewRoleHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&CreateRoleBody{
				Name:        "test",
				StorageType: "powerflex",
				SystemID:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "10",
			})

			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodDelete, "/proxy/roles/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusNoContent {
				t.Errorf("expected status code %d, got %d", http.StatusNoContent, code)
			}
		})
		t.Run("handles bad query param", func(t *testing.T) {
			client := &mocks.FakeRoleServiceClient{}

			sut := NewRoleHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodDelete, "/proxy/roles/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, code)
			}
		})
		t.Run("handles error from Role service", func(t *testing.T) {
			client := &mocks.FakeRoleServiceClient{
				DeleteRoleFn: func(ctx context.Context, ctr *pb.RoleDeleteRequest, co ...grpc.CallOption) (*pb.RoleDeleteResponse, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewRoleHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&CreateRoleBody{
				Name:        "test",
				StorageType: "powerflex",
				SystemID:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "10",
			})

			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodDelete, "/proxy/roles/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
	})
}
