// Copyright © 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"karavi-authorization/internal/storage-service"
	"karavi-authorization/internal/types"
	"karavi-authorization/pb"
	"reflect"
	"testing"
)

func TestServiceCreate(t *testing.T) {

	// define test input
	tests := map[string]func(t *testing.T) (*pb.StorageCreateRequest, storage.Validator, storage.Kube, checkFn){
		"success": func(t *testing.T) (*pb.StorageCreateRequest, storage.Validator, storage.Kube, checkFn) {
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
		"fail validation": func(t *testing.T) (*pb.StorageCreateRequest, storage.Validator, storage.Kube, checkFn) {
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
		"fail kube and validation": func(t *testing.T) (*pb.StorageCreateRequest, storage.Validator, storage.Kube, checkFn) {
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
		"fail kube": func(t *testing.T) (*pb.StorageCreateRequest, storage.Validator, storage.Kube, checkFn) {
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
			svc := storage.NewService(kube, validator)
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
	tests := map[string]func(t *testing.T) (storage.Kube, checkFn){
		"success": func(t *testing.T) (storage.Kube, checkFn) {
			getStorageFn := func(ctx context.Context) (types.Storage, error) {
				return types.Storage{
					"powerflex": types.SystemType{
						"11e4e7d35817bd0f": types.System{
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
		"error getting configured storage": func(t *testing.T) (storage.Kube, checkFn) {
			getStorageFn := func(ctx context.Context) (types.Storage, error) {
				return nil, errors.New("error")
			}
			return fakeKube{GetConfiguredStorageFn: getStorageFn}, errIsNotNil(t, "")
		},
	}

	// run the tests
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			kube, checkFn := tc(t)
			svc := storage.NewService(kube, nil)
			resp, err := svc.List(context.Background(), &pb.StorageListRequest{})
			checkFn(t, err, resp)
		})
	}
}

func TestServiceUpdate(t *testing.T) {
	// define check functions to pass or fail tests
	type checkFn func(t *testing.T, err error, kube fakeKube)

	checkExpected := func(t *testing.T, want types.Storage) func(t *testing.T, err error, kube fakeKube) {
		return func(t *testing.T, err error, kube fakeKube) {
			if err != nil {
				t.Errorf("want nil error, got %v", err)
			}

			if !reflect.DeepEqual(want, kube.storage) {
				t.Errorf("want %v, got %v", kube.storage, kube.storage)
			}
		}
	}

	errIsNotNil := func(t *testing.T, want types.Storage) func(t *testing.T, err error, kube fakeKube) {
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

			updatedStorage := types.Storage{
				"powerflex": types.SystemType{
					"11e4e7d35817bd0f": types.System{
						User:     "admin",
						Password: "test",
						Endpoint: "https://10.0.0.10",
						Insecure: false,
					},
				},
			}
			storage := types.Storage{
				"powerflex": types.SystemType{
					"11e4e7d35817bd0f": types.System{
						User:     "admin",
						Password: "test",
						Endpoint: "https://10.0.0.1",
						Insecure: false,
					},
				},
			}

			getConfiguredStorageFn := func(ctx context.Context) (types.Storage, error) {
				return storage, nil
			}

			updateStorageFn := func(ctx context.Context, storages types.Storage) error {
				return nil
			}

			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
				UpdateStoragesRn:       updateStorageFn,
				storage:                storage,
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

			getConfiguredStorageFn := func(ctx context.Context) (types.Storage, error) {
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

			storage := types.Storage{
				"powerflex": types.SystemType{
					"11e4e7d35817bd0f": types.System{
						User:     "admin",
						Password: "test",
						Endpoint: "https://10.0.0.1",
						Insecure: false,
					},
				},
			}

			getConfiguredStorageFn := func(ctx context.Context) (types.Storage, error) {
				return storage, nil
			}

			updateStorageFn := func(ctx context.Context, storages types.Storage) error {
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

			storage := types.Storage{
				"powerflex": types.SystemType{
					"11e4e7d35817bd0g": types.System{
						User:     "admin",
						Password: "test",
						Endpoint: "https://10.0.0.1",
						Insecure: false,
					},
				},
			}

			getConfiguredStorageFn := func(ctx context.Context) (types.Storage, error) {
				return storage, nil
			}

			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
				storage:                storage,
			}

			return req, kube, errIsNotNil(t, nil)
		},
	}

	// run the tests
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req, kube, checkFn := tc(t)
			svc := storage.NewService(kube, nil)
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
	tests := map[string]func(t *testing.T) (*pb.StorageGetRequest, storage.Kube, checkFn){
		"success": func(t *testing.T) (*pb.StorageGetRequest, storage.Kube, checkFn) {
			getStorageFn := func(ctx context.Context) (types.Storage, error) {

				return types.Storage{
					"powerflex": types.SystemType{
						"11e4e7d35817bd0f": types.System{
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
		"error getting configured storage": func(t *testing.T) (*pb.StorageGetRequest, storage.Kube, checkFn) {
			getStorageFn := func(ctx context.Context) (types.Storage, error) {
				return nil, errors.New("error")
			}

			req := &pb.StorageGetRequest{
				StorageType: "powerflex",
				SystemId:    "non-existing-system-id",
			}

			return req, fakeKube{GetConfiguredStorageFn: getStorageFn}, errIsNotNil(t, "")
		},
		"error system type missing": func(t *testing.T) (*pb.StorageGetRequest, storage.Kube, checkFn) {
			getStorageFn := func(ctx context.Context) (types.Storage, error) {
				return types.Storage{
					"powermax": types.SystemType{
						"11e4e7d35817bd0f": types.System{
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
		"error system id missing": func(t *testing.T) (*pb.StorageGetRequest, storage.Kube, checkFn) {
			getStorageFn := func(ctx context.Context) (types.Storage, error) {
				return types.Storage{
					"powerflex": types.SystemType{
						"11e4e7d35817bd0f": types.System{
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
			svc := storage.NewService(kube, nil)
			resp, err := svc.Get(context.Background(), req)
			checkFn(t, err, resp)
		})
	}
}

func TestServiceDelete(t *testing.T) {
	// define check functions to pass or fail tests
	type checkFn func(t *testing.T, err error, kube fakeKube)

	checkExpected := func(t *testing.T, want types.Storage) func(t *testing.T, err error, kube fakeKube) {
		return func(t *testing.T, err error, kube fakeKube) {
			if err != nil {
				t.Errorf("want nil error, got %v", err)
			}

			if !reflect.DeepEqual(want, kube.storage) {
				t.Errorf("want %v, got %v", kube.storage, kube.storage)
			}
		}
	}

	errIsNotNil := func(t *testing.T, want types.Storage) func(t *testing.T, err error, kube fakeKube) {
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

			storage := types.Storage{
				"powerflex": types.SystemType{
					"11e4e7d35817bd0f": types.System{
						User:     "admin",
						Password: "test",
						Endpoint: "https://10.0.0.1",
						Insecure: false,
					},
					"308cafa32643240d": types.System{
						User:     "admin",
						Password: "test",
						Endpoint: "https://10.0.0.1",
						Insecure: false,
					},
				},
			}

			updatedStorage := types.Storage{
				"powerflex": types.SystemType{
					"308cafa32643240d": types.System{
						User:     "admin",
						Password: "test",
						Endpoint: "https://10.0.0.1",
						Insecure: false,
					},
				},
			}

			getConfiguredStorageFn := func(ctx context.Context) (types.Storage, error) {
				return storage, nil
			}

			updateStorageFn := func(ctx context.Context, storages types.Storage) error {
				return nil
			}

			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
				UpdateStoragesRn:       updateStorageFn,
				storage:                storage,
			}

			return req, kube, checkExpected(t, updatedStorage)
		},
		"fail get configured storage": func(t *testing.T) (*pb.StorageDeleteRequest, fakeKube, checkFn) {
			req := &pb.StorageDeleteRequest{
				StorageType: "powerflex",
				SystemId:    "11e4e7d35817bd0f",
			}

			getConfiguredStorageFn := func(ctx context.Context) (types.Storage, error) {
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

			storage := types.Storage{
				"powerflex": types.SystemType{
					"11e4e7d35817bd0f": types.System{
						User:     "admin",
						Password: "test",
						Endpoint: "https://10.0.0.1",
						Insecure: false,
					},
				},
			}

			getConfiguredStorageFn := func(ctx context.Context) (types.Storage, error) {
				return storage, nil
			}

			updateStorageFn := func(ctx context.Context, storages types.Storage) error {
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

			storage := types.Storage{
				"powerflex": types.SystemType{
					"11e4e7d35817bd0g": types.System{
						User:     "admin",
						Password: "test",
						Endpoint: "https://10.0.0.1",
						Insecure: false,
					},
				},
			}

			getConfiguredStorageFn := func(ctx context.Context) (types.Storage, error) {
				return storage, nil
			}

			kube := fakeKube{
				GetConfiguredStorageFn: getConfiguredStorageFn,
				storage:                storage,
			}

			return req, kube, errIsNotNil(t, nil)
		},
	}

	// run the tests
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req, kube, checkFn := tc(t)
			svc := storage.NewService(kube, nil)
			_, err := svc.Delete(context.Background(), req)
			checkFn(t, err, kube)
		})
	}
}

func TestCheckForDuplicates(t *testing.T) {

	tests := map[string]func(t *testing.T) (types.Storage, string, checkFn){
		"Passed_No_Duplicates": func(t *testing.T) (types.Storage, string, checkFn) {
			existingStorages := types.Storage{
				"powerflex": types.SystemType{
					"542a2d5f5122210f": types.System{},
				},
			}

			return existingStorages, "different-system-id", errIsNil
		},
		"failed systemID exists": func(t *testing.T) (types.Storage, string, checkFn) {
			existingStorages := types.Storage{
				"powerflex": types.SystemType{
					"542a2d5f5122210f": types.System{},
				},
			}
			return existingStorages, "542a2d5f5122210f", errIsNotNil
		},
	}

	// run the tests
	for name, testcase := range tests {
		t.Run(name, func(t *testing.T) {
			existingStorages, systemID, checkFn := testcase(t)
			err := storage.CheckForDuplicates(context.Background(), existingStorages, systemID, "powerflex")
			checkFn(t, err)
		})
	}
}

// define check functions to pass or fail tests
type checkFn func(*testing.T, error)

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

func (k successfulKube) UpdateStorages(ctx context.Context, storages types.Storage) error {
	return nil
}

func (k successfulKube) GetConfiguredStorage(ctx context.Context) (types.Storage, error) {
	return types.Storage{}, nil
}

type failKube struct{}

func (k failKube) UpdateStorages(ctx context.Context, storages types.Storage) error {
	return errors.New("error")
}

func (k failKube) GetConfiguredStorage(ctx context.Context) (types.Storage, error) {
	return nil, nil
}

type successfulValidator struct{}

func (v successfulValidator) Validate(ctx context.Context, systemID string, systemType string, system types.System) error {
	return nil
}

type failValidator struct{}

func (v failValidator) Validate(ctx context.Context, systemID string, systemType string, system types.System) error {
	return errors.New("error")
}

type fakeKube struct {
	UpdateStoragesRn       func(ctx context.Context, storages types.Storage) error
	GetConfiguredStorageFn func(ctx context.Context) (types.Storage, error)
	storage                types.Storage
}

func (k fakeKube) UpdateStorages(ctx context.Context, storages types.Storage) error {
	k.storage = storages
	if k.UpdateStoragesRn != nil {
		return k.UpdateStoragesRn(ctx, storages)
	}
	return nil
}

func (k fakeKube) GetConfiguredStorage(ctx context.Context) (types.Storage, error) {
	if k.GetConfiguredStorageFn != nil {
		return k.GetConfiguredStorageFn(ctx)
	}
	return types.Storage{}, nil
}
