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
			r := &pb.RoleCreateRequest{
				Name:        "test",
				StorageType: "powerflex",
				SystemId:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "9GB",
			}
			return r, successfulValidator{}, successfulKube{}, errIsNil
		},
		"fail validation": func(t *testing.T) (*pb.RoleCreateRequest, role.Validator, role.Kube, checkFn) {
			r := &pb.RoleCreateRequest{
				Name:        "test",
				StorageType: "powerflex",
				SystemId:    "542a2d5f5122210f",
				Pool:        "bronze",
				Quota:       "-1",
			}
			return r, failValidator{}, successfulKube{}, errIsNotNil
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

type successfulKube struct{}

func (k successfulKube) UpdateRoles(ctx context.Context, roles *roles.JSON) error {
	return nil
}

func (k successfulKube) GetExistingRoles(ctx context.Context) (*roles.JSON, error) {
	return &roles.JSON{}, nil
}

type successfulValidator struct{}

func (v successfulValidator) Validate(ctx context.Context, role *roles.Instance) error {
	return nil
}

type failValidator struct{}

func (v failValidator) Validate(ctx context.Context, role *roles.Instance) error {
	return errors.New("error")
}
