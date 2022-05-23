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

type fakeStorageServiceClient struct {
	CreateStorageFn func(context.Context, *pb.StorageCreateRequest, ...grpc.CallOption) (*pb.StorageCreateResponse, error)
	ListStorageFn   func(context.Context, *pb.StorageListRequest, ...grpc.CallOption) (*pb.StorageListResponse, error)
	UpdateStorageFn func(context.Context, *pb.StorageUpdateRequest, ...grpc.CallOption) (*pb.StorageUpdateResponse, error)
}

func (f *fakeStorageServiceClient) Create(ctx context.Context, in *pb.StorageCreateRequest, opts ...grpc.CallOption) (*pb.StorageCreateResponse, error) {
	if f.CreateStorageFn != nil {
		return f.CreateStorageFn(ctx, in, opts...)
	}
	return &pb.StorageCreateResponse{}, nil
}

func (f *fakeStorageServiceClient) List(ctx context.Context, in *pb.StorageListRequest, opts ...grpc.CallOption) (*pb.StorageListResponse, error) {
	if f.ListStorageFn != nil {
		return f.ListStorageFn(ctx, in, opts...)
	}
	return &pb.StorageListResponse{}, nil
}

func (f *fakeStorageServiceClient) Update(ctx context.Context, in *pb.StorageUpdateRequest, opts ...grpc.CallOption) (*pb.StorageUpdateResponse, error) {
	if f.UpdateStorageFn != nil {
		return f.UpdateStorageFn(ctx, in, opts...)
	}
	return &pb.StorageUpdateResponse{}, nil
}
