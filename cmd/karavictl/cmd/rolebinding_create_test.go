// Copyright © 2021-2023 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"karavi-authorization/cmd/karavictl/cmd/api"
	"karavi-authorization/cmd/karavictl/cmd/api/mocks"
	"net/url"
	"os"
	"testing"
)

func TestRolebindingCreate(t *testing.T) {
	afterFn := func() {
		CreateHTTPClient = createHTTPClient
		JSONOutput = jsonOutput
		osExit = os.Exit
		ReadAccessAdminToken = readAccessAdminToken
	}

	t.Run("it creates a rolebinding", func(t *testing.T) {
		defer afterFn()
		var gotCalled bool
		CreateHTTPClient = func(_ string, _ bool) (api.Client, error) {
			return &mocks.FakeClient{
				PostFn: func(_ context.Context, _ string, _ map[string]string, _ url.Values, _, _ interface{}) error {
					gotCalled = true
					return nil
				},
			}, nil
		}
		ReadAccessAdminToken = func(_ string) (string, string, error) {
			return "AUnumberTokenIsNotWorkingman", "AUnumberTokenIsNotWorkingman", nil
		}
		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetOutput(&gotOutput)
		cmd.SetArgs([]string{"rolebinding", "create", "--tenant=MyTenant", "--role=CAMedium", "--admin-token", "admin.yaml", "--addr", "proxy.com"})
		cmd.Execute()

		if !gotCalled {
			t.Error("expected BindRole to be called, but it wasn't")
		}
	})
	t.Run("it requires a valid tenant server connection", func(t *testing.T) {
		defer afterFn()
		CreateHTTPClient = func(_ string, _ bool) (api.Client, error) {
			return nil, errors.New("test error")
		}
		ReadAccessAdminToken = func(_ string) (string, string, error) {
			return "AUnumberTokenIsNotWorkingman", "AUnumberTokenIsNotWorkingman", nil
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
		cmd.SetArgs([]string{"rolebinding", "create", "--tenant=MyTenant", "--role=CAMedium", "--admin-token", "admin.yaml", "--addr", "proxy.com"})
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
		CreateHTTPClient = func(_ string, _ bool) (api.Client, error) {
			return &mocks.FakeClient{
				PostFn: func(_ context.Context, _ string, _ map[string]string, _ url.Values, _, _ interface{}) error {
					return errors.New("test error")
				},
			}, nil
		}
		ReadAccessAdminToken = func(_ string) (string, string, error) {
			return "AUnumberTokenIsNotWorkingman", "AUnumberTokenIsNotWorkingman", nil
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
		cmd.SetArgs([]string{"rolebinding", "create", "--tenant=MyTenant", "--role=CAMedium", "--admin-token", "admin.yaml", "--addr", "proxy.com"})

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
}
