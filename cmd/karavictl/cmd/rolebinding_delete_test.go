// Copyright © 2021 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"karavi-authorization/pb"
	"os"
	"testing"

	"google.golang.org/grpc"
)

func TestRolebindingDelete(t *testing.T) {
	afterFn := func() {
		CreateTenantServiceClient = createTenantServiceClient
		JSONOutput = jsonOutput
		osExit = os.Exit
	}

	t.Run("it deletes a rolebinding", func(t *testing.T) {
		defer afterFn()
		var gotCalled bool
		CreateTenantServiceClient = func(_ string) (pb.TenantServiceClient, io.Closer, error) {
			return &fakeTenantServiceClient{
				UnbindRoleFn: func(_ context.Context, _ *pb.UnbindRoleRequest, _ ...grpc.CallOption) (*pb.UnbindRoleResponse, error) {
					gotCalled = true
					return &pb.UnbindRoleResponse{}, nil
				},
			}, ioutil.NopCloser(nil), nil
		}
		var gotOutput bytes.Buffer

		rootCmd.SetOutput(&gotOutput)
		rootCmd.SetArgs([]string{"rolebinding", "delete"})
		rootCmd.Execute()

		if !gotCalled {
			t.Error("expected UnbindRole to be called, but it wasn't")
		}
	})
	t.Run("it requires a valid tenant server connection", func(t *testing.T) {
		defer afterFn()
		CreateTenantServiceClient = func(_ string) (pb.TenantServiceClient, io.Closer, error) {
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

		rootCmd.SetErr(&gotOutput)
		rootCmd.SetArgs([]string{"rolebinding", "delete"})
		go rootCmd.Execute()
		<-done

		wantCode := 1
		if gotCode != wantCode {
			t.Errorf("got exit code %d, want %d", gotCode, wantCode)
		}
		var gotErr CmdError
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
		CreateTenantServiceClient = func(_ string) (pb.TenantServiceClient, io.Closer, error) {
			return &fakeTenantServiceClient{
				UnbindRoleFn: func(_ context.Context, _ *pb.UnbindRoleRequest, _ ...grpc.CallOption) (*pb.UnbindRoleResponse, error) {
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

		deleteRoleBindingCmd.SetErr(&gotOutput)
		rootCmd.SetArgs([]string{"rolebinding", "delete"})

		go rootCmd.Execute()
		<-done

		wantCode := 1
		if gotCode != wantCode {
			t.Errorf("got exit code %d, want %d", gotCode, wantCode)
		}
		var gotErr CmdError
		if err := json.NewDecoder(&gotOutput).Decode(&gotErr); err != nil {
			t.Fatal(err)
		}
		wantErrMsg := "test error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
}
