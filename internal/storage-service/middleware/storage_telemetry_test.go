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
	"fmt"
	mocks "karavi-authorization/internal/storage-service/mocks"
	"karavi-authorization/pb"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestStorage(t *testing.T) {
	t.Run("Create test cases", func(t *testing.T) {
		t.Run("Create successful run", func(t *testing.T) {
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

		t.Run("Create invaild request", func(t *testing.T) {
			var isCalled bool
			next := &mocks.FakeStorageServiceServer{
				CreateStorageFn: func(ctx context.Context, ctr *pb.StorageCreateRequest) (*pb.StorageCreateResponse, error) {
					isCalled = true
					return nil, fmt.Errorf("error: system with ID %s does not exist", ctr.GetSystemId())
				},
			}

			sut := NewStorageTelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
			_, err := sut.Create(context.Background(), &pb.StorageCreateRequest{
				StorageType: "powerflex",
				Endpoint:    "0.0.0.0:443",
				UserName:    "test",
				Password:    "test",
				Insecure:    true,
			})
			if err == nil {
				t.Fatal("expected error message from invalid request")
			}
			if !isCalled {
				t.Errorf("expected next service to be called")
			}
		})
	})
	t.Run("Update test cases", func(t *testing.T) {
		t.Run("Update sucessful test run", func(t *testing.T) {
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

		t.Run("Update invaild request", func(t *testing.T) {
			var isCalled bool
			next := &mocks.FakeStorageServiceServer{
				UpdateStorageFn: func(ctx context.Context, ctr *pb.StorageUpdateRequest) (*pb.StorageUpdateResponse, error) {
					isCalled = true
					return nil, fmt.Errorf("error: system with ID %s does not exist", ctr.GetSystemId())
				},
			}

			sut := NewStorageTelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
			_, err := sut.Update(context.Background(), &pb.StorageUpdateRequest{
				StorageType: "powerflex",
				Endpoint:    "0.0.0.0:443",
				UserName:    "test",
				Password:    "test",
				Insecure:    true,
			})
			if err == nil {
				t.Fatal("expected error message from invalid request")
			}
			if !isCalled {
				t.Errorf("expected next service to be called")
			}
		})
	})
	t.Run("Get test cases", func(t *testing.T) {
		t.Run("Get successful run", func(t *testing.T) {
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
		t.Run("Get invaild request", func(t *testing.T) {
			var isCalled bool
			next := &mocks.FakeStorageServiceServer{
				GetStorageFn: func(ctx context.Context, ctr *pb.StorageGetRequest) (*pb.StorageGetResponse, error) {
					isCalled = true
					return nil, fmt.Errorf("error: system with ID %s does not exist", ctr.GetSystemId())
				},
			}

			sut := NewStorageTelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
			_, err := sut.Get(context.Background(), &pb.StorageGetRequest{
				StorageType: "powerflex",
			})
			if err == nil {
				t.Fatal("expected error message from invalid request")
			}
			if !isCalled {
				t.Errorf("expected next service to be called")
			}
		})

	})
	t.Run("Delete test cases", func(t *testing.T) {
		t.Run("Delete sucssessful run", func(t *testing.T) {
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
		t.Run("Delete invaild request", func(t *testing.T) {
			var isCalled bool
			next := &mocks.FakeStorageServiceServer{
				DeleteStorageFn: func(ctx context.Context, ctr *pb.StorageDeleteRequest) (*pb.StorageDeleteResponse, error) {
					isCalled = true
					return nil, fmt.Errorf("error: system with ID %s does not exist", ctr.GetSystemId())
				},
			}

			sut := NewStorageTelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
			_, err := sut.Delete(context.Background(), &pb.StorageDeleteRequest{
				StorageType: "powerflex",
			})
			if err == nil {
				t.Fatal("expected error message from invalid request")
			}
			if !isCalled {
				t.Errorf("expected next service to be called")
			}
		})
	})
	t.Run("List test cases", func(t *testing.T) {
		t.Run("List successful run", func(t *testing.T) {
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
		t.Run("list invaild request", func(t *testing.T) {
			var isCalled bool
			next := &mocks.FakeStorageServiceServer{
				ListStorageFn: func(ctx context.Context, ctr *pb.StorageListRequest) (*pb.StorageListResponse, error) {
					isCalled = true
					return nil, fmt.Errorf("Unable to unmarshal JSON")
				},
			}

			sut := NewStorageTelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
			_, err := sut.List(context.Background(), &pb.StorageListRequest{})
			if err == nil {
				t.Fatal("expected error message from invalid JSON")
			}
			if !isCalled {
				t.Errorf("expected next service to be called")
			}
		})
	})

	t.Run("getPowerflexVolumes test cases", func(t *testing.T) {
		t.Run("getPowerflexVolumes successful run", func(t *testing.T) {
			var gotCalled bool
			next := &mocks.FakeStorageServiceServer{
				GetPowerflexVolumesFn: func(ctx context.Context, ctr *pb.GetPowerflexVolumesRequest) (*pb.GetPowerflexVolumesResponse, error) {
					gotCalled = true
					return &pb.GetPowerflexVolumesResponse{Volume: []*pb.Volume{{
						Name:     "k8s-6aac50817e",
						Size:     8,
						SystemId: "542a2d5f5122210f",
						Id:       "volumeId1",
						Pool:     "bronze",
					}}}, nil
				},
			}

			sut := NewStorageTelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
			_, err := sut.GetPowerflexVolumes(context.Background(), &pb.GetPowerflexVolumesRequest{
				VolumeName: []string{"k8s-6aac50817e"},
				SystemId:   "542a2d5f5122210f",
			})
			if err != nil {
				t.Fatal(err)
			}
			if !gotCalled {
				t.Errorf("expected next service to be called")
			}
		})
		t.Run("GetPowerflexVolumes invaild request", func(t *testing.T) {
			var isCalled bool
			next := &mocks.FakeStorageServiceServer{
				GetPowerflexVolumesFn: func(ctx context.Context, ctr *pb.GetPowerflexVolumesRequest) (*pb.GetPowerflexVolumesResponse, error) {
					isCalled = true
					return nil, fmt.Errorf("error: system with ID %s does not exist", ctr.GetSystemId())
				},
			}

			sut := NewStorageTelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
			_, err := sut.GetPowerflexVolumes(context.Background(), &pb.GetPowerflexVolumesRequest{
				VolumeName: []string{"k8s-6aac50817e"},
			})
			if err == nil {
				t.Fatal("expected error message from invalid request")
			}
			if !isCalled {
				t.Errorf("expected next service to be called")
			}
		})
	})

}
