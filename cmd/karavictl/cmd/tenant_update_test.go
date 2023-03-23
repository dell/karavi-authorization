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

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"karavi-authorization/internal/tenantsvc/mocks"
	"karavi-authorization/pb"
	"os"
	"testing"

	"google.golang.org/grpc"
)

func TestTenantUpdate(t *testing.T) {
	afterFn := func() {
		CreateTenantServiceClient = createTenantServiceClient
		JSONOutput = jsonOutput
		osExit = os.Exit
	}

	t.Run("it requests updation of a tenant", func(t *testing.T) {
		defer afterFn()
		CreateTenantServiceClient = func(_ string, _ bool) (pb.TenantServiceClient, io.Closer, error) {
			return &mocks.FakeTenantServiceClient{}, ioutil.NopCloser(nil), nil
		}
		JSONOutput = func(w io.Writer, _ interface{}) error {
			return nil
		}
		osExit = func(code int) {
		}
		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetOutput(&gotOutput)
		cmd.SetArgs([]string{"tenant", "update", "-n", "testname", "--approvesdc", "true"})
		cmd.Execute()

		if len(gotOutput.Bytes()) != 0 {
			t.Errorf("expected zero output but got %q", string(gotOutput.Bytes()))
		}
	})
	t.Run("it requires a valid tenant server connection", func(t *testing.T) {
		defer afterFn()
		CreateTenantServiceClient = func(_ string, _ bool) (pb.TenantServiceClient, io.Closer, error) {
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
		cmd.SetArgs([]string{"tenant", "update", "-n", "testname", "--approvesdc", "true"})
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
	t.Run("it requires a valid name argument", func(t *testing.T) {
		defer afterFn()
		CreateTenantServiceClient = func(_ string, _ bool) (pb.TenantServiceClient, io.Closer, error) {
			return &mocks.FakeTenantServiceClient{}, ioutil.NopCloser(nil), nil
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
		rootCmd.SetArgs([]string{"tenant", "update"})

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
		wantErrMsg := "empty name not allowed"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
	t.Run("it handles server errors", func(t *testing.T) {
		defer afterFn()
		CreateTenantServiceClient = func(_ string, _ bool) (pb.TenantServiceClient, io.Closer, error) {
			return &mocks.FakeTenantServiceClient{
				UpdateTenantFn: func(_ context.Context, _ *pb.UpdateTenantRequest, _ ...grpc.CallOption) (*pb.Tenant, error) {
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
		rootCmd.SetArgs([]string{"tenant", "update", "-n", "test", "--approvesdc", "true"})

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
