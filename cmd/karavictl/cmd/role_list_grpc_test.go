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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/pb"
	"os"
	"strings"
	"testing"

	"google.golang.org/grpc"
)

func TestRoleListGrpc(t *testing.T) {
	afterFn := func() {
		CreateRoleServiceClient = createRoleServiceClient
		JSONOutput = jsonOutput
		osExit = os.Exit
	}

	t.Run("it requests creation of a role", func(t *testing.T) {
		defer afterFn()

		r := roles.NewJSON()
		r.Add(&roles.Instance{
			Quota: 10,
			RoleKey: roles.RoleKey{
				Name:       "test",
				SystemType: "powerflex",
				SystemID:   "542a2d5f5122210f",
				Pool:       "bronze",
			},
		})

		b, err := r.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}

		listRoleFn := func(context.Context, *pb.RoleListRequest, ...grpc.CallOption) (*pb.RoleListResponse, error) {
			return &pb.RoleListResponse{Roles: b}, nil
		}

		CreateRoleServiceClient = func(_ string, _ bool) (pb.RoleServiceClient, io.Closer, error) {
			return &fakeRoleServiceClient{ListRoleFn: listRoleFn}, ioutil.NopCloser(nil), nil
		}
		osExit = func(code int) {
		}

		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetOutput(&gotOutput)
		cmd.SetArgs([]string{"role", "list", "--addr", "https://role-service.com", "--insecure"})
		cmd.Execute()

		got := strings.ReplaceAll(gotOutput.String(), "\n", "")
		got = strings.ReplaceAll(got, " ", "")

		want := `{"test":{"system_types":{"powerflex":{"system_ids":{"542a2d5f5122210f":{"pool_quotas":{"bronze":"10kB"}}}}}}}`
		if want != got {
			t.Errorf("want %s, got \n%s", want, got)
		}
	})
	t.Run("it requires a valid role server connection", func(t *testing.T) {
		defer afterFn()
		CreateRoleServiceClient = func(_ string, _ bool) (pb.RoleServiceClient, io.Closer, error) {
			return nil, ioutil.NopCloser(nil), errors.New("test error")
		}
		var gotCode int
		done := make(chan struct{})
		osExit = func(code int) {
			gotCode = code
			done <- struct{}{}
			done <- struct{}{} // we can't let this function return
		}
		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetErr(&gotOutput)
		cmd.SetArgs([]string{"role", "list", "--addr", "https://role-service.com", "--insecure"})
		go cmd.Execute()
		<-done

		wantCode := 1
		if gotCode != wantCode {
			t.Errorf("got exit code %d, want %d", gotCode, wantCode)
		}
		var gotErr CommandError
		if err := json.NewDecoder(&gotOutput).Decode(&gotErr); err != nil {
			t.Fatal(err)
		}
		wantErrMsg := "test error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
	t.Run("it handles server errors", func(t *testing.T) {
		defer afterFn()
		CreateRoleServiceClient = func(_ string, _ bool) (pb.RoleServiceClient, io.Closer, error) {
			return &fakeRoleServiceClient{
				CreateRoleFn: func(_ context.Context, _ *pb.RoleCreateRequest, _ ...grpc.CallOption) (*pb.RoleCreateResponse, error) {
					return nil, errors.New("test error")
				},
				DeleteRoleFn: func(_ context.Context, _ *pb.RoleDeleteRequest, _ ...grpc.CallOption) (*pb.RoleDeleteResponse, error) {
					return nil, errors.New("test error")
				},
				ListRoleFn: func(_ context.Context, _ *pb.RoleListRequest, _ ...grpc.CallOption) (*pb.RoleListResponse, error) {
					return nil, errors.New("test error")
				},
			}, ioutil.NopCloser(nil), nil
		}
		var gotCode int
		done := make(chan struct{})
		osExit = func(code int) {
			gotCode = code
			done <- struct{}{}
			done <- struct{}{} // we can't let this function return
		}
		var gotOutput bytes.Buffer

		rootCmd := NewRootCmd()
		rootCmd.SetErr(&gotOutput)
		rootCmd.SetArgs([]string{"role", "list", "--addr", "https://role-service.com", "--insecure"})

		go rootCmd.Execute()
		<-done

		wantCode := 1
		if gotCode != wantCode {
			t.Errorf("got exit code %d, want %d", gotCode, wantCode)
		}
		var gotErr CommandError
		if err := json.NewDecoder(&gotOutput).Decode(&gotErr); err != nil {
			t.Fatal(err)
		}
		wantErrMsg := "test error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
}
