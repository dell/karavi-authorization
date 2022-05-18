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
	GetRoleFn    func(context.Context, *pb.RoleGetRequest, ...grpc.CallOption) (*pb.RoleGetResponse, error)
	ListRoleFn   func(context.Context, *pb.RoleListRequest, ...grpc.CallOption) (*pb.RoleListResponse, error)
	UpdateRoleFn func(context.Context, *pb.RoleUpdateRequest, ...grpc.CallOption) (*pb.RoleUpdateResponse, error)
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

func (f *fakeRoleServiceClient) List(ctx context.Context, in *pb.RoleListRequest, opts ...grpc.CallOption) (*pb.RoleListResponse, error) {
	if f.ListRoleFn != nil {
		return f.ListRoleFn(ctx, in, opts...)
	}
	return &pb.RoleListResponse{}, nil
}

func (f *fakeRoleServiceClient) Get(ctx context.Context, in *pb.RoleGetRequest, opts ...grpc.CallOption) (*pb.RoleGetResponse, error) {
	if f.GetRoleFn != nil {
		return f.GetRoleFn(ctx, in, opts...)
	}
	return &pb.RoleGetResponse{}, nil
}

func (f *fakeRoleServiceClient) Update(ctx context.Context, in *pb.RoleUpdateRequest, opts ...grpc.CallOption) (*pb.RoleUpdateResponse, error) {
	if f.UpdateRoleFn != nil {
		return f.UpdateRoleFn(ctx, in, opts...)
	}
	return &pb.RoleUpdateResponse{}, nil
}
