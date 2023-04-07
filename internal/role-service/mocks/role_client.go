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

// FakeRoleServiceClient is a mock Role service client
type FakeRoleServiceClient struct {
	pb.RoleServiceClient
	CreateRoleFn func(context.Context, *pb.RoleCreateRequest, ...grpc.CallOption) (*pb.RoleCreateResponse, error)
	UpdateRoleFn func(context.Context, *pb.RoleUpdateRequest, ...grpc.CallOption) (*pb.RoleUpdateResponse, error)
	GetRoleFn    func(context.Context, *pb.RoleGetRequest, ...grpc.CallOption) (*pb.RoleGetResponse, error)
	DeleteRoleFn func(context.Context, *pb.RoleDeleteRequest, ...grpc.CallOption) (*pb.RoleDeleteResponse, error)
}

// CreateRole executes the mock CreateRole
func (f *FakeRoleServiceClient) CreateRole(ctx context.Context, in *pb.RoleCreateRequest, opts ...grpc.CallOption) (*pb.RoleCreateResponse, error) {
	if f.CreateRoleFn != nil {
		return f.CreateRoleFn(ctx, in, opts...)
	}
	return &pb.RoleCreateResponse{}, nil
}

// UpdateRole executes the mock UpdateRole
func (f *FakeRoleServiceClient) UpdateRole(ctx context.Context, in *pb.RoleUpdateRequest, opts ...grpc.CallOption) (*pb.RoleUpdateResponse, error) {
	if f.UpdateRoleFn != nil {
		return f.UpdateRoleFn(ctx, in, opts...)
	}
	return &pb.RoleUpdateResponse{}, nil
}

// GetRole executes the mock GetRole
func (f *FakeRoleServiceClient) GetRole(ctx context.Context, in *pb.RoleGetRequest, opts ...grpc.CallOption) (*pb.RoleGetResponse, error) {
	if f.GetRoleFn != nil {
		return f.GetRoleFn(ctx, in, opts...)
	}
	return &pb.RoleGetResponse{}, nil
}

// DeleteRole executes the mock DeleteRole
func (f *FakeRoleServiceClient) DeleteRole(ctx context.Context, in *pb.RoleDeleteRequest, opts ...grpc.CallOption) (*pb.RoleDeleteResponse, error) {
	if f.DeleteRoleFn != nil {
		return f.DeleteRoleFn(ctx, in, opts...)
	}
	return &pb.RoleDeleteResponse{}, nil
}
