package middleware

import (
	"context"
	"karavi-authorization/internal/tenantsvc/mocks"
	"karavi-authorization/pb"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestTelemetry(t *testing.T) {
	t.Run("CreateTenant", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeTenantServiceServer{
			CreateTenantFn: func(ctx context.Context, ctr *pb.CreateTenantRequest) (*pb.Tenant, error) {
				gotCalled = true
				return &pb.Tenant{}, nil
			},
		}

		sut := TelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.CreateTenant(context.Background(), &pb.CreateTenantRequest{
			Tenant: &pb.Tenant{
				Name:       "test",
				Approvesdc: true,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})

	t.Run("UpdateTenant", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeTenantServiceServer{
			UpdateTenantFn: func(ctx context.Context, ctr *pb.UpdateTenantRequest) (*pb.Tenant, error) {
				gotCalled = true
				return &pb.Tenant{}, nil
			},
		}

		sut := TelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.UpdateTenant(context.Background(), &pb.UpdateTenantRequest{
			TenantName: "test",
			Approvesdc: true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})

	t.Run("GetTenant", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeTenantServiceServer{
			GetTenantFn: func(ctx context.Context, ctr *pb.GetTenantRequest) (*pb.Tenant, error) {
				gotCalled = true
				return &pb.Tenant{}, nil
			},
		}

		sut := TelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.GetTenant(context.Background(), &pb.GetTenantRequest{
			Name: "test",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})

	t.Run("DeleteTenant", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeTenantServiceServer{
			DeleteTenantFn: func(ctx context.Context, ctr *pb.DeleteTenantRequest) (*pb.DeleteTenantResponse, error) {
				gotCalled = true
				return &pb.DeleteTenantResponse{}, nil
			},
		}

		sut := TelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.DeleteTenant(context.Background(), &pb.DeleteTenantRequest{
			Name: "test",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})

	t.Run("ListTenant", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeTenantServiceServer{
			ListTenantFn: func(ctx context.Context, ctr *pb.ListTenantRequest) (*pb.ListTenantResponse, error) {
				gotCalled = true
				return &pb.ListTenantResponse{}, nil
			},
		}

		sut := TelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.ListTenant(context.Background(), &pb.ListTenantRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})

	t.Run("BindRole", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeTenantServiceServer{
			BindRoleFn: func(ctx context.Context, ctr *pb.BindRoleRequest) (*pb.BindRoleResponse, error) {
				gotCalled = true
				return &pb.BindRoleResponse{}, nil
			},
		}

		sut := TelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.BindRole(context.Background(), &pb.BindRoleRequest{
			TenantName: "test",
			RoleName:   "role",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})

	t.Run("UnbindRole", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeTenantServiceServer{
			UnbindRoleFn: func(ctx context.Context, ctr *pb.UnbindRoleRequest) (*pb.UnbindRoleResponse, error) {
				gotCalled = true
				return &pb.UnbindRoleResponse{}, nil
			},
		}

		sut := TelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.UnbindRole(context.Background(), &pb.UnbindRoleRequest{
			TenantName: "test",
			RoleName:   "role",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})

	t.Run("GenerateToken", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeTenantServiceServer{
			GenerateTokenFn: func(ctx context.Context, ctr *pb.GenerateTokenRequest) (*pb.GenerateTokenResponse, error) {
				gotCalled = true
				return &pb.GenerateTokenResponse{}, nil
			},
		}

		sut := TelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.GenerateToken(context.Background(), &pb.GenerateTokenRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})

	t.Run("RefreshToken", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeTenantServiceServer{
			RefreshTokenFn: func(ctx context.Context, ctr *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
				gotCalled = true
				return &pb.RefreshTokenResponse{}, nil
			},
		}

		sut := TelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.RefreshToken(context.Background(), &pb.RefreshTokenRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})

	t.Run("RevokeTenant", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeTenantServiceServer{
			RevokeTenantFn: func(ctx context.Context, ctr *pb.RevokeTenantRequest) (*pb.RevokeTenantResponse, error) {
				gotCalled = true
				return &pb.RevokeTenantResponse{}, nil
			},
		}

		sut := TelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.RevokeTenant(context.Background(), &pb.RevokeTenantRequest{
			TenantName: "test",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})

	t.Run("CancelRevokeTenant", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeTenantServiceServer{
			CancelRevokeTenantFn: func(ctx context.Context, ctr *pb.CancelRevokeTenantRequest) (*pb.CancelRevokeTenantResponse, error) {
				gotCalled = true
				return &pb.CancelRevokeTenantResponse{}, nil
			},
		}

		sut := TelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.CancelRevokeTenant(context.Background(), &pb.CancelRevokeTenantRequest{
			TenantName: "test",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})
}
