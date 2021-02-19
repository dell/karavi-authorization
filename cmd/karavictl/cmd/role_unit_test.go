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
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Unit_RoleCreate(t *testing.T) {

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

	// Creates a fake powerflex handler with the ability
	// to control the response to api/types/System/instances.
	var systemInstancesTestDataPath string
	ts := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/login":
				fmt.Fprintf(w, `"token"`)
			case "/api/version":
				fmt.Fprintf(w, "3.5")
			case "/api/types/System/instances":
				b, err := ioutil.ReadFile(systemInstancesTestDataPath)
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
		"success creating role with json file": func(*testing.T) int {
			return 4
		},
	}
	for name := range tests {
		t.Run(name, func(t *testing.T) {

			cmd := rootCmd
			cmd.SetArgs([]string{"role", "create", "-f", "testdata/test-role-create.json"})

			stdOut := bytes.NewBufferString("")
			cmd.SetOutput(stdOut)

			err := cmd.Execute()
			assert.Nil(t, err)
		})
	}
}

func Test_Unit_RoleList(t *testing.T) {

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

	tests := map[string]func(t *testing.T) int{
		"success listing default role quotas": func(*testing.T) int {
			return 4
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			expectedRoleQuotas := tc(t)

			cmd := rootCmd
			cmd.SetArgs([]string{"role", "list"})

			stdOut := bytes.NewBufferString("")
			cmd.SetOutput(stdOut)

			err := cmd.Execute()
			assert.Nil(t, err)

			normalOut, err := ioutil.ReadAll(stdOut)
			assert.Nil(t, err)

			// read number of newlines from stdout of the command
			numberOfStdoutNewlines := len(strings.Split(strings.TrimSuffix(string(normalOut), "\n"), "\n"))
			// remove 2 header lines from stdout
			numberOfRoleQuotas := numberOfStdoutNewlines - 2
			assert.Equal(t, expectedRoleQuotas, numberOfRoleQuotas)
		})
	}
}

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

	tests := map[string]func(t *testing.T) ([]string, bool){
		"success getting existing role": func(*testing.T) ([]string, bool) {
			return []string{"CSIGold"}, false
		},
		"error getting role that doesn't exist": func(*testing.T) ([]string, bool) {
			return []string{"non-existing-role"}, true
		},
		"error passing no role to the command": func(*testing.T) ([]string, bool) {
			return []string{}, true
		},
		"error passing multiple roles to the command": func(*testing.T) ([]string, bool) {
			return []string{"role-1", "role-2"}, true
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			rolesToGet, expectError := tc(t)

			cmd := rootCmd

			args := []string{"role", "get"}
			for _, role := range rolesToGet {
				args = append(args, role)
			}
			cmd.SetArgs(args)

			stdOut := bytes.NewBufferString("")
			cmd.SetOutput(stdOut)

			err := cmd.Execute()

			if expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func Test_Unit_RoleDelete(t *testing.T) {

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

	tests := map[string]func(t *testing.T) ([]string, bool){
		"success deleting existing role": func(*testing.T) ([]string, bool) {
			return []string{"CSIGold"}, false
		},
		"error deleting role that doesn't exist": func(*testing.T) ([]string, bool) {
			return []string{"non-existing-role"}, true
		},
		"error passing no role to the command": func(*testing.T) ([]string, bool) {
			return []string{}, true
		},
		"error passing multiple roles to the command": func(*testing.T) ([]string, bool) {
			return []string{"role-1", "role-2"}, true
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			rolesToDelete, expectError := tc(t)

			cmd := rootCmd
			args := []string{"role", "delete"}
			for _, role := range rolesToDelete {
				args = append(args, role)
			}
			cmd.SetArgs(args)

			stdOut := bytes.NewBufferString("")
			cmd.SetOutput(stdOut)

			err := cmd.Execute()

			if expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
		})
	}
}
