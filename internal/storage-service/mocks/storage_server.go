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

package storage

import (
	"context"
	"karavi-authorization/pb"
)

// FakeStorageServiceServer is a mock storage service server
type FakeStorageServiceServer struct {
	pb.UnimplementedStorageServiceServer
	CreateStorageFn       func(context.Context, *pb.StorageCreateRequest) (*pb.StorageCreateResponse, error)
	ListStorageFn         func(context.Context, *pb.StorageListRequest) (*pb.StorageListResponse, error)
	UpdateStorageFn       func(context.Context, *pb.StorageUpdateRequest) (*pb.StorageUpdateResponse, error)
	DeleteStorageFn       func(context.Context, *pb.StorageDeleteRequest) (*pb.StorageDeleteResponse, error)
	GetStorageFn          func(context.Context, *pb.StorageGetRequest) (*pb.StorageGetResponse, error)
	GetPowerflexVolumesFn func(context.Context, *pb.GetPowerflexVolumesRequest) (*pb.GetPowerflexVolumesResponse, error)
}

// Create mocks Create for StorageServiceServer
func (f *FakeStorageServiceServer) Create(ctx context.Context, in *pb.StorageCreateRequest) (*pb.StorageCreateResponse, error) {
	if f.CreateStorageFn != nil {
		return f.CreateStorageFn(ctx, in)
	}
	return &pb.StorageCreateResponse{}, nil
}

// List mocks List for StorageServiceServer
func (f *FakeStorageServiceServer) List(ctx context.Context, in *pb.StorageListRequest) (*pb.StorageListResponse, error) {
	if f.ListStorageFn != nil {
		return f.ListStorageFn(ctx, in)
	}
	return &pb.StorageListResponse{}, nil
}

// Update mocks Update for StorageServiceServer
func (f *FakeStorageServiceServer) Update(ctx context.Context, in *pb.StorageUpdateRequest) (*pb.StorageUpdateResponse, error) {
	if f.UpdateStorageFn != nil {
		return f.UpdateStorageFn(ctx, in)
	}
	return &pb.StorageUpdateResponse{}, nil
}

// Delete mocks Delete for StorageServiceServer
func (f *FakeStorageServiceServer) Delete(ctx context.Context, in *pb.StorageDeleteRequest) (*pb.StorageDeleteResponse, error) {
	if f.DeleteStorageFn != nil {
		return f.DeleteStorageFn(ctx, in)
	}
	return &pb.StorageDeleteResponse{}, nil
}

// Get mocks Get for StorageServiceServer
func (f *FakeStorageServiceServer) Get(ctx context.Context, in *pb.StorageGetRequest) (*pb.StorageGetResponse, error) {
	if f.GetStorageFn != nil {
		return f.GetStorageFn(ctx, in)
	}
	return &pb.StorageGetResponse{}, nil
}

// GetPowerflexVolumes mocks GetPowerflexVolumes for StorageServiceServer
func (f *FakeStorageServiceServer) GetPowerflexVolumes(ctx context.Context, in *pb.GetPowerflexVolumesRequest) (*pb.GetPowerflexVolumesResponse, error) {
	if f.GetPowerflexVolumesFn != nil {
		return f.GetPowerflexVolumesFn(ctx, in)
	}
	return &pb.GetPowerflexVolumesResponse{}, nil
}
