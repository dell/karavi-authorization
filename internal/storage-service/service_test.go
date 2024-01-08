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

package storage_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	storage "karavi-authorization/cmd/karavictl/cmd"
	service "karavi-authorization/internal/storage-service"
	"karavi-authorization/pb"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestServiceCreate(t *testing.T) {
	// define check functions to pass or fail tests
	type checkFn func(*testing.T, error)

	// define test input
	tests := map[string]func(t *testing.T) (*pb.StorageCreateRequest, service.Validator, service.Kube, checkFn){
		"success": func(t *testing.T) (*pb.StorageCreateRequest, service.Validator, service.Kube, checkFn) {
			r := &pb.StorageCreateRequest{
				StorageType: "powerflex",
				Endpoint:    "0.0.0.0:443",
				SystemId:    "542a2d5f5122210f",
				UserName:    "test",
				Password:    "test",
				Insecure:    true,
			}
			return r, successfulValidator{}, successfulKube{}, errIsNil
		},
		"fail validation": func(t *testing.T) (*pb.StorageCreateRequest, service.Validator, service.Kube, checkFn) {
			r := &pb.StorageCreateRequest{
				StorageType: "invalid-storage-type",
				Endpoint:    "0.0.0.0:443",
				SystemId:    "542a2d5f5122210f",
				UserName:    "test",
				Password:    "test",
				Insecure:    true,
			}
			return r, failValidator{}, successfulKube{}, errIsNotNil
		},
		"fail kube and validation": func(t *testing.T) (*pb.StorageCreateRequest, service.Validator, service.Kube, checkFn) {
			r := &pb.StorageCreateRequest{
				StorageType: "powerflex",
				Endpoint:    "0.0.0.0:443",
				SystemId:    "542a2d5f5122210f",
				UserName:    "test",
				Password:    "test",
				Insecure:    true,
			}
			return r, failValidator{}, failKube{}, errIsNotNil
		},
		"fail kube": func(t *testing.T) (*pb.StorageCreateRequest, service.Validator, service.Kube, checkFn) {
			r := &pb.StorageCreateRequest{
				StorageType: "powerflex",
				Endpoint:    "0.0.0.0:443",
				SystemId:    "542a2d5f5122210f",
				UserName:    "test",
				Password:    "test",
				Insecure:    true,
			}
			return r, successfulValidator{}, failKube{}, errIsNotNil
		},
	}

	// run the tests
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req, validator, kube, checkFn := tc(t)
			svc := service.NewService(kube, validator)
			_, err := svc.Create(context.Background(), req)
			checkFn(t, err)
		})
	}
}

func TestServiceList(t *testing.T) {
	// define check functions to pass or fail tests
	type checkFn func(t *testing.T, err error, got *pb.StorageListResponse)

	checkExpected := func(t *testing.T, want string) func(t *testing.T, err error, got *pb.StorageListResponse) {
		return func(t *testing.T, err error, got *pb.StorageListResponse) {
			if err != nil {
				t.Errorf("want nil error, got %v", err)
			}

			if want != string(got.Storage) {
				t.Errorf("want %s, got %s", want, string(got.Storage))
			}
		}
	}

	errIsNotNil := func(t *testing.T, want string) func(t *testing.T, err error, got *pb.StorageListResponse) {
		return func(t *testing.T, err error, got *pb.StorageListResponse) {
			if err == nil {
				t.Errorf("expected non-nil err")
			}
		}
	}

	// define test input
	tests := map[string]func(t *testing.T) (service.Kube, checkFn){
		"success": func(t *testing.T) (service.Kube, checkFn) {
			getStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return storage.Storage{
					"powerflex": storage.SystemType{
						"11e4e7d35817bd0f": storage.System{
							User:     "admin",
							Password: "test",
							Endpoint: "https://10.0.0.1",
							Insecure: false,
						},
					},
				}, nil
			}

			want := `{"powerflex":{"11e4e7d35817bd0f":{"User":"admin","Password":"test","Endpoint":"https://10.0.0.1","Insecure":false}}}`
			return fakeKube{GetConfiguredStorageFn: getStorageFn}, checkExpected(t, want)
		},
		"error getting configured storage": func(t *testing.T) (service.Kube, checkFn) {
			getStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return nil, errors.New("error")
			}
			return fakeKube{GetConfiguredStorageFn: getStorageFn}, errIsNotNil(t, "")
		},
	}

	// run the tests
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			kube, checkFn := tc(t)
			svc := service.NewService(kube, nil)
			resp, err := svc.List(context.Background(), &pb.StorageListRequest{})
			checkFn(t, err, resp)
		})
	}
}

func TestServiceUpdate(t *testing.T) {
	// define check functions to pass or fail tests
	type checkFn func(t *testing.T, err error, kube fakeKube)

	checkExpected := func(t *testing.T, want storage.Storage) func(t *testing.T, err error, kube fakeKube) {
		return func(t *testing.T, err error, kube fakeKube) {
			if err != nil {
				t.Errorf("want nil error, got %v", err)
			}

			if !reflect.DeepEqual(want, kube.storage) {
				t.Errorf("want %v, got %v", kube.storage, kube.storage)
			}
		}
	}

	errIsNotNil := func(t *testing.T, want storage.Storage) func(t *testing.T, err error, kube fakeKube) {
		return func(t *testing.T, err error, kube fakeKube) {
			if err == nil {
				t.Errorf("want an error, got nil")
			}
		}
	}

	// define test input
	tests := map[string]func(t *testing.T) (*pb.StorageUpdateRequest, fakeKube, checkFn){
		"success": func(t *testing.T) (*pb.StorageUpdateRequest, fakeKube, checkFn) {
			req := &pb.StorageUpdateRequest{
				StorageType: "powerflex",
				SystemId:    "11e4e7d35817bd0f",
				Endpoint:    "https://10.0.0.10",
				UserName:    "admin",
				Password:    "test",
				Insecure:    false,
			}

			updatedStorage := storage.Storage{
				"powerflex": storage.SystemType{
					"11e4e7d35817bd0f": storage.System{
						User:     "admin",
						Password: "test",
						Endpoint: "https://10.0.0.10",
						Insecure: false,
					},
				},
			}
			cfgStorage := storage.Storage{
				"powerflex": storage.SystemType{
					"11e4e7d35817bd0f": storage.System{
						User:     "admin",
						Password: "test",
						Endpoint: "https://10.0.0.1",
						Insecure: false,
					},
				},
			}

			getConfiguredStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return cfgStorage, nil
			}

			updateStorageFn := func(ctx context.Context, storages storage.Storage) error {
				return nil
			}

			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
				UpdateStoragesRn:       updateStorageFn,
				storage:                cfgStorage,
			}

			return req, kube, checkExpected(t, updatedStorage)
		},
		"fail get configured storage": func(t *testing.T) (*pb.StorageUpdateRequest, fakeKube, checkFn) {
			req := &pb.StorageUpdateRequest{
				StorageType: "powerflex",
				SystemId:    "11e4e7d35817bd0f",
				Endpoint:    "https://10.0.0.10",
				UserName:    "admin",
				Password:    "test",
				Insecure:    true,
			}

			getConfiguredStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return nil, errors.New("error")
			}

			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
			}

			return req, kube, errIsNotNil(t, nil)
		},
		"fail update storage": func(t *testing.T) (*pb.StorageUpdateRequest, fakeKube, checkFn) {
			req := &pb.StorageUpdateRequest{
				StorageType: "powerflex",
				SystemId:    "11e4e7d35817bd0f",
				Endpoint:    "https://10.0.0.10",
				UserName:    "admin",
				Password:    "test",
				Insecure:    true,
			}

			cfgStorage := storage.Storage{
				"powerflex": storage.SystemType{
					"11e4e7d35817bd0f": storage.System{
						User:     "admin",
						Password: "test",
						Endpoint: "https://10.0.0.1",
						Insecure: false,
					},
				},
			}

			getConfiguredStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return cfgStorage, nil
			}

			updateStorageFn := func(ctx context.Context, storages storage.Storage) error {
				return errors.New("error")
			}

			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
				UpdateStoragesRn:       updateStorageFn,
			}

			return req, kube, errIsNotNil(t, nil)
		},
		"storage not found": func(t *testing.T) (*pb.StorageUpdateRequest, fakeKube, checkFn) {
			req := &pb.StorageUpdateRequest{
				StorageType: "powerflex",
				SystemId:    "11e4e7d35817bd0f",
				Endpoint:    "https://10.0.0.10",
				UserName:    "admin",
				Password:    "test",
				Insecure:    false,
			}

			cfgStorage := storage.Storage{
				"powerflex": storage.SystemType{
					"11e4e7d35817bd0g": storage.System{
						User:     "admin",
						Password: "test",
						Endpoint: "https://10.0.0.1",
						Insecure: false,
					},
				},
			}

			getConfiguredStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return cfgStorage, nil
			}

			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
				storage:                cfgStorage,
			}

			return req, kube, errIsNotNil(t, nil)
		},
	}

	// run the tests
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req, kube, checkFn := tc(t)
			svc := service.NewService(kube, nil)
			_, err := svc.Update(context.Background(), req)
			checkFn(t, err, kube)
		})
	}
}

func TestServiceGet(t *testing.T) {
	// define check functions to pass or fail tests
	type checkFn func(t *testing.T, err error, got *pb.StorageGetResponse)

	checkExpected := func(t *testing.T, want string) func(t *testing.T, err error, got *pb.StorageGetResponse) {
		return func(t *testing.T, err error, got *pb.StorageGetResponse) {
			if err != nil {
				t.Errorf("want nil error, got %v", err)
			}

			if want != string(got.Storage) {
				t.Errorf("want %s, got %s", want, string(got.Storage))
			}
		}
	}

	errIsNotNil := func(t *testing.T, want string) func(t *testing.T, err error, got *pb.StorageGetResponse) {
		return func(t *testing.T, err error, got *pb.StorageGetResponse) {
			if err == nil {
				t.Errorf("expected non-nil err")
			}
		}
	}

	// define test input
	tests := map[string]func(t *testing.T) (*pb.StorageGetRequest, service.Kube, checkFn){
		"success": func(t *testing.T) (*pb.StorageGetRequest, service.Kube, checkFn) {
			getStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return storage.Storage{
					"powerflex": storage.SystemType{
						"11e4e7d35817bd0f": storage.System{
							User:     "admin",
							Password: "test",
							Endpoint: "https://10.0.0.1",
							Insecure: false,
						},
					},
				}, nil
			}

			req := &pb.StorageGetRequest{
				StorageType: "powerflex",
				SystemId:    "11e4e7d35817bd0f",
			}

			want := `{"User":"admin","Password":"(omitted)","Endpoint":"https://10.0.0.1","Insecure":false}`
			return req, fakeKube{GetConfiguredStorageFn: getStorageFn}, checkExpected(t, want)
		},
		"error getting configured storage": func(t *testing.T) (*pb.StorageGetRequest, service.Kube, checkFn) {
			getStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return nil, errors.New("error")
			}

			req := &pb.StorageGetRequest{
				StorageType: "powerflex",
				SystemId:    "non-existing-system-id",
			}

			return req, fakeKube{GetConfiguredStorageFn: getStorageFn}, errIsNotNil(t, "")
		},
		"error system type missing": func(t *testing.T) (*pb.StorageGetRequest, service.Kube, checkFn) {
			getStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return storage.Storage{
					"powermax": storage.SystemType{
						"11e4e7d35817bd0f": storage.System{
							User:     "admin",
							Password: "test",
							Endpoint: "https://10.0.0.1",
							Insecure: false,
						},
					},
				}, nil
			}

			req := &pb.StorageGetRequest{
				StorageType: "powerflex",
				SystemId:    "11e4e7d35817bd0f",
			}

			return req, fakeKube{GetConfiguredStorageFn: getStorageFn}, errIsNotNil(t, "")
		},
		"error system id missing": func(t *testing.T) (*pb.StorageGetRequest, service.Kube, checkFn) {
			getStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return storage.Storage{
					"powerflex": storage.SystemType{
						"11e4e7d35817bd0f": storage.System{
							User:     "admin",
							Password: "test",
							Endpoint: "https://10.0.0.1",
							Insecure: false,
						},
					},
				}, nil
			}

			req := &pb.StorageGetRequest{
				StorageType: "powerflex",
				SystemId:    "non-existing-system-id",
			}

			return req, fakeKube{GetConfiguredStorageFn: getStorageFn}, errIsNotNil(t, "")
		},
	}

	// run the tests
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req, kube, checkFn := tc(t)
			svc := service.NewService(kube, nil)
			resp, err := svc.Get(context.Background(), req)
			checkFn(t, err, resp)
		})
	}
}

func TestServiceDelete(t *testing.T) {
	// define check functions to pass or fail tests
	type checkFn func(t *testing.T, err error, kube fakeKube)

	checkExpected := func(t *testing.T, want storage.Storage) func(t *testing.T, err error, kube fakeKube) {
		return func(t *testing.T, err error, kube fakeKube) {
			if err != nil {
				t.Errorf("want nil error, got %v", err)
			}

			if !reflect.DeepEqual(want, kube.storage) {
				t.Errorf("want %v, got %v", kube.storage, kube.storage)
			}
		}
	}

	errIsNotNil := func(t *testing.T, want storage.Storage) func(t *testing.T, err error, kube fakeKube) {
		return func(t *testing.T, err error, kube fakeKube) {
			if err == nil {
				t.Errorf("want an error, got nil")
			}
		}
	}

	// define test input
	tests := map[string]func(t *testing.T) (*pb.StorageDeleteRequest, fakeKube, checkFn){
		"success": func(t *testing.T) (*pb.StorageDeleteRequest, fakeKube, checkFn) {
			req := &pb.StorageDeleteRequest{
				StorageType: "powerflex",
				SystemId:    "11e4e7d35817bd0f",
			}

			cfgStorage := storage.Storage{
				"powerflex": storage.SystemType{
					"11e4e7d35817bd0f": storage.System{
						User:     "admin",
						Password: "test",
						Endpoint: "https://10.0.0.1",
						Insecure: false,
					},
					"308cafa32643240d": storage.System{
						User:     "admin",
						Password: "test",
						Endpoint: "https://10.0.0.1",
						Insecure: false,
					},
				},
			}

			updatedStorage := storage.Storage{
				"powerflex": storage.SystemType{
					"308cafa32643240d": storage.System{
						User:     "admin",
						Password: "test",
						Endpoint: "https://10.0.0.1",
						Insecure: false,
					},
				},
			}

			getConfiguredStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return cfgStorage, nil
			}

			updateStorageFn := func(ctx context.Context, storages storage.Storage) error {
				return nil
			}

			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
				UpdateStoragesRn:       updateStorageFn,
				storage:                cfgStorage,
			}

			return req, kube, checkExpected(t, updatedStorage)
		},
		"fail get configured storage": func(t *testing.T) (*pb.StorageDeleteRequest, fakeKube, checkFn) {
			req := &pb.StorageDeleteRequest{
				StorageType: "powerflex",
				SystemId:    "11e4e7d35817bd0f",
			}

			getConfiguredStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return nil, errors.New("error")
			}

			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
			}

			return req, kube, errIsNotNil(t, nil)
		},
		"fail delete storage": func(t *testing.T) (*pb.StorageDeleteRequest, fakeKube, checkFn) {
			req := &pb.StorageDeleteRequest{
				StorageType: "powerflex",
				SystemId:    "11e4e7d35817bd0f",
			}

			cfgStorage := storage.Storage{
				"powerflex": storage.SystemType{
					"11e4e7d35817bd0f": storage.System{
						User:     "admin",
						Password: "test",
						Endpoint: "https://10.0.0.1",
						Insecure: false,
					},
				},
			}

			getConfiguredStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return cfgStorage, nil
			}

			updateStorageFn := func(ctx context.Context, storages storage.Storage) error {
				return errors.New("error")
			}

			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
				UpdateStoragesRn:       updateStorageFn,
			}

			return req, kube, errIsNotNil(t, nil)
		},
		"storage not found": func(t *testing.T) (*pb.StorageDeleteRequest, fakeKube, checkFn) {
			req := &pb.StorageDeleteRequest{
				StorageType: "powerflex",
				SystemId:    "11e4e7d35817bd0f",
			}

			cfgStorage := storage.Storage{
				"powerflex": storage.SystemType{
					"11e4e7d35817bd0g": storage.System{
						User:     "admin",
						Password: "test",
						Endpoint: "https://10.0.0.1",
						Insecure: false,
					},
				},
			}

			getConfiguredStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return cfgStorage, nil
			}

			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
				storage:                cfgStorage,
			}

			return req, kube, errIsNotNil(t, nil)
		},
	}

	// run the tests
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req, kube, checkFn := tc(t)
			svc := service.NewService(kube, nil)
			_, err := svc.Delete(context.Background(), req)
			checkFn(t, err, kube)
		})
	}
}

func TestServiceGetPowerflexVolumes(t *testing.T) {
	// define check functions to pass or fail tests
	type checkFn func(t *testing.T, err error, resp *pb.GetPowerflexVolumesResponse)

	checkExpected := func(t *testing.T, want []*pb.Volume) func(*testing.T, error, *pb.GetPowerflexVolumesResponse) {
		return func(t *testing.T, err error, resp *pb.GetPowerflexVolumesResponse) {
			if err != nil {
				t.Errorf("want nil error, got %v", err)
			}

			if !reflect.DeepEqual(want, resp.Volume) {
				t.Errorf("want %v\ngot %v", want, resp.Volume)
			}
		}
	}

	errNotNil := func(t *testing.T, want []*pb.Volume) func(*testing.T, error, *pb.GetPowerflexVolumesResponse) {
		return func(t *testing.T, err error, resp *pb.GetPowerflexVolumesResponse) {
			if err == nil {
				t.Errorf("want an error, got nil")
			}
		}
	}

	// define the tests
	tests := map[string]func(t *testing.T) (*pb.GetPowerflexVolumesRequest, fakeKube, *httptest.Server, checkFn){
		"success": func(t *testing.T) (*pb.GetPowerflexVolumesRequest, fakeKube, *httptest.Server, checkFn) {
			// setup mock powerflex
			mockPowerflex := httptest.NewTLSServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api/login":
						fmt.Fprintf(w, `"token"`)
					case "/api/version":
						fmt.Fprintf(w, "3.5")
					case "/api/types/Volume/instances/action/queryIdByKey":
						body, err := io.ReadAll(r.Body)
						if err != nil {
							t.Fatal(err)
						}
						if strings.Contains(string(body), "volume1") {
							fmt.Fprintf(w, "volume1Id")
						}
						if strings.Contains(string(body), "volume2") {
							fmt.Fprintf(w, "volume2Id")
						}
					case "/api/instances/Volume::volume1Id":
						b, err := os.ReadFile("testdata/powerflex_api_instances_volume_volume1Id.json")
						if err != nil {
							t.Fatal(err)
						}
						w.Write(b)
					case "/api/instances/Volume::volume2Id":
						b, err := os.ReadFile("testdata/powerflex_api_instances_volume_volume2Id.json")
						if err != nil {
							t.Fatal(err)
						}
						w.Write(b)
					case "/api/types/StoragePool/instances":
						b, err := os.ReadFile("testdata/powerflex_api_types_storagepool_instances.json")
						if err != nil {
							t.Fatal(err)
						}
						w.Write(b)
					default:
						t.Errorf("unhandled request path: %s", r.URL.Path)
					}
				}))

			// define the input request
			req := &pb.GetPowerflexVolumesRequest{
				SystemId:   "systemId1",
				VolumeName: []string{"volume1", "volume2"},
			}

			// define mock storage data
			cfgStorage := storage.Storage{
				"powerflex": storage.SystemType{
					"systemId1": storage.System{
						User:     "admin",
						Password: "test",
						Endpoint: mockPowerflex.URL,
						Insecure: true,
					},
				},
			}

			// define the fake k8s client
			getConfiguredStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return cfgStorage, nil
			}
			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
			}

			// define test scenario
			want := []*pb.Volume{
				{
					Name:     "volume1",
					Size:     8,
					SystemId: "systemId1",
					Id:       "volumeId1",
					Pool:     "pool1",
				},
				{
					Name:     "volume2",
					Size:     8,
					SystemId: "systemId1",
					Id:       "volumeId2",
					Pool:     "pool2",
				},
			}
			return req, kube, mockPowerflex, checkExpected(t, want)
		},
		"error getting configured storage": func(t *testing.T) (*pb.GetPowerflexVolumesRequest, fakeKube, *httptest.Server, checkFn) {
			mockPowerflex := httptest.NewTLSServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

			// define the input request
			req := &pb.GetPowerflexVolumesRequest{
				SystemId:   "systemId1",
				VolumeName: []string{"volume1", "volume2"},
			}

			// define the fake k8s client
			getConfiguredStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return nil, fmt.Errorf("error")
			}
			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
			}
			return req, kube, mockPowerflex, errNotNil(t, nil)
		},
		"error no powerflex storage configured": func(t *testing.T) (*pb.GetPowerflexVolumesRequest, fakeKube, *httptest.Server, checkFn) {
			mockPowerflex := httptest.NewTLSServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

			// define the input request
			req := &pb.GetPowerflexVolumesRequest{
				SystemId:   "systemId1",
				VolumeName: []string{"volume1", "volume2"},
			}

			// define mock storage data
			cfgStorage := storage.Storage{
				"powermax": storage.SystemType{
					"systemId1": storage.System{
						User:     "admin",
						Password: "test",
						Endpoint: mockPowerflex.URL,
						Insecure: true,
					},
				},
			}

			// define the fake k8s client
			getConfiguredStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return cfgStorage, nil
			}
			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
			}
			return req, kube, mockPowerflex, errNotNil(t, nil)
		},
		"error system is not configured": func(t *testing.T) (*pb.GetPowerflexVolumesRequest, fakeKube, *httptest.Server, checkFn) {
			mockPowerflex := httptest.NewTLSServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

			// define the input request
			req := &pb.GetPowerflexVolumesRequest{
				SystemId:   "systemId",
				VolumeName: []string{"volume1", "volume2"},
			}

			// define mock storage data
			cfgStorage := storage.Storage{
				"powerflex": storage.SystemType{
					"systemId1": storage.System{
						User:     "admin",
						Password: "test",
						Endpoint: mockPowerflex.URL,
						Insecure: true,
					},
				},
			}

			// define the fake k8s client
			getConfiguredStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return cfgStorage, nil
			}
			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
			}
			return req, kube, mockPowerflex, errNotNil(t, nil)
		},
		"error authenticating to powerflex": func(t *testing.T) (*pb.GetPowerflexVolumesRequest, fakeKube, *httptest.Server, checkFn) {
			mockPowerflex := httptest.NewTLSServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api/login":
						w.WriteHeader(http.StatusUnauthorized)
					default:
						t.Errorf("unhandled request path: %s", r.URL.Path)
					}
				}))

			// define the input request
			req := &pb.GetPowerflexVolumesRequest{
				SystemId:   "systemId1",
				VolumeName: []string{"volume1", "volume2"},
			}

			// define mock storage data
			cfgStorage := storage.Storage{
				"powerflex": storage.SystemType{
					"systemId1": storage.System{
						User:     "admin",
						Password: "test",
						Endpoint: mockPowerflex.URL,
						Insecure: true,
					},
				},
			}

			// define the fake k8s client
			getConfiguredStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return cfgStorage, nil
			}
			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
			}
			return req, kube, mockPowerflex, errNotNil(t, nil)
		},
		"error getting a volume from powerflex": func(t *testing.T) (*pb.GetPowerflexVolumesRequest, fakeKube, *httptest.Server, checkFn) {
			mockPowerflex := httptest.NewTLSServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api/login":
						fmt.Fprintf(w, `"token"`)
					case "/api/version":
						fmt.Fprintf(w, "3.5")
					case "/api/types/Volume/instances/action/queryIdByKey":
						w.WriteHeader(http.StatusInternalServerError)
					default:
						t.Errorf("unhandled request path: %s", r.URL.Path)
					}
				}))

			// define the input request
			req := &pb.GetPowerflexVolumesRequest{
				SystemId:   "systemId1",
				VolumeName: []string{"volume1", "volume2"},
			}

			// define mock storage data
			cfgStorage := storage.Storage{
				"powerflex": storage.SystemType{
					"systemId1": storage.System{
						User:     "admin",
						Password: "test",
						Endpoint: mockPowerflex.URL,
						Insecure: true,
					},
				},
			}

			// define the fake k8s client
			getConfiguredStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return cfgStorage, nil
			}
			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
			}
			return req, kube, mockPowerflex, errNotNil(t, nil)
		},
		"error getting relevant storage pool": func(t *testing.T) (*pb.GetPowerflexVolumesRequest, fakeKube, *httptest.Server, checkFn) {
			mockPowerflex := httptest.NewTLSServer(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					switch r.URL.Path {
					case "/api/login":
						fmt.Fprintf(w, `"token"`)
					case "/api/version":
						fmt.Fprintf(w, "3.5")
					case "/api/types/Volume/instances/action/queryIdByKey":
						body, err := io.ReadAll(r.Body)
						if err != nil {
							t.Fatal(err)
						}
						if strings.Contains(string(body), "volume1") {
							fmt.Fprintf(w, "volume1Id")
						}
						if strings.Contains(string(body), "volume2") {
							fmt.Fprintf(w, "volume2Id")
						}
					case "/api/instances/Volume::volume1Id":
						b, err := os.ReadFile("testdata/powerflex_api_instances_volume_volume1Id.json")
						if err != nil {
							t.Fatal(err)
						}
						w.Write(b)
					case "/api/instances/Volume::volume2Id":
						b, err := os.ReadFile("testdata/powerflex_api_instances_volume_volume2Id.json")
						if err != nil {
							t.Fatal(err)
						}
						w.Write(b)
					case "/api/types/StoragePool/instances":
						w.WriteHeader(http.StatusInternalServerError)
					default:
						t.Errorf("unhandled request path: %s", r.URL.Path)
					}
				}))

			// define the input request
			req := &pb.GetPowerflexVolumesRequest{
				SystemId:   "systemId1",
				VolumeName: []string{"volume1", "volume2"},
			}

			// define mock storage data
			cfgStorage := storage.Storage{
				"powerflex": storage.SystemType{
					"systemId1": storage.System{
						User:     "admin",
						Password: "test",
						Endpoint: mockPowerflex.URL,
						Insecure: true,
					},
				},
			}

			// define the fake k8s client
			getConfiguredStorageFn := func(ctx context.Context) (storage.Storage, error) {
				return cfgStorage, nil
			}
			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
			}
			return req, kube, mockPowerflex, errNotNil(t, nil)
		},
	}

	// run the tests
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req, kube, mockPowerflex, checkFn := tc(t)
			defer mockPowerflex.Close()

			svc := service.NewService(kube, nil)
			svc.SetConcurrentPowerFlexRequests(10)
			resp, err := svc.GetPowerflexVolumes(context.Background(), req)
			checkFn(t, err, resp)
		})
	}
}

func TestCheckForDuplicates(t *testing.T) {
	// define check functions to pass or fail tests
	type checkFn func(*testing.T, error)

	tests := map[string]func(t *testing.T) (storage.Storage, string, checkFn){
		"Passed_No_Duplicates": func(t *testing.T) (storage.Storage, string, checkFn) {
			existingStorages := storage.Storage{
				"powerflex": storage.SystemType{
					"542a2d5f5122210f": storage.System{},
				},
			}

			return existingStorages, "different-system-id", errIsNil
		},
		"failed systemID exists": func(t *testing.T) (storage.Storage, string, checkFn) {
			existingStorages := storage.Storage{
				"powerflex": storage.SystemType{
					"542a2d5f5122210f": storage.System{},
				},
			}
			return existingStorages, "542a2d5f5122210f", errIsNotNil
		},
	}

	// run the tests
	for name, testcase := range tests {
		t.Run(name, func(t *testing.T) {
			existingStorages, systemID, checkFn := testcase(t)
			err := service.CheckForDuplicates(context.Background(), existingStorages, systemID, "powerflex")
			checkFn(t, err)
		})
	}
}

func errIsNil(t *testing.T, err error) {
	if err != nil {
		t.Errorf("expected nil err, got %v", err)
	}
}

func errIsNotNil(t *testing.T, err error) {
	if err == nil {
		t.Errorf("expected non-nil err")
	}
}

type successfulKube struct{}

func (k successfulKube) UpdateStorages(_ context.Context, _ storage.Storage) error {
	return nil
}

func (k successfulKube) GetConfiguredStorage(_ context.Context) (storage.Storage, error) {
	return storage.Storage{}, nil
}

type failKube struct{}

func (k failKube) UpdateStorages(_ context.Context, _ storage.Storage) error {
	return errors.New("error")
}

func (k failKube) GetConfiguredStorage(_ context.Context) (storage.Storage, error) {
	return nil, nil
}

type successfulValidator struct{}

func (v successfulValidator) Validate(_ context.Context, _ string, _ string, _ storage.System) error {
	return nil
}

type failValidator struct{}

func (v failValidator) Validate(_ context.Context, _ string, _ string, _ storage.System) error {
	return errors.New("error")
}

type fakeKube struct {
	UpdateStoragesRn       func(ctx context.Context, storages storage.Storage) error
	GetConfiguredStorageFn func(ctx context.Context) (storage.Storage, error)
	storage                storage.Storage
}

func (k fakeKube) UpdateStorages(ctx context.Context, storages storage.Storage) error {
	k.storage = storages
	if k.UpdateStoragesRn != nil {
		return k.UpdateStoragesRn(ctx, storages)
	}
	return nil
}

func (k fakeKube) GetConfiguredStorage(ctx context.Context) (storage.Storage, error) {
	if k.GetConfiguredStorageFn != nil {
		return k.GetConfiguredStorageFn(ctx)
	}
	return storage.Storage{}, nil
}
