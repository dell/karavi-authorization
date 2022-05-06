// Copyright © 2021 Dell Inc., or its subsidiaries. All Rights Reserved.
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

package cmd

import (
	"context"
	"karavi-authorization/pb"

	"google.golang.org/grpc"
)

type fakeRoleServiceClient struct {
	pb.TenantServiceClient
	CreateRoleFn func(context.Context, *pb.RoleCreateRequest, ...grpc.CallOption) (*pb.RoleCreateResponse, error)
	DeleteRoleFn func(context.Context, *pb.RoleDeleteRequest, ...grpc.CallOption) (*pb.RoleDeleteResponse, error)
	//GetRoleFn    func(context.Context, *pb.GetTenantRequest, ...grpc.CallOption) (*pb.Tenant, error)
	//DeleteRoleFn func(context.Context, *pb.DeleteTenantRequest, ...grpc.CallOption) (*pb.DeleteTenantResponse, error)
	//ListRoleFn   func(context.Context, *pb.ListTenantRequest, ...grpc.CallOption) (*pb.ListTenantResponse, error)
}

func (f *fakeRoleServiceClient) Create(ctx context.Context, in *pb.RoleCreateRequest, opts ...grpc.CallOption) (*pb.RoleCreateResponse, error) {
	if f.CreateRoleFn != nil {
		return f.CreateRoleFn(ctx, in, opts...)
	}
	return &pb.RoleCreateResponse{}, nil
}

func (f *fakeRoleServiceClient) Delete(ctx context.Context, in *pb.RoleDeleteRequest, opts ...grpc.CallOption) (*pb.RoleDeleteResponse, error) {
	if f.CreateRoleFn != nil {
		return f.DeleteRoleFn(ctx, in, opts...)
	}
	return &pb.RoleDeleteResponse{}, nil
}

/*func (f *fakeRoleServiceClient) GetTenant(ctx context.Context, in *pb.GetTenantRequest, opts ...grpc.CallOption) (*pb.Tenant, error) {
	if f.GetTenantFn != nil {
		return f.GetTenantFn(ctx, in, opts...)
	}
	return &pb.Tenant{
		Name: "testname",
	}, nil
}

func (f *fakeRoleServiceClient) DeleteTenant(ctx context.Context, in *pb.DeleteTenantRequest, opts ...grpc.CallOption) (*pb.DeleteTenantResponse, error) {
	if f.DeleteTenantFn != nil {
		return f.DeleteTenantFn(ctx, in, opts...)
	}
	return &pb.DeleteTenantResponse{}, nil
}

func (f *fakeRoleServiceClient) ListTenant(ctx context.Context, in *pb.ListTenantRequest, opts ...grpc.CallOption) (*pb.ListTenantResponse, error) {
	if f.ListTenantFn != nil {
		return f.ListTenantFn(ctx, in, opts...)
	}
	return &pb.ListTenantResponse{}, nil
}

func (f *fakeRoleServiceClient) BindRole(ctx context.Context, in *pb.BindRoleRequest, opts ...grpc.CallOption) (*pb.BindRoleResponse, error) {
	if f.BindRoleFn != nil {
		return f.BindRoleFn(ctx, in, opts...)
	}
	return &pb.BindRoleResponse{}, nil
}

func (f *fakeRoleServiceClient) UnbindRole(ctx context.Context, in *pb.UnbindRoleRequest, opts ...grpc.CallOption) (*pb.UnbindRoleResponse, error) {
	if f.UnbindRoleFn != nil {
		return f.UnbindRoleFn(ctx, in, opts...)
	}
	return &pb.UnbindRoleResponse{}, nil
}*/
