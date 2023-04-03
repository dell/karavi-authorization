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
	mocks "karavi-authorization/internal/storage-service/mocks"
	"karavi-authorization/pb"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func TestStorageHandler(t *testing.T) {
	t.Run("it handles storage create", func(t *testing.T) {
		t.Run("successfully creates a storage", func(t *testing.T) {
			client := &mocks.FakeStorageServiceClient{
				CreateStorageFn: func(ctx context.Context, ctr *pb.StorageCreateRequest, co ...grpc.CallOption) (*pb.StorageCreateResponse, error) {
					return &pb.StorageCreateResponse{}, nil
				},
			}

			sut := NewStorageHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&createStorageBody{
				StorageType: "powerflex",
				Endpoint:    "0.0.0.0:443",
				SystemID:    "542a2d5f5122210f",
				UserName:    "test",
				Password:    "test",
				Insecure:    true,
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPost, "/proxy/storage/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusCreated {
				t.Errorf("expected status code %d, got %d", http.StatusCreated, code)
			}
		})
		t.Run("handles malformed request body", func(t *testing.T) {
			client := &mocks.FakeStorageServiceClient{}

			sut := NewStorageHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodPost, "/proxy/storage/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, code)
			}
		})
		t.Run("handles error from storage service", func(t *testing.T) {
			client := &mocks.FakeStorageServiceClient{
				CreateStorageFn: func(ctx context.Context, ctr *pb.StorageCreateRequest, co ...grpc.CallOption) (*pb.StorageCreateResponse, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewStorageHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&createStorageBody{
				StorageType: "powerflex",
				Endpoint:    "0.0.0.0:443",
				SystemID:    "542a2d5f5122210f",
				UserName:    "test",
				Password:    "test",
				Insecure:    true,
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPost, "/proxy/storage/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
	})

	t.Run("it handles storage list", func(t *testing.T) {
		t.Run("successfully lists storages", func(t *testing.T) {
			client := &mocks.FakeStorageServiceClient{
				ListStorageFn: func(ctx context.Context, ctr *pb.StorageListRequest, co ...grpc.CallOption) (*pb.StorageListResponse, error) {

					return &pb.StorageListResponse{Storage: []byte("{\"powerflex\":{\"11e4e7d35817bd0f\":{\"User\":\"admin\",\"Password\":\"test\",\"Endpoint\":\"https://10.0.0.1\",\"Insecure\":false}}," +
						"\"powermax\":{\"542a2d5f5122210f\":{\"User\":\"admin\",\"Password\":\"test\",\"Endpoint\":\"https://10.0.0.1\",\"Insecure\":false}}}")}, nil
				},
			}

			sut := NewStorageHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodGet, "/proxy/storage/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusOK {
				t.Errorf("expected status code %d, got %d", http.StatusOK, code)
			}

			type resp struct {
				Storage []byte
			}
			want := resp{
				Storage: []byte("{\"powerflex\":{\"11e4e7d35817bd0f\":{\"User\":\"admin\",\"Password\":\"test\",\"Endpoint\":\"https://10.0.0.1\",\"Insecure\":false}}," +
					"\"powermax\":{\"542a2d5f5122210f\":{\"User\":\"admin\",\"Password\":\"test\",\"Endpoint\":\"https://10.0.0.1\",\"Insecure\":false}}}"),
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
		t.Run("handles error from storage service list", func(t *testing.T) {
			client := &mocks.FakeStorageServiceClient{
				ListStorageFn: func(ctx context.Context, ctr *pb.StorageListRequest, co ...grpc.CallOption) (*pb.StorageListResponse, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewStorageHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodGet, "/proxy/storage/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
	})

	t.Run("it handles storage update", func(t *testing.T) {
		t.Run("successfully updates a storage", func(t *testing.T) {
			client := &mocks.FakeStorageServiceClient{
				UpdateStorageFn: func(ctx context.Context, ctr *pb.StorageUpdateRequest, co ...grpc.CallOption) (*pb.StorageUpdateResponse, error) {
					return &pb.StorageUpdateResponse{}, nil
				},
			}

			sut := NewStorageHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&createStorageBody{
				StorageType: "powerflex",
				Endpoint:    "0.0.0.0:443",
				SystemID:    "542a2d5f5122210f",
				UserName:    "test",
				Password:    "test",
				Insecure:    true,
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPatch, "/proxy/storage/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusNoContent {
				t.Errorf("expected status code %d, got %d", http.StatusNoContent, code)
			}
		})
		t.Run("handles error from storage service", func(t *testing.T) {
			client := &mocks.FakeStorageServiceClient{
				UpdateStorageFn: func(ctx context.Context, ctr *pb.StorageUpdateRequest, co ...grpc.CallOption) (*pb.StorageUpdateResponse, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewStorageHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&createStorageBody{
				StorageType: "powerflex",
				Endpoint:    "0.0.0.0:443",
				SystemID:    "542a2d5f5122210f",
				UserName:    "test",
				Password:    "test",
				Insecure:    true,
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPatch, "/proxy/storage/", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
	})

	t.Run("it handles storage delete", func(t *testing.T) {
		t.Run("successfully deletes a storage", func(t *testing.T) {
			client := &mocks.FakeStorageServiceClient{
				DeleteStorageFn: func(ctx context.Context, ctr *pb.StorageDeleteRequest, co ...grpc.CallOption) (*pb.StorageDeleteResponse, error) {
					return &pb.StorageDeleteResponse{}, nil
				},
			}

			sut := NewStorageHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodDelete, "/proxy/storage/", nil)
			w := httptest.NewRecorder()

			q := r.URL.Query()
			q.Add("StorageType", "powerflex")
			q.Add("SystemId", "542a2d5f5122210f")
			r.URL.RawQuery = q.Encode()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusNoContent {
				t.Errorf("expected status code %d, got %d", http.StatusNoContent, code)
			}
		})
		t.Run("handles bad query param", func(t *testing.T) {
			client := &mocks.FakeStorageServiceClient{
				DeleteStorageFn: func(ctx context.Context, ctr *pb.StorageDeleteRequest, co ...grpc.CallOption) (*pb.StorageDeleteResponse, error) {
					return &pb.StorageDeleteResponse{}, nil
				},
			}

			sut := NewStorageHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodDelete, "/proxy/storage/", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, code)
			}
		})
		t.Run("handles error from storage service", func(t *testing.T) {
			client := &mocks.FakeStorageServiceClient{
				DeleteStorageFn: func(ctx context.Context, ctr *pb.StorageDeleteRequest, co ...grpc.CallOption) (*pb.StorageDeleteResponse, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewStorageHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodDelete, "/proxy/storage/", nil)
			w := httptest.NewRecorder()

			q := r.URL.Query()
			q.Add("StorageType", "powerflex")
			q.Add("SystemId", "542a2d5f5122210f")
			r.URL.RawQuery = q.Encode()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
	})
	t.Run("it handles storage get", func(t *testing.T) {
		t.Run("successfully gets a storage", func(t *testing.T) {
			client := &mocks.FakeStorageServiceClient{
				GetStorageFn: func(ctx context.Context, ctr *pb.StorageGetRequest, co ...grpc.CallOption) (*pb.StorageGetResponse, error) {
					return &pb.StorageGetResponse{Storage: []byte("{\"powerflex\":{\"542a2d5f5122210f\":{\"User\":\"admin\",\"Password\":\"test\",\"Endpoint\":\"https://10.0.0.1\",\"Insecure\":false}}}")}, nil
				},
			}

			sut := NewStorageHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodGet, "/proxy/storage/", nil)
			w := httptest.NewRecorder()

			q := r.URL.Query()
			q.Add("StorageType", "powerflex")
			q.Add("SystemId", "542a2d5f5122210f")
			r.URL.RawQuery = q.Encode()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusOK {
				t.Errorf("expected status code %d, got %d", http.StatusOK, code)
			}

			type storage struct {
				Storage []byte
			}

			want := storage{
				Storage: []byte("{\"powerflex\":{\"542a2d5f5122210f\":{\"User\":\"admin\",\"Password\":\"test\",\"Endpoint\":\"https://10.0.0.1\",\"Insecure\":false}}}"),
			}

			var got storage
			err := json.NewDecoder(w.Result().Body).Decode(&got)
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(want, got) {
				t.Errorf("expected %v, got %v", want, got)
			}

		})

		t.Run("handles error from storage service get", func(t *testing.T) {
			client := &mocks.FakeStorageServiceClient{
				GetStorageFn: func(ctx context.Context, ctr *pb.StorageGetRequest, co ...grpc.CallOption) (*pb.StorageGetResponse, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewStorageHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodGet, "/proxy/storage/", nil)
			w := httptest.NewRecorder()

			q := r.URL.Query()
			q.Add("StorageType", "powerflex")
			q.Add("SystemId", "542a2d5f5122210f")
			r.URL.RawQuery = q.Encode()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
	})
}
