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
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
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
			case "/api/instances/StoragePool::674ad77200000000/relationships/Statistics":
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

	oldGetPowerFlexEndpoint := GetPowerFlexEndpoint
	GetPowerFlexEndpoint = func(storageSystemDetails System) string {
		return ts.URL
	}
	defer func() { GetPowerFlexEndpoint = oldGetPowerFlexEndpoint }()

	tests := map[string]func(t *testing.T) int{
		"success creating role with json file": func(*testing.T) int {
			return 4
		},
	}
	for name := range tests {
		t.Run(name, func(t *testing.T) {

			cmd := rootCmd
			cmd.SetArgs([]string{"role", "update", "-f", "testdata/test-role-update.json"})

			stdOut := bytes.NewBufferString("")
			cmd.SetOutput(stdOut)

			err := cmd.Execute()
			assert.Nil(t, err)
		})
	}
}
