// Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package middleware

import (
	"context"
	mocks "karavi-authorization/internal/role-service/mocks"
	"karavi-authorization/pb"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestTelemetry(t *testing.T) {
	t.Run("CreateRole", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeRoleServiceServer{
			CreateRoleFn: func(ctx context.Context, ctr *pb.RoleCreateRequest) (*pb.RoleCreateResponse, error) {
				gotCalled = true
				return &pb.RoleCreateResponse{}, nil
			},
		}

		sut := NewRoleTelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.Create(context.Background(), &pb.RoleCreateRequest{
			Name:        "test-name",
			StorageType: "powerflex",
			SystemId:    "542a2d5f5122210f",
			Pool:        "test-pool",
			Quota:       "test-quota",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})

	t.Run("UpdateRole", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeRoleServiceServer{
			UpdateRoleFn: func(ctx context.Context, ctr *pb.RoleUpdateRequest) (*pb.RoleUpdateResponse, error) {
				gotCalled = true
				return &pb.RoleUpdateResponse{}, nil
			},
		}

		sut := NewRoleTelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.Update(context.Background(), &pb.RoleUpdateRequest{
			Name:        "test-name",
			StorageType: "powerflex",
			SystemId:    "542a2d5f5122210f",
			Pool:        "test-pool",
			Quota:       "test-quota",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})

	t.Run("GetRole", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeRoleServiceServer{
			GetRoleFn: func(ctx context.Context, ctr *pb.RoleGetRequest) (*pb.RoleGetResponse, error) {
				gotCalled = true
				return &pb.RoleGetResponse{}, nil
			},
		}

		sut := NewRoleTelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.Get(context.Background(), &pb.RoleGetRequest{
			Name: "test-name",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})

	t.Run("DeleteRole", func(t *testing.T) {
		var gotCalled bool
		next := &mocks.FakeRoleServiceServer{
			DeleteRoleFn: func(ctx context.Context, ctr *pb.RoleDeleteRequest) (*pb.RoleDeleteResponse, error) {
				gotCalled = true
				return &pb.RoleDeleteResponse{}, nil
			},
		}

		sut := NewRoleTelemetryMW(logrus.NewEntry(logrus.StandardLogger()), next)
		_, err := sut.Delete(context.Background(), &pb.RoleDeleteRequest{
			Name:        "test-name",
			StorageType: "powerflex",
			SystemId:    "542a2d5f5122210f",
			Pool:        "test-pool",
			Quota:       "test-quota",
		})
		if err != nil {
			t.Fatal(err)
		}
		if !gotCalled {
			t.Errorf("expected next service to be called")
		}
	})
}
