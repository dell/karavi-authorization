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
)

// FakeRoleServiceServer is a mock role service server
type FakeRoleServiceServer struct {
	pb.UnimplementedRoleServiceServer
	CreateRoleFn func(context.Context, *pb.RoleCreateRequest) (*pb.RoleCreateResponse, error)
	UpdateRoleFn func(context.Context, *pb.RoleUpdateRequest) (*pb.RoleUpdateResponse, error)
	GetRoleFn    func(context.Context, *pb.RoleGetRequest) (*pb.RoleGetResponse, error)
	DeleteRoleFn func(context.Context, *pb.RoleDeleteRequest) (*pb.RoleDeleteResponse, error)
}

// CreateRole handles the mock Create
func (f *FakeRoleServiceServer) Create(ctx context.Context, in *pb.RoleCreateRequest) (*pb.RoleCreateResponse, error) {
	if f.CreateRoleFn != nil {
		return f.CreateRoleFn(ctx, in)
	}
	return &pb.RoleCreateResponse{}, nil
}

// UpdateRole handles the mock Update
func (f *FakeRoleServiceServer) Update(ctx context.Context, in *pb.RoleUpdateRequest) (*pb.RoleUpdateResponse, error) {
	if f.UpdateRoleFn != nil {
		return f.UpdateRoleFn(ctx, in)
	}
	return &pb.RoleUpdateResponse{}, nil
}

// GetRole handles the mock Get
func (f *FakeRoleServiceServer) Get(ctx context.Context, in *pb.RoleGetRequest) (*pb.RoleGetResponse, error) {
	if f.GetRoleFn != nil {
		return f.GetRoleFn(ctx, in)
	}
	return &pb.RoleGetResponse{}, nil
}

// DeleteRole handles the mock Delete
func (f *FakeRoleServiceServer) Delete(ctx context.Context, in *pb.RoleDeleteRequest) (*pb.RoleDeleteResponse, error) {
	if f.DeleteRoleFn != nil {
		return f.DeleteRoleFn(ctx, in)
	}
	return &pb.RoleDeleteResponse{}, nil
}
