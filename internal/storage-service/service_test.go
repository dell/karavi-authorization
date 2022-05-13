<<<<<<< HEAD
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
=======
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
>>>>>>> d02e067 (Fix code check errors)
