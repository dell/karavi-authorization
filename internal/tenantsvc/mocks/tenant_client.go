// Copyright © 2023 Dell Inc., or its subsidiaries. All Rights Reserved.
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

package mocks

import (
	"context"
	"karavi-authorization/pb"

	"google.golang.org/grpc"
)

// FakeTenantServiceClient is a mock tenant service client
type FakeTenantServiceClient struct {
	pb.TenantServiceClient
	CreateTenantFn       func(context.Context, *pb.CreateTenantRequest, ...grpc.CallOption) (*pb.Tenant, error)
	UpdateTenantFn       func(context.Context, *pb.UpdateTenantRequest, ...grpc.CallOption) (*pb.Tenant, error)
	GetTenantFn          func(context.Context, *pb.GetTenantRequest, ...grpc.CallOption) (*pb.Tenant, error)
	DeleteTenantFn       func(context.Context, *pb.DeleteTenantRequest, ...grpc.CallOption) (*pb.DeleteTenantResponse, error)
	ListTenantFn         func(context.Context, *pb.ListTenantRequest, ...grpc.CallOption) (*pb.ListTenantResponse, error)
	BindRoleFn           func(context.Context, *pb.BindRoleRequest, ...grpc.CallOption) (*pb.BindRoleResponse, error)
	UnbindRoleFn         func(context.Context, *pb.UnbindRoleRequest, ...grpc.CallOption) (*pb.UnbindRoleResponse, error)
	GenerateTokenFn      func(context.Context, *pb.GenerateTokenRequest, ...grpc.CallOption) (*pb.GenerateTokenResponse, error)
	RevokeTenantFn       func(context.Context, *pb.RevokeTenantRequest, ...grpc.CallOption) (*pb.RevokeTenantResponse, error)
	CancelRevokeTenantFn func(context.Context, *pb.CancelRevokeTenantRequest, ...grpc.CallOption) (*pb.CancelRevokeTenantResponse, error)
}

// CreateTenant executes the mock CreateTenant
func (f *FakeTenantServiceClient) CreateTenant(ctx context.Context, in *pb.CreateTenantRequest, opts ...grpc.CallOption) (*pb.Tenant, error) {
	if f.CreateTenantFn != nil {
		return f.CreateTenantFn(ctx, in, opts...)
	}
	return &pb.Tenant{
		Name: "testname",
	}, nil
}

// UpdateTenant executes the mock UpdateTenant
func (f *FakeTenantServiceClient) UpdateTenant(ctx context.Context, in *pb.UpdateTenantRequest, opts ...grpc.CallOption) (*pb.Tenant, error) {
	if f.UpdateTenantFn != nil {
		return f.UpdateTenantFn(ctx, in, opts...)
	}
	return &pb.Tenant{
		Name: "testname",
	}, nil
}

// GetTenant executes the mock GetTenant
func (f *FakeTenantServiceClient) GetTenant(ctx context.Context, in *pb.GetTenantRequest, opts ...grpc.CallOption) (*pb.Tenant, error) {
	if f.GetTenantFn != nil {
		return f.GetTenantFn(ctx, in, opts...)
	}
	return &pb.Tenant{
		Name: "testname",
	}, nil
}

// DeleteTenant executes the mock DeleteTenant
func (f *FakeTenantServiceClient) DeleteTenant(ctx context.Context, in *pb.DeleteTenantRequest, opts ...grpc.CallOption) (*pb.DeleteTenantResponse, error) {
	if f.DeleteTenantFn != nil {
		return f.DeleteTenantFn(ctx, in, opts...)
	}
	return &pb.DeleteTenantResponse{}, nil
}

// ListTenant executes the mock ListTenant
func (f *FakeTenantServiceClient) ListTenant(ctx context.Context, in *pb.ListTenantRequest, opts ...grpc.CallOption) (*pb.ListTenantResponse, error) {
	if f.ListTenantFn != nil {
		return f.ListTenantFn(ctx, in, opts...)
	}
	return &pb.ListTenantResponse{}, nil
}

// BindRole executes the mock BindRole
func (f *FakeTenantServiceClient) BindRole(ctx context.Context, in *pb.BindRoleRequest, opts ...grpc.CallOption) (*pb.BindRoleResponse, error) {
	if f.BindRoleFn != nil {
		return f.BindRoleFn(ctx, in, opts...)
	}
	return &pb.BindRoleResponse{}, nil
}

// UnbindRole executes the mock UnbindRole
func (f *FakeTenantServiceClient) UnbindRole(ctx context.Context, in *pb.UnbindRoleRequest, opts ...grpc.CallOption) (*pb.UnbindRoleResponse, error) {
	if f.UnbindRoleFn != nil {
		return f.UnbindRoleFn(ctx, in, opts...)
	}
	return &pb.UnbindRoleResponse{}, nil
}

// GenerateToken executes the mock GenerateToken
func (f *FakeTenantServiceClient) GenerateToken(ctx context.Context, in *pb.GenerateTokenRequest, opts ...grpc.CallOption) (*pb.GenerateTokenResponse, error) {
	if f.GenerateTokenFn != nil {
		return f.GenerateTokenFn(ctx, in, opts...)
	}
	return &pb.GenerateTokenResponse{}, nil
}

// RevokeTenant executes the mock RevokeTenant
func (f *FakeTenantServiceClient) RevokeTenant(ctx context.Context, in *pb.RevokeTenantRequest, opts ...grpc.CallOption) (*pb.RevokeTenantResponse, error) {
	if f.RevokeTenantFn != nil {
		return f.RevokeTenantFn(ctx, in, opts...)
	}
	return &pb.RevokeTenantResponse{}, nil
}

// CancelRevokeTenant executes the mock CancelRevokeTenant
func (f *FakeTenantServiceClient) CancelRevokeTenant(ctx context.Context, in *pb.CancelRevokeTenantRequest, opts ...grpc.CallOption) (*pb.CancelRevokeTenantResponse, error) {
	if f.CancelRevokeTenantFn != nil {
		return f.CancelRevokeTenantFn(ctx, in, opts...)
	}
	return &pb.CancelRevokeTenantResponse{}, nil
}
