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
	ListRoleFn   func(context.Context, *pb.RoleListRequest, ...grpc.CallOption) (*pb.RoleListResponse, error)
	DeleteRoleFn func(context.Context, *pb.RoleDeleteRequest, ...grpc.CallOption) (*pb.RoleDeleteResponse, error)
}

// Create executes the mock Create
func (f *FakeRoleServiceClient) Create(ctx context.Context, in *pb.RoleCreateRequest, opts ...grpc.CallOption) (*pb.RoleCreateResponse, error) {
	if f.CreateRoleFn != nil {
		return f.CreateRoleFn(ctx, in, opts...)
	}
	return &pb.RoleCreateResponse{}, nil
}

// Update executes the mock Update
func (f *FakeRoleServiceClient) Update(ctx context.Context, in *pb.RoleUpdateRequest, opts ...grpc.CallOption) (*pb.RoleUpdateResponse, error) {
	if f.UpdateRoleFn != nil {
		return f.UpdateRoleFn(ctx, in, opts...)
	}
	return &pb.RoleUpdateResponse{}, nil
}

// Get executes the mock Get
func (f *FakeRoleServiceClient) Get(ctx context.Context, in *pb.RoleGetRequest, opts ...grpc.CallOption) (*pb.RoleGetResponse, error) {
	if f.GetRoleFn != nil {
		return f.GetRoleFn(ctx, in, opts...)
	}
	return &pb.RoleGetResponse{}, nil
}

// List executes the mock List
func (f *FakeRoleServiceClient) List(ctx context.Context, in *pb.RoleListRequest, opts ...grpc.CallOption) (*pb.RoleListResponse, error) {
	if f.ListRoleFn != nil {
		return f.ListRoleFn(ctx, in, opts...)
	return &pb.RoleListResponse{}, nil
}

// Delete executes the mock Delete
func (f *FakeRoleServiceClient) Delete(ctx context.Context, in *pb.RoleDeleteRequest, opts ...grpc.CallOption) (*pb.RoleDeleteResponse, error) {
	if f.DeleteRoleFn != nil {
