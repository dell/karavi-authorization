// Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package middleware

import (
	"context"
	mocks "karavi-authorization/internal/storage-service/mocks"
	"karavi-authorization/pb"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestStorage(t *testing.T) {
	t.Run("CreateStorage", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeStorageServiceServer{
			CreateStorageFn: func(ctx context.Context, ctr *pb.StorageCreateRequest) (*pb.StorageCreateResponse, error) {
				gotCalled = true
				return &pb.StorageCreateResponse{}, nil
			},
		}

		sut := NewStorageTelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.Create(context.Background(), &pb.StorageCreateRequest{
			StorageType: "powerflex",
			Endpoint:    "0.0.0.0:443",
			SystemId:    "542a2d5f5122210f",
			UserName:    "test",
			Password:    "test",
			Insecure:    true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})

	t.Run("UpdateStorage", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeStorageServiceServer{
			UpdateStorageFn: func(ctx context.Context, ctr *pb.StorageUpdateRequest) (*pb.StorageUpdateResponse, error) {
				gotCalled = true
				return &pb.StorageUpdateResponse{}, nil
			},
		}

		sut := NewStorageTelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.Update(context.Background(), &pb.StorageUpdateRequest{
			StorageType: "powerflex",
			Endpoint:    "0.0.0.0:443",
			SystemId:    "542a2d5f5122210f",
			UserName:    "test",
			Password:    "test",
			Insecure:    true,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})

	t.Run("GetStorage", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeStorageServiceServer{
			GetStorageFn: func(ctx context.Context, ctr *pb.StorageGetRequest) (*pb.StorageGetResponse, error) {
				gotCalled = true
				return &pb.StorageGetResponse{}, nil
			},
		}

		sut := NewStorageTelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.Get(context.Background(), &pb.StorageGetRequest{
			StorageType: "powerflex",
			SystemId:    "542a2d5f5122210f",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})

	t.Run("DeleteStorage", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeStorageServiceServer{
			DeleteStorageFn: func(ctx context.Context, ctr *pb.StorageDeleteRequest) (*pb.StorageDeleteResponse, error) {
				gotCalled = true
				return &pb.StorageDeleteResponse{}, nil
			},
		}

		sut := NewStorageTelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.Delete(context.Background(), &pb.StorageDeleteRequest{
			StorageType: "powerflex",
			SystemId:    "542a2d5f5122210f",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})

	t.Run("ListStorage", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeStorageServiceServer{
			ListStorageFn: func(ctx context.Context, ctr *pb.StorageListRequest) (*pb.StorageListResponse, error) {
				gotCalled = true
				return &pb.StorageListResponse{}, nil
			},
		}

		sut := NewStorageTelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.List(context.Background(), &pb.StorageListRequest{})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})
}
