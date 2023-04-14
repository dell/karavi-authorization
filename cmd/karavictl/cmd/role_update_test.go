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
	"fmt"
	"io"
	"io/ioutil"
	"karavi-authorization/cmd/karavictl/cmd/api"
	"karavi-authorization/cmd/karavictl/cmd/api/mocks"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"testing"
)

func Test_Unit_RoleUpdate(t *testing.T) {

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
	// Creates a fake powerflex handler
	ts := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("PATH : %s\n", r.URL.Path)
			switch r.URL.Path {
			case "/api/login":
				fmt.Fprintf(w, `"token"`)
			case "/api/version":
				fmt.Fprintf(w, "3.5")
			case "/api/types/System/instances":
				b, err := ioutil.ReadFile("testdata/powerflex_api_types_System_instances_542a2d5f5122210f.json")
				if err != nil {
					t.Error(err)
					return
				}
				if _, err := io.Copy(w, bytes.NewReader(b)); err != nil {
					t.Error(err)
					return
				}
			case "/api/instances/System::542a2d5f5122210f/relationships/ProtectionDomain":
				b, err := ioutil.ReadFile("testdata/protection_domains.json")
				if err != nil {
					t.Error(err)
					return
				}
				if _, err := io.Copy(w, bytes.NewReader(b)); err != nil {
					t.Error(err)
					return
				}

			case "/api/instances/ProtectionDomain::0000000000000001/relationships/StoragePool":
				b, err := ioutil.ReadFile("testdata/storage_pools.json")
				if err != nil {
					t.Error(err)
					return
				}
				if _, err := io.Copy(w, bytes.NewReader(b)); err != nil {
					t.Error(err)
					return
				}
			case "/api/instances/StoragePool::7000000000000000/relationships/Statistics":
				b, err := ioutil.ReadFile("testdata/storage_pool_statistics.json")
				if err != nil {
					t.Error(err)
					return
				}
				if _, err := io.Copy(w, bytes.NewReader(b)); err != nil {
					t.Error(err)
					return
				}
			default:
				t.Errorf("unhandled request path: %s", r.URL.Path)
			}
		}))
	defer ts.Close()

	tests := map[string]func(t *testing.T) int{
		"updating an existing roles quota": func(*testing.T) int {
			return 4
		},
	}
	for name := range tests {
		t.Run(name, func(t *testing.T) {
			cmd := NewRootCmd()
			cmd.SetArgs([]string{"--admin-token", "admin.yaml", "role", "update",
				"--role=CSIBronze=powerflex=542a2d5f5122210f=bronze=9000000"})
			var (
				stdout bytes.Buffer
				stderr bytes.Buffer
			)
			cmd.SetOutput(&stdout)
			cmd.SetErr(&stderr)
			GetPowerFlexEndpoint = func(_ System) string {
				return ts.URL
			}
			done := make(chan struct{})
			var osExitCalled bool
			osExit = func(c int) {
				osExitCalled = true
				done <- struct{}{}
			}
			defer func() {
				osExit = os.Exit
			}()

			var err error
			go func() {
				err = cmd.Execute()
				done <- struct{}{}
			}()
			<-done
			if err != nil {
				t.Fatal(err)
			}

			if len(stdout.Bytes()) != 0 {
				t.Errorf("expected no response to mark success")
			}
			if osExitCalled {
				t.Errorf("osExitCalled: got %v, want %v", osExitCalled, false)
			}
		})
	}
}

func TestRoleUpdateHandler(t *testing.T) {
	afterFn := func() {
		CreateHTTPClient = createHTTPClient
		JSONOutput = jsonOutput
		osExit = os.Exit
		ReadAccessAdminToken = readAccessAdminToken
	}

	t.Run("it requests update of a role", func(t *testing.T) {
		defer afterFn()
		CreateHTTPClient = func(addr string, insecure bool) (api.Client, error) {
			return &mocks.FakeClient{
				PatchFn: func(ctx context.Context, path string, headers map[string]string, query url.Values, body, resp interface{}) error {
					return nil
				},
			}, nil
		}
		JSONOutput = func(w io.Writer, _ interface{}) error {
			return nil
		}
		osExit = func(code int) {
		}
		ReadAccessAdminToken = func(afile string) (string, string, error) {
			return "AUnumberTokenIsNotWorkingman", "AUnumberTokenIsNotWorkingman", nil
		}
		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetOutput(&gotOutput)
		cmd.SetArgs([]string{"--admin-token", "admin.yaml", "role", "update", "--addr", "https://role-service.com", "--insecure", "--role=bar=powerflex=11e4e7d35817bd0f=mypool=75GB"})
		cmd.Execute()

		if len(gotOutput.Bytes()) != 0 {
			t.Errorf("expected zero output but got %q", string(gotOutput.Bytes()))
		}
	})

	t.Run("it requires a valid role server connection", func(t *testing.T) {
		defer afterFn()
		CreateHTTPClient = func(addr string, insecure bool) (api.Client, error) {
			return nil, errors.New("failed to update role: test server error")
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
		cmd.SetArgs([]string{"--admin-token", "admin.yaml", "role", "update", "--addr", "https://role-service.com", "--insecure", "--role=bar=powerflex=11e4e7d35817bd0f=mypool=75GB"})
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
		wantErrMsg := "failed to update role: test server error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})

	t.Run("it handles server errors", func(t *testing.T) {
		defer afterFn()
		CreateHTTPClient = func(addr string, insecure bool) (api.Client, error) {
			return &mocks.FakeClient{
				PatchFn: func(ctx context.Context, path string, headers map[string]string, query url.Values, body, resp interface{}) error {
					return errors.New("failed to update role: test error")
				},
			}, nil
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

		rootCmd := NewRootCmd()
		rootCmd.SetErr(&gotOutput)
		rootCmd.SetArgs([]string{"--admin-token", "admin.yaml", "role", "update", "--addr", "https://role-service.com", "--insecure", "--role=bar=powerflex=11e4e7d35817bd0f=mypool=75GB"})

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
		wantErrMsg := "failed to update role: test error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
}
