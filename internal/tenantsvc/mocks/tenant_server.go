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

package mocks

import (
	"context"
	"karavi-authorization/pb"
)

type FakeTenantServiceServer struct {
	pb.UnimplementedTenantServiceServer
	CreateTenantFn       func(context.Context, *pb.CreateTenantRequest) (*pb.Tenant, error)
	UpdateTenantFn       func(context.Context, *pb.UpdateTenantRequest) (*pb.Tenant, error)
	GetTenantFn          func(context.Context, *pb.GetTenantRequest) (*pb.Tenant, error)
	DeleteTenantFn       func(context.Context, *pb.DeleteTenantRequest) (*pb.DeleteTenantResponse, error)
	ListTenantFn         func(context.Context, *pb.ListTenantRequest) (*pb.ListTenantResponse, error)
	BindRoleFn           func(context.Context, *pb.BindRoleRequest) (*pb.BindRoleResponse, error)
	UnbindRoleFn         func(context.Context, *pb.UnbindRoleRequest) (*pb.UnbindRoleResponse, error)
	GenerateTokenFn      func(context.Context, *pb.GenerateTokenRequest) (*pb.GenerateTokenResponse, error)
	RefreshTokenFn       func(context.Context, *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error)
	RevokeTenantFn       func(context.Context, *pb.RevokeTenantRequest) (*pb.RevokeTenantResponse, error)
	CancelRevokeTenantFn func(context.Context, *pb.CancelRevokeTenantRequest) (*pb.CancelRevokeTenantResponse, error)
}

func (f *FakeTenantServiceServer) CreateTenant(ctx context.Context, in *pb.CreateTenantRequest) (*pb.Tenant, error) {
	if f.CreateTenantFn != nil {
		return f.CreateTenantFn(ctx, in)
	}
	return &pb.Tenant{
		Name: "testname",
	}, nil
}

func (f *FakeTenantServiceServer) UpdateTenant(ctx context.Context, in *pb.UpdateTenantRequest) (*pb.Tenant, error) {
	if f.UpdateTenantFn != nil {
		return f.UpdateTenantFn(ctx, in)
	}
	return &pb.Tenant{
		Name: "testname",
	}, nil
}

func (f *FakeTenantServiceServer) GetTenant(ctx context.Context, in *pb.GetTenantRequest) (*pb.Tenant, error) {
	if f.GetTenantFn != nil {
		return f.GetTenantFn(ctx, in)
	}
	return &pb.Tenant{
		Name: "testname",
	}, nil
}

func (f *FakeTenantServiceServer) DeleteTenant(ctx context.Context, in *pb.DeleteTenantRequest) (*pb.DeleteTenantResponse, error) {
	if f.DeleteTenantFn != nil {
		return f.DeleteTenantFn(ctx, in)
	}
	return &pb.DeleteTenantResponse{}, nil
}

func (f *FakeTenantServiceServer) ListTenant(ctx context.Context, in *pb.ListTenantRequest) (*pb.ListTenantResponse, error) {
	if f.ListTenantFn != nil {
		return f.ListTenantFn(ctx, in)
	}
	return &pb.ListTenantResponse{}, nil
}

func (f *FakeTenantServiceServer) BindRole(ctx context.Context, in *pb.BindRoleRequest) (*pb.BindRoleResponse, error) {
	if f.BindRoleFn != nil {
		return f.BindRoleFn(ctx, in)
	}
	return &pb.BindRoleResponse{}, nil
}

func (f *FakeTenantServiceServer) UnbindRole(ctx context.Context, in *pb.UnbindRoleRequest) (*pb.UnbindRoleResponse, error) {
	if f.UnbindRoleFn != nil {
		return f.UnbindRoleFn(ctx, in)
	}
	return &pb.UnbindRoleResponse{}, nil
}

func (f *FakeTenantServiceServer) GenerateToken(ctx context.Context, in *pb.GenerateTokenRequest) (*pb.GenerateTokenResponse, error) {
	if f.GenerateTokenFn != nil {
		return f.GenerateTokenFn(ctx, in)
	}
	return &pb.GenerateTokenResponse{}, nil
}

func (f *FakeTenantServiceServer) RefreshToken(ctx context.Context, in *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	if f.RefreshTokenFn != nil {
		return f.RefreshTokenFn(ctx, in)
	}
	return &pb.RefreshTokenResponse{}, nil
}

func (f *FakeTenantServiceServer) RevokeTenant(ctx context.Context, in *pb.RevokeTenantRequest) (*pb.RevokeTenantResponse, error) {
	if f.RevokeTenantFn != nil {
		return f.RevokeTenantFn(ctx, in)
	}
	return &pb.RevokeTenantResponse{}, nil
}

func (f *FakeTenantServiceServer) CancelRevokeTenant(ctx context.Context, in *pb.CancelRevokeTenantRequest) (*pb.CancelRevokeTenantResponse, error) {
	if f.CancelRevokeTenantFn != nil {
		return f.CancelRevokeTenantFn(ctx, in)
	}
	return &pb.CancelRevokeTenantResponse{}, nil
}
