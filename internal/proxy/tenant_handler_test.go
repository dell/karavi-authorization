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
	"karavi-authorization/internal/tenantsvc/mocks"
	"karavi-authorization/pb"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func TestTenantHandler(t *testing.T) {
	t.Run("it handles tenant create", func(t *testing.T) {
		t.Run("successfully creates a tenant", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				CreateTenantFn: func(_ context.Context, _ *pb.CreateTenantRequest, _ ...grpc.CallOption) (*pb.Tenant, error) {
					return &pb.Tenant{
						Name:       "test",
						Approvesdc: true,
					}, nil
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&CreateTenantBody{
				Tenant:     "test",
				ApproveSdc: true,
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPost, "/proxy/tenant/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusCreated {
				t.Errorf("expected status code %d, got %d", http.StatusCreated, code)
			}
		})
		t.Run("handles malformed request body", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodPost, "/proxy/tenant/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, code)
			}
		})
		t.Run("handles error from tenant service", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				CreateTenantFn: func(_ context.Context, _ *pb.CreateTenantRequest, _ ...grpc.CallOption) (*pb.Tenant, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&CreateTenantBody{
				Tenant:     "test",
				ApproveSdc: true,
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPost, "/proxy/tenant/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
	})
	t.Run("it handles tenant update", func(t *testing.T) {
		t.Run("successfully updates a tenant", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				UpdateTenantFn: func(_ context.Context, _ *pb.UpdateTenantRequest, _ ...grpc.CallOption) (*pb.Tenant, error) {
					return &pb.Tenant{
						Name:       "test",
						Approvesdc: true,
					}, nil
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&CreateTenantBody{
				Tenant:     "test",
				ApproveSdc: true,
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPatch, "/proxy/tenant/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusNoContent {
				t.Errorf("expected status code %d, got %d", http.StatusNoContent, code)
			}
		})
		t.Run("handles malformed request body", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodPatch, "/proxy/tenant/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, code)
			}
		})
		t.Run("handles error from tenant service", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				UpdateTenantFn: func(_ context.Context, _ *pb.UpdateTenantRequest, _ ...grpc.CallOption) (*pb.Tenant, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&CreateTenantBody{
				Tenant:     "test",
				ApproveSdc: true,
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPatch, "/proxy/tenant/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
	})
	t.Run("it handles tenant get", func(t *testing.T) {
		t.Run("successfully gets a tenant", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				GetTenantFn: func(_ context.Context, _ *pb.GetTenantRequest, _ ...grpc.CallOption) (*pb.Tenant, error) {
					return &pb.Tenant{
						Name:       "test",
						Roles:      "test",
						Approvesdc: false,
					}, nil
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodGet, "/proxy/tenant/", nil)
			w := httptest.NewRecorder()

			q := r.URL.Query()
			q.Add("name", "test")
			r.URL.RawQuery = q.Encode()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusOK {
				t.Errorf("expected status code %d, got %d", http.StatusOK, code)
			}

			type tenant struct {
				Name       string `json:"name"`
				Roles      string `json:"roles"`
				ApproveSdc bool   `json:"approvesdc"`
			}
			want := tenant{
				Name:       "test",
				Roles:      "test",
				ApproveSdc: false,
			}

			var got tenant
			err := json.NewDecoder(w.Result().Body).Decode(&got)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(want, got) {
				t.Errorf("expectecd %v, got %v", want, got)
			}
		})
		t.Run("handles error from tenant service get", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				GetTenantFn: func(_ context.Context, _ *pb.GetTenantRequest, _ ...grpc.CallOption) (*pb.Tenant, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodGet, "/proxy/tenant/", nil)
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
		t.Run("successfully lists tenants", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				ListTenantFn: func(_ context.Context, _ *pb.ListTenantRequest, _ ...grpc.CallOption) (*pb.ListTenantResponse, error) {
					return &pb.ListTenantResponse{
						Tenants: []*pb.Tenant{
							{
								Name: "test",
							},
							{
								Name: "test2",
							},
						},
					}, nil
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodGet, "/proxy/tenant/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusOK {
				t.Errorf("expected status code %d, got %d", http.StatusOK, code)
			}

			type tenant struct {
				Name string `json:"name"`
			}

			type resp struct {
				Tenants []tenant `json:"tenants"`
			}

			want := resp{
				Tenants: []tenant{
					{
						Name: "test",
					},
					{
						Name: "test2",
					},
				},
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
		t.Run("handles error from tenant service list", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				ListTenantFn: func(_ context.Context, _ *pb.ListTenantRequest, _ ...grpc.CallOption) (*pb.ListTenantResponse, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodGet, "/proxy/tenant/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
	})
	t.Run("it handles tenant delete", func(t *testing.T) {
		t.Run("successfully deletes a tenant", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				DeleteTenantFn: func(_ context.Context, _ *pb.DeleteTenantRequest, _ ...grpc.CallOption) (*pb.DeleteTenantResponse, error) {
					return &pb.DeleteTenantResponse{}, nil
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodDelete, "/proxy/tenant/", nil)
			w := httptest.NewRecorder()

			q := r.URL.Query()
			q.Add("name", "test")
			r.URL.RawQuery = q.Encode()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusNoContent {
				t.Errorf("expected status code %d, got %d", http.StatusNoContent, code)
			}
		})
		t.Run("handles bad query param", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				DeleteTenantFn: func(_ context.Context, _ *pb.DeleteTenantRequest, _ ...grpc.CallOption) (*pb.DeleteTenantResponse, error) {
					return &pb.DeleteTenantResponse{}, nil
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodDelete, "/proxy/tenant/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, code)
			}
		})
		t.Run("handles error from tenant service", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				DeleteTenantFn: func(_ context.Context, _ *pb.DeleteTenantRequest, _ ...grpc.CallOption) (*pb.DeleteTenantResponse, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodDelete, "/proxy/tenant/", nil)
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
	})
	t.Run("it handles bind role", func(t *testing.T) {
		t.Run("successfully binds a role", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				BindRoleFn: func(_ context.Context, _ *pb.BindRoleRequest, _ ...grpc.CallOption) (*pb.BindRoleResponse, error) {
					return &pb.BindRoleResponse{}, nil
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&BindRoleBody{
				Tenant: "test",
				Role:   "test",
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPost, "/proxy/tenant/bind/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusCreated {
				t.Errorf("expected status code %d, got %d", http.StatusCreated, code)
			}
		})
		t.Run("handles bad request", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				BindRoleFn: func(_ context.Context, _ *pb.BindRoleRequest, _ ...grpc.CallOption) (*pb.BindRoleResponse, error) {
					return &pb.BindRoleResponse{}, nil
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodGet, "/proxy/tenant/bind/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusMethodNotAllowed {
				t.Errorf("expected status code %d, got %d", http.StatusMethodNotAllowed, code)
			}
		})
		t.Run("handles malformed request body", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodPost, "/proxy/tenant/bind/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, code)
			}
		})
		t.Run("handles error from tenant service", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				BindRoleFn: func(_ context.Context, _ *pb.BindRoleRequest, _ ...grpc.CallOption) (*pb.BindRoleResponse, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&BindRoleBody{
				Tenant: "test",
				Role:   "test",
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPost, "/proxy/tenant/bind/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
	})
	t.Run("it handles unbind role", func(t *testing.T) {
		t.Run("successfully unbinds a role", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				UnbindRoleFn: func(_ context.Context, _ *pb.UnbindRoleRequest, _ ...grpc.CallOption) (*pb.UnbindRoleResponse, error) {
					return &pb.UnbindRoleResponse{}, nil
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&BindRoleBody{
				Tenant: "test",
				Role:   "test",
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPost, "/proxy/tenant/unbind/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusNoContent {
				t.Errorf("expected status code %d, got %d", http.StatusNoContent, code)
			}
		})
		t.Run("handles bad request", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				BindRoleFn: func(_ context.Context, _ *pb.BindRoleRequest, _ ...grpc.CallOption) (*pb.BindRoleResponse, error) {
					return &pb.BindRoleResponse{}, nil
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodGet, "/proxy/tenant/unbind/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusMethodNotAllowed {
				t.Errorf("expected status code %d, got %d", http.StatusMethodNotAllowed, code)
			}
		})
		t.Run("handles malformed request body", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodPost, "/proxy/tenant/unbind/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, code)
			}
		})
		t.Run("handles error from tenant service", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				UnbindRoleFn: func(_ context.Context, _ *pb.UnbindRoleRequest, _ ...grpc.CallOption) (*pb.UnbindRoleResponse, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&BindRoleBody{
				Tenant: "test",
				Role:   "test",
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPost, "/proxy/tenant/unbind/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
	})
	t.Run("it handles generate token", func(t *testing.T) {
		t.Run("successfully generates a token", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				GenerateTokenFn: func(_ context.Context, _ *pb.GenerateTokenRequest, _ ...grpc.CallOption) (*pb.GenerateTokenResponse, error) {
					return &pb.GenerateTokenResponse{
						Token: "token",
					}, nil
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&GenerateTokenBody{
				Tenant:          "Test",
				AccessTokenTTL:  "30s",
				RefreshTokenTTL: "1m",
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPost, "/proxy/tenant/token/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusOK {
				t.Errorf("expected status code %d, got %d", http.StatusOK, code)
			}
		})
		t.Run("handles bad request", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				GenerateTokenFn: func(_ context.Context, _ *pb.GenerateTokenRequest, _ ...grpc.CallOption) (*pb.GenerateTokenResponse, error) {
					return &pb.GenerateTokenResponse{
						Token: "token",
					}, nil
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodGet, "/proxy/tenant/token/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusMethodNotAllowed {
				t.Errorf("expected status code %d, got %d", http.StatusMethodNotAllowed, code)
			}
		})
		t.Run("handles malformed request body", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodPost, "/proxy/tenant/token/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, code)
			}
		})
		t.Run("handles error from tenant service", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				GenerateTokenFn: func(_ context.Context, _ *pb.GenerateTokenRequest, _ ...grpc.CallOption) (*pb.GenerateTokenResponse, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&GenerateTokenBody{
				Tenant:          "test",
				AccessTokenTTL:  "30s",
				RefreshTokenTTL: "1m",
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPost, "/proxy/tenant/token/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
	})
	t.Run("it handles tenant revoke", func(t *testing.T) {
		t.Run("successfully revokes a tenant", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				RevokeTenantFn: func(_ context.Context, _ *pb.RevokeTenantRequest, _ ...grpc.CallOption) (*pb.RevokeTenantResponse, error) {
					return &pb.RevokeTenantResponse{}, nil
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&TenantRevokeBody{
				Tenant: "test",
				Cancel: false,
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPatch, "/proxy/tenant/revoke/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusNoContent {
				t.Errorf("expected status code %d, got %d", http.StatusNoContent, code)
			}
		})
		t.Run("successfully cancells tenant revocation", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				CancelRevokeTenantFn: func(_ context.Context, _ *pb.CancelRevokeTenantRequest, _ ...grpc.CallOption) (*pb.CancelRevokeTenantResponse, error) {
					return &pb.CancelRevokeTenantResponse{}, nil
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&TenantRevokeBody{
				Tenant: "test",
				Cancel: true,
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPatch, "/proxy/tenant/revoke/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusNoContent {
				t.Errorf("expected status code %d, got %d", http.StatusNoContent, code)
			}
		})
		t.Run("handles bad request", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodGet, "/proxy/tenant/revoke/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusMethodNotAllowed {
				t.Errorf("expected status code %d, got %d", http.StatusMethodNotAllowed, code)
			}
		})
		t.Run("handles malformed request body", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodPatch, "/proxy/tenant/revoke/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, code)
			}
		})
		t.Run("handles error from revoking tenant", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				RevokeTenantFn: func(_ context.Context, _ *pb.RevokeTenantRequest, _ ...grpc.CallOption) (*pb.RevokeTenantResponse, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&TenantRevokeBody{
				Tenant: "test",
				Cancel: false,
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPatch, "/proxy/tenant/revoke/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
		t.Run("handles error from cancelling tenant revokation", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				CancelRevokeTenantFn: func(_ context.Context, _ *pb.CancelRevokeTenantRequest, _ ...grpc.CallOption) (*pb.CancelRevokeTenantResponse, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&TenantRevokeBody{
				Tenant: "test",
				Cancel: true,
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPatch, "/proxy/tenant/revoke/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
	})
}
