// Copyright © 2021-2022 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"karavi-authorization/cmd/karavictl/cmd/api"
	"karavi-authorization/cmd/karavictl/cmd/api/mocks"
	"karavi-authorization/internal/role-service/roles"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestRoleGetHandler(t *testing.T) {
	afterFn := func() {
		CreateHTTPClient = createHTTPClient
		JSONOutput = jsonOutput
		osExit = os.Exit
		ReadAccessAdminToken = readAccessAdminToken
	}

	t.Run("it requests get role", func(t *testing.T) {
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

		CreateHTTPClient = func(_ string, _ bool) (api.Client, error) {
			return &mocks.FakeClient{
				GetFn: func(_ context.Context, _ string, _ map[string]string, _ url.Values, resp interface{}) error {
					b64Content := base64.StdEncoding.EncodeToString([]byte(b))
					jsonStr := fmt.Sprintf(`{"role": "%s"}`, b64Content)
					err = json.Unmarshal([]byte(jsonStr), resp)
					if err != nil {
						t.Fatal(err)
					}
					return nil
				},
			}, nil
		}

		osExit = func(_ int) {
		}
		ReadAccessAdminToken = func(_ string) (string, string, error) {
			return "AUnumberTokenIsNotWorkingman", "AUnumberTokenIsNotWorkingman", nil
		}
		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetOutput(&gotOutput)
		cmd.SetArgs([]string{"role", "get", "--insecure", "--name", "test", "--admin-token", "admin.yaml", "--addr", "proxy.com"})
		cmd.Execute()

		got := strings.ReplaceAll(gotOutput.String(), "\n", "")
		got = strings.ReplaceAll(got, " ", "")

		want := `{"test":{"system_types":{"powerflex":{"system_ids":{"542a2d5f5122210f":{"pool_quotas":{"bronze":10}}}}}}}`
		if want != got {
			t.Errorf("want %s, got \n%s", want, got)
		}
	})

	t.Run("it requires a valid role server connection", func(t *testing.T) {
		defer afterFn()
		CreateHTTPClient = func(_ string, _ bool) (api.Client, error) {
			return nil, errors.New("failed to get role: test server error")
		}

		var gotCode int
		done := make(chan struct{})
		osExit = func(code int) {
			gotCode = code
			done <- struct{}{}
			done <- struct{}{} // we can't let this function return
		}
		ReadAccessAdminToken = func(_ string) (string, string, error) {
			return "AUnumberTokenIsNotWorkingman", "AUnumberTokenIsNotWorkingman", nil
		}
		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetErr(&gotOutput)
		cmd.SetArgs([]string{"role", "get", "--insecure", "--admin-token", "admin.yaml", "--addr", "proxy.com"})
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
		wantErrMsg := "failed to get role: test server error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})

	t.Run("it handles server errors", func(t *testing.T) {
		defer afterFn()
		CreateHTTPClient = func(_ string, _ bool) (api.Client, error) {
			return &mocks.FakeClient{
				GetFn: func(_ context.Context, _ string, _ map[string]string, _ url.Values, _ interface{}) error {
					return errors.New("failed to get role: test error")
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

		rootCmd := NewRootCmd()
		rootCmd.SetErr(&gotOutput)
		rootCmd.SetArgs([]string{"role", "get", "--insecure", "--admin-token", "admin.yaml", "--addr", "proxy.com"})

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
		wantErrMsg := "failed to get role: test error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
}
