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
		"success": func(t *testing.T) (*pb.RoleCreateRequest, role.Validator, role.Kube, checkFn) {
			req := &pb.RoleCreateRequest{
				Name:        "test",
				StorageType: "powerflex",
				SystemId:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "9GB",
			}

			r := roles.NewJSON()
			return req, successfulValidator{}, successfulKube{roles: &r}, errIsNil
		},
		"fail validation": func(t *testing.T) (*pb.RoleCreateRequest, role.Validator, role.Kube, checkFn) {
			req := &pb.RoleCreateRequest{
				Name:        "test",
				StorageType: "powerflex",
				SystemId:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "-1",
			}

			r := roles.NewJSON()
			return req, failValidator{}, successfulKube{roles: &r}, errIsNotNil
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

			r := &pb.RoleDeleteRequest{
				Name:        "test",
				StorageType: "powerflex",
				SystemId:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "9GB",
			}

			return r, successfulKube{roles: &rff}, errIsNil
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

			r := &pb.RoleDeleteRequest{
				Name:        "notFound",
				StorageType: "powerflex",
				SystemId:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "9GB",
			}

			return r, successfulKube{roles: &rff}, errIsNotNil
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

type successfulKube struct {
	roles *roles.JSON
}

func (k successfulKube) UpdateRoles(ctx context.Context, roles *roles.JSON) error {
	return nil
}

func (k successfulKube) GetConfiguredRoles(ctx context.Context) (*roles.JSON, error) {
	return k.roles, nil
}

type successfulValidator struct{}

func (v successfulValidator) Validate(ctx context.Context, role *roles.Instance) error {
	return nil
}

type failValidator struct{}

func (v failValidator) Validate(ctx context.Context, role *roles.Instance) error {
	return errors.New("error")
}
