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

package role_test

import (
	"context"
	"errors"
	"karavi-authorization/internal/role-service"
	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/pb"
	"testing"
)

func TestServiceCreate(t *testing.T) {
	// define check functions to pass or fail tests
	type checkFn func(*testing.T, error)

	errIsNil := func(t *testing.T, err error) {
		if err != nil {
			t.Errorf("expected nil err, got %v", err)
		}
	}

	errIsNotNil := func(t *testing.T, err error) {
		if err == nil {
			t.Errorf("expected non-nil err")
		}
	}

	// define test input
	tests := map[string]func(t *testing.T) (*pb.RoleCreateRequest, role.Validator, role.Kube, checkFn){
		"success": func(_ *testing.T) (*pb.RoleCreateRequest, role.Validator, role.Kube, checkFn) {
			req := &pb.RoleCreateRequest{
				Name:        "test",
				StorageType: "powerflex",
				SystemId:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "9GB",
			}

			getRolesFn := func(_ context.Context) (*roles.JSON, error) {
				r := roles.NewJSON()
				return &r, nil
			}

			return req, successfulValidator{}, fakeKube{GetConfiguredRolesFn: getRolesFn}, errIsNil
		},
		"fail validation": func(_ *testing.T) (*pb.RoleCreateRequest, role.Validator, role.Kube, checkFn) {
			req := &pb.RoleCreateRequest{
				Name:        "test",
				StorageType: "powerflex",
				SystemId:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "-1",
			}

			getRolesFn := func(_ context.Context) (*roles.JSON, error) {
				r := roles.NewJSON()
				return &r, nil
			}

			return req, failValidator{}, fakeKube{GetConfiguredRolesFn: getRolesFn}, errIsNotNil
		},
		"fail update roles": func(t *testing.T) (*pb.RoleCreateRequest, role.Validator, role.Kube, checkFn) {
			req := &pb.RoleCreateRequest{
				Name:        "test",
				StorageType: "powerflex",
				SystemId:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "20GB",
			}

			ri, err := roles.NewInstance("test", "powerflex", "542a2d5f5122210f", "bronze", "9GB")
			if err != nil {
				t.Fatal(err)
			}

			r := roles.NewJSON()
			err = r.Add(ri)
			if err != nil {
				t.Fatal(err)
			}

			getRolesFn := func(_ context.Context) (*roles.JSON, error) {
				return &r, nil
			}

			updateRolesFn := func(_ context.Context, _ *roles.JSON) error {
				return errors.New("error")
			}

			fakeClient := fakeKube{
				GetConfiguredRolesFn: getRolesFn,
				UpdateRolesRn:        updateRolesFn,
			}

			return req, successfulValidator{}, fakeClient, errIsNotNil
		},
	}

	// run the tests
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req, validator, kube, checkFn := tc(t)
			svc := role.NewService(kube, validator)
			_, err := svc.Create(context.Background(), req)
			checkFn(t, err)
		})
	}
}

func TestServiceDelete(t *testing.T) {
	// define check functions to pass or fail tests
	type checkFn func(*testing.T, error)

	errIsNil := func(t *testing.T, err error) {
		if err != nil {
			t.Errorf("expected nil err, got %v", err)
		}
	}

	errIsNotNil := func(t *testing.T, err error) {
		if err == nil {
			t.Errorf("expected non-nil err")
		}
	}

	// define test input
	tests := map[string]func(t *testing.T) (*pb.RoleDeleteRequest, role.Kube, checkFn){
		"success": func(t *testing.T) (*pb.RoleDeleteRequest, role.Kube, checkFn) {
			roleInstance, err := roles.NewInstance("test", "powerflex", "542a2d5f5122210f", "bronze", "9GB")
			if err != nil {
				t.Fatal(err)
			}

			rff := roles.NewJSON()
			err = rff.Add(roleInstance)
			if err != nil {
				t.Fatal(err)
			}

			getRolesFn := func(_ context.Context) (*roles.JSON, error) {
				return &rff, nil
			}

			r := &pb.RoleDeleteRequest{
				Name:        "test",
				StorageType: "powerflex",
				SystemId:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "9GB",
			}

			return r, fakeKube{GetConfiguredRolesFn: getRolesFn}, errIsNil
		},
		"role not found": func(t *testing.T) (*pb.RoleDeleteRequest, role.Kube, checkFn) {
			roleInstance, err := roles.NewInstance("test", "powerflex", "542a2d5f5122210f", "bronze", "9GB")
			if err != nil {
				t.Fatal(err)
			}

			rff := roles.NewJSON()
			err = rff.Add(roleInstance)
			if err != nil {
				t.Fatal(err)
			}

			getRolesFn := func(_ context.Context) (*roles.JSON, error) {
				return &rff, nil
			}

			r := &pb.RoleDeleteRequest{
				Name:        "notFound",
				StorageType: "powerflex",
				SystemId:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "9GB",
			}

			return r, fakeKube{GetConfiguredRolesFn: getRolesFn}, errIsNotNil
		},
	}

	// run the tests
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req, kube, checkFn := tc(t)
			svc := role.NewService(kube, successfulValidator{})
			_, err := svc.Delete(context.Background(), req)
			checkFn(t, err)
		})
	}
}

func TestServiceList(t *testing.T) {
	// define check functions to pass or fail tests
	type checkFn func(t *testing.T, err error, got *pb.RoleListResponse)

	checkExpected := func(_ *testing.T, want string) func(t *testing.T, err error, got *pb.RoleListResponse) {
		return func(t *testing.T, err error, got *pb.RoleListResponse) {
			if err != nil {
				t.Errorf("want nil error, got %v", err)
			}

			if want != string(got.Roles) {
				t.Errorf("want %s, got %s", want, string(got.Roles))
			}
		}
	}

	errIsNotNil := func(_ *testing.T, _ string) func(t *testing.T, err error, got *pb.RoleListResponse) {
		return func(t *testing.T, err error, _ *pb.RoleListResponse) {
			if err == nil {
				t.Errorf("expected non-nil err")
			}
		}
	}

	// define test input
	tests := map[string]func(t *testing.T) (*pb.RoleListRequest, role.Kube, checkFn){
		"success": func(t *testing.T) (*pb.RoleListRequest, role.Kube, checkFn) {
			roleInstance, err := roles.NewInstance("test", "powerflex", "542a2d5f5122210f", "bronze", "9GB")
			if err != nil {
				t.Fatal(err)
			}

			rff := roles.NewJSON()
			err = rff.Add(roleInstance)
			if err != nil {
				t.Fatal(err)
			}

			getRolesFn := func(_ context.Context) (*roles.JSON, error) {
				return &rff, nil
			}

			want := `{"test":{"system_types":{"powerflex":{"system_ids":{"542a2d5f5122210f":{"pool_quotas":{"bronze":9000000}}}}}}}`

			return &pb.RoleListRequest{}, fakeKube{GetConfiguredRolesFn: getRolesFn}, checkExpected(t, want)
		},
		"error getting configured roles": func(t *testing.T) (*pb.RoleListRequest, role.Kube, checkFn) {
			roleInstance, err := roles.NewInstance("test", "powerflex", "542a2d5f5122210f", "bronze", "9GB")
			if err != nil {
				t.Fatal(err)
			}

			rff := roles.NewJSON()
			err = rff.Add(roleInstance)
			if err != nil {
				t.Fatal(err)
			}

			getRolesFn := func(_ context.Context) (*roles.JSON, error) {
				return nil, errors.New("error")
			}

			return &pb.RoleListRequest{}, fakeKube{GetConfiguredRolesFn: getRolesFn}, errIsNotNil(t, "")
		},
	}

	// run the tests
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req, kube, checkFn := tc(t)
			svc := role.NewService(kube, successfulValidator{})
			resp, err := svc.List(context.Background(), req)
			checkFn(t, err, resp)
		})
	}
}

func TestServiceGet(t *testing.T) {
	// define check functions to pass or fail tests
	type checkFn func(t *testing.T, err error, got *pb.RoleGetResponse)

	checkExpected := func(_ *testing.T, want string) func(t *testing.T, err error, got *pb.RoleGetResponse) {
		return func(t *testing.T, err error, got *pb.RoleGetResponse) {
			if err != nil {
				t.Errorf("want nil error, got %v", err)
			}

			if want != string(got.Role) {
				t.Errorf("want %s, got %s", want, string(got.Role))
			}
		}
	}

	errIsNotNil := func(_ *testing.T, _ string) func(t *testing.T, err error, got *pb.RoleGetResponse) {
		return func(t *testing.T, err error, _ *pb.RoleGetResponse) {
			if err == nil {
				t.Errorf("expected non-nil err")
			}
		}
	}

	// define test input
	tests := map[string]func(t *testing.T) (*pb.RoleGetRequest, role.Kube, checkFn){
		"success": func(t *testing.T) (*pb.RoleGetRequest, role.Kube, checkFn) {
			ri, err := roles.NewInstance("test", "powerflex", "542a2d5f5122210f", "bronze", "9GB")
			if err != nil {
				t.Fatal(err)
			}

			riTwo, err := roles.NewInstance("fizz", "powerflex", "542a2d5f5122210f", "silver", "9GB")
			if err != nil {
				t.Fatal(err)
			}

			rff := roles.NewJSON()
			err = rff.Add(ri)
			if err != nil {
				t.Fatal(err)
			}

			err = rff.Add(riTwo)
			if err != nil {
				t.Fatal(err)
			}

			getRolesFn := func(_ context.Context) (*roles.JSON, error) {
				return &rff, nil
			}

			want := `{"test":{"system_types":{"powerflex":{"system_ids":{"542a2d5f5122210f":{"pool_quotas":{"bronze":9000000}}}}}}}`

			return &pb.RoleGetRequest{Name: "test"}, fakeKube{GetConfiguredRolesFn: getRolesFn}, checkExpected(t, want)
		},
		"error getting configured roles": func(t *testing.T) (*pb.RoleGetRequest, role.Kube, checkFn) {
			roleInstance, err := roles.NewInstance("test", "powerflex", "542a2d5f5122210f", "bronze", "9GB")
			if err != nil {
				t.Fatal(err)
			}

			rff := roles.NewJSON()
			err = rff.Add(roleInstance)
			if err != nil {
				t.Fatal(err)
			}

			getRolesFn := func(_ context.Context) (*roles.JSON, error) {
				return nil, errors.New("error")
			}

			return &pb.RoleGetRequest{Name: "test"}, fakeKube{GetConfiguredRolesFn: getRolesFn}, errIsNotNil(t, "")
		},
	}

	// run the tests
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req, kube, checkFn := tc(t)
			svc := role.NewService(kube, successfulValidator{})
			resp, err := svc.Get(context.Background(), req)
			checkFn(t, err, resp)
		})
	}
}

func TestServiceUpdate(t *testing.T) {
	// define check functions to pass or fail tests
	type checkFn func(*testing.T, error)

	errIsNil := func(t *testing.T, err error) {
		if err != nil {
			t.Errorf("expected nil err, got %v", err)
		}
	}

	errIsNotNil := func(t *testing.T, err error) {
		if err == nil {
			t.Errorf("expected non-nil err")
		}
	}

	// define test input
	tests := map[string]func(t *testing.T) (*pb.RoleUpdateRequest, role.Validator, role.Kube, checkFn){
		"success update quota": func(t *testing.T) (*pb.RoleUpdateRequest, role.Validator, role.Kube, checkFn) {
			req := &pb.RoleUpdateRequest{
				Name:        "test",
				StorageType: "powerflex",
				SystemId:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "20GB",
			}

			ri, err := roles.NewInstance("test", "powerflex", "542a2d5f5122210f", "bronze", "9GB")
			if err != nil {
				t.Fatal(err)
			}

			r := roles.NewJSON()
			err = r.Add(ri)
			if err != nil {
				t.Fatal(err)
			}

			getRolesFn := func(_ context.Context) (*roles.JSON, error) {
				return &r, nil
			}

			return req, successfulValidator{}, fakeKube{GetConfiguredRolesFn: getRolesFn}, errIsNil
		},
		"fail update non-quota": func(t *testing.T) (*pb.RoleUpdateRequest, role.Validator, role.Kube, checkFn) {
			req := &pb.RoleUpdateRequest{
				Name:        "test",
				StorageType: "powerflex",
				SystemId:    "542a2d5f5122210f",
				Pool:        "silver",
				Quota:       "9GB",
			}

			ri, err := roles.NewInstance("test", "powerflex", "542a2d5f5122210f", "bronze", "9GB")
			if err != nil {
				t.Fatal(err)
			}

			r := roles.NewJSON()
			err = r.Add(ri)
			if err != nil {
				t.Fatal(err)
			}

			getRolesFn := func(_ context.Context) (*roles.JSON, error) {
				return &r, nil
			}

			return req, successfulValidator{}, fakeKube{GetConfiguredRolesFn: getRolesFn}, errIsNotNil
		},
		"fail validation": func(_ *testing.T) (*pb.RoleUpdateRequest, role.Validator, role.Kube, checkFn) {
			req := &pb.RoleUpdateRequest{
				Name:        "test",
				StorageType: "powerflex",
				SystemId:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "-1",
			}

			r := roles.NewJSON()
			getRolesFn := func(_ context.Context) (*roles.JSON, error) {
				return &r, nil
			}

			return req, failValidator{}, fakeKube{GetConfiguredRolesFn: getRolesFn}, errIsNotNil
		},
		"fail update roles": func(t *testing.T) (*pb.RoleUpdateRequest, role.Validator, role.Kube, checkFn) {
			req := &pb.RoleUpdateRequest{
				Name:        "test",
				StorageType: "powerflex",
				SystemId:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "20GB",
			}

			ri, err := roles.NewInstance("test", "powerflex", "542a2d5f5122210f", "bronze", "9GB")
			if err != nil {
				t.Fatal(err)
			}

			r := roles.NewJSON()
			err = r.Add(ri)
			if err != nil {
				t.Fatal(err)
			}

			getRolesFn := func(_ context.Context) (*roles.JSON, error) {
				return &r, nil
			}

			updateRolesFn := func(_ context.Context, _ *roles.JSON) error {
				return errors.New("error")
			}

			fakeClient := fakeKube{
				GetConfiguredRolesFn: getRolesFn,
				UpdateRolesRn:        updateRolesFn,
			}

			return req, successfulValidator{}, fakeClient, errIsNotNil
		},
	}

	// run the tests
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			req, validator, kube, checkFn := tc(t)
			svc := role.NewService(kube, validator)
			_, err := svc.Update(context.Background(), req)
			checkFn(t, err)
		})
	}
}

type fakeKube struct {
	UpdateRolesRn        func(ctx context.Context, roles *roles.JSON) error
	GetConfiguredRolesFn func(ctx context.Context) (*roles.JSON, error)
}

func (k fakeKube) UpdateRoles(ctx context.Context, roles *roles.JSON) error {
	if k.UpdateRolesRn != nil {
		return k.UpdateRolesRn(ctx, roles)
	}
	return nil
}

func (k fakeKube) GetConfiguredRoles(ctx context.Context) (*roles.JSON, error) {
	if k.GetConfiguredRolesFn != nil {
		return k.GetConfiguredRolesFn(ctx)
	}
	r := roles.NewJSON()
	return &r, nil
}

type successfulValidator struct{}

func (v successfulValidator) Validate(_ context.Context, _ *roles.Instance) error {
	return nil
}

type failValidator struct{}

func (v failValidator) Validate(_ context.Context, _ *roles.Instance) error {
	return errors.New("error")
}
