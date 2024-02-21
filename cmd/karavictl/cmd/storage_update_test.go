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
	"io"
	"karavi-authorization/cmd/karavictl/cmd/api"
	"karavi-authorization/cmd/karavictl/cmd/api/mocks"
	"net/url"
	"os"
	"testing"
)

func TestStorageUpdateHandler(t *testing.T) {
	afterFn := func() {
		CreateHTTPClient = createHTTPClient
		JSONOutput = jsonOutput
		osExit = os.Exit
		ReadAccessAdminToken = readAccessAdminToken
	}

	t.Run("it requests update of storage", func(t *testing.T) {
		defer afterFn()
		var gotCalled bool
		CreateHTTPClient = func(_ string, _ bool) (api.Client, error) {
			return &mocks.FakeClient{
				PatchFn: func(_ context.Context, _ string, _ map[string]string, _ url.Values, _ interface{}, _ interface{}) error {
					gotCalled = true
					return nil
				},
			}, nil
		}
		ReadAccessAdminToken = func(_ string) (string, string, error) {
			return "AUnumberTokenIsNotWorkingman", "AUnumberTokenIsNotWorkingman", nil
		}
		JSONOutput = func(_ io.Writer, _ interface{}) error {
			return nil
		}
		osExit = func(_ int) {
		}
		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetOutput(&gotOutput)
		cmd.SetArgs([]string{"storage", "update", "--type=powerflex", "--insecure", "--endpoint=https://10.0.0.1", "--system-id=542a2d5f5122210f", "--user=admin", "--password=test", "--array-insecure", "--admin-token", "admin.yaml", "--addr", "proxy.com"})
		cmd.Execute()

		if !gotCalled {
			t.Error("expected Update to be called, but it wasn't")
		}
	})
	t.Run("it requires a valid storage server connection", func(t *testing.T) {
		defer afterFn()
		CreateHTTPClient = func(_ string, _ bool) (api.Client, error) {
			return nil, errors.New("failed to update storage: test error")
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
		cmd.SetArgs([]string{"storage", "update", "--type=powerflex", "--insecure", "--endpoint=https://10.0.0.1", "--system-id=542a2d5f5122210f", "--user=admin", "--password=test", "--array-insecure", "--admin-token", "admin.yaml", "--addr", "proxy.com"})
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
		wantErrMsg := "failed to update storage: test error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
	t.Run("it handles server errors", func(t *testing.T) {
		defer afterFn()
		CreateHTTPClient = func(_ string, _ bool) (api.Client, error) {
			return nil, errors.New("failed to update storage: test error")
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

		rootCmd := NewRootCmd()
		rootCmd.SetErr(&gotOutput)
		rootCmd.SetArgs([]string{"storage", "update", "--type=powerflex", "--insecure", "--endpoint=https://10.0.0.1", "--system-id=542a2d5f5122210f", "--user=admin", "--password=test", "--array-insecure", "--admin-token", "admin.yaml", "--addr", "proxy.com"})

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
		wantErrMsg := "failed to update storage: test error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
}
