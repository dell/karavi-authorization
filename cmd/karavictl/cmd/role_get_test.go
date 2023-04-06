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
	"encoding/json"
	"errors"
	"fmt"
	"karavi-authorization/cmd/karavictl/cmd/api"
	"karavi-authorization/cmd/karavictl/cmd/api/mocks"
	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/pb"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Unit_RoleGet(t *testing.T) {
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(
			context.Background(),
			os.Args[0],
			append([]string{
				"-test.run=TestK3sRoleSubprocess",
				"--",
				name}, args...)...)
		cmd.Env = append(os.Environ(), "WANT_GO_TEST_SUBPROCESS=1")

		return cmd
	}
	defer func() {
		execCommandContext = exec.CommandContext
	}()
	ReadAccessAdminToken = func(afile string) (string, string, error) {
		return "AUnumberTokenIsNotWorkingman", "AUnumberTokenIsNotWorkingman", nil
	}
	tests := map[string]func(t *testing.T) ([]string, int){
		"success getting existing role": func(*testing.T) ([]string, int) {
			// --role=CSIGold not supported
			return []string{"CSIGold"}, 0
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			rolesToGet, wantCode := tc(t)

			cmd := NewRootCmd()

			args := []string{"--admin-token", "admin.yaml", "role", "get"}
			for _, role := range rolesToGet {
				args = append(args, fmt.Sprintf("--name=%s", role))
			}
			cmd.SetArgs(args)

			var gotCode int
			done := make(chan struct{})
			if wantCode == 1 {
				defer func() { osExit = os.Exit }()
				osExit = func(code int) {
					gotCode = code
					done <- struct{}{}
					done <- struct{}{}
				}

				go cmd.Execute()
				<-done
			} else {
				osExit = os.Exit
				cmd.Execute()
			}

			assert.Equal(t, wantCode, gotCode)
		})
	}
}

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

		CreateHTTPClient = func(addr string, insecure bool) (api.Client, error) {
			return &mocks.FakeClient{
				GetFn: func(ctx context.Context, path string, headers map[string]string, query url.Values, resp interface{}) error {
					resp = pb.RoleGetResponse{Role: b}
					return nil
				},
			}, nil
		}

		osExit = func(code int) {
		}
		ReadAccessAdminToken = func(afile string) (string, string, error) {
			return "AUnumberTokenIsNotWorkingman", "AUnumberTokenIsNotWorkingman", nil
		}
		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetOutput(&gotOutput)
		cmd.SetArgs([]string{"--admin-token", "admin.yaml", "role", "get", "--addr", "https://role-service.com", "--insecure", "--name", "test"})
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
		CreateHTTPClient = func(addr string, insecure bool) (api.Client, error) {
			return nil, errors.New("failed to get role: test server error")
		}

		var gotCode int
		done := make(chan struct{})
		osExit = func(code int) {
			gotCode = code
			done <- struct{}{}
			done <- struct{}{} // we can't let this function return
		}
		ReadAccessAdminToken = func(afile string) (string, string, error) {
			return "AUnumberTokenIsNotWorkingman", "AUnumberTokenIsNotWorkingman", nil
		}
		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetErr(&gotOutput)
		cmd.SetArgs([]string{"--admin-token", "admin.yaml", "role", "get", "--addr", "https://role-service.com", "--insecure"})
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
		CreateHTTPClient = func(addr string, insecure bool) (api.Client, error) {
			return &mocks.FakeClient{
				GetFn: func(ctx context.Context, path string, headers map[string]string, query url.Values, resp interface{}) error {
					return errors.New("failed to get role: test error")
				},
			}, nil
		}
		ReadAccessAdminToken = func(afile string) (string, string, error) {
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
		rootCmd.SetArgs([]string{"--admin-token", "admin.yaml", "role", "get", "--addr", "https://role-service.com", "--insecure"})

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
