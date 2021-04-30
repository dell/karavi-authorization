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

	// Creates a fake powerflex handler
	ts := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	oldGetPowerFlexEndpoint := GetPowerFlexEndpoint
	GetPowerFlexEndpoint = func(storageSystemDetails System) string {
		return ts.URL
	}
	defer func() { GetPowerFlexEndpoint = oldGetPowerFlexEndpoint }()

	tests := map[string]func(t *testing.T) (string, int){
		"success creating role with json file": func(*testing.T) (string, int) {
			return "--role=NewRole1=powerflex=542a2d5f5122210f=bronze=9000000", 0
		},
		"failure creating role with negative quota": func(*testing.T) (string, int) {
			return "--role=NewRole1=powerflex=542a2d5f5122210f=bronze=-2", 1
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			roleArg, wantCode := tc(t)
			cmd := rootCmd
			cmd.SetArgs([]string{"role", "create", roleArg})
			var (
				outBuf, errBuf bytes.Buffer
			)
			cmd.SetOut(&outBuf)
			cmd.SetErr(&errBuf)
			var gotCode int
			done := make(chan struct{})
			osExit = func(code int) {
				gotCode = code
				done <- struct{}{}
				select {}
			}
			defer func() { osExit = os.Exit }()

			var err error
			go func() {
				err = cmd.Execute()
				done <- struct{}{}
			}()
			<-done
			if err != nil {
				t.Fatal(err)
			}

			if gotCode != wantCode {
				t.Errorf("exitCode: got %v, want %v", gotCode, wantCode)
			}
		})
	}
}

func Test_Unit_RoleCreate_PowerMax(t *testing.T) {
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(
			context.Background(),
			os.Args[0],
			append([]string{
				"-test.run=TestK3sRoleSubprocessPowerMax",
				"--",
				name}, args...)...)
		cmd.Env = append(os.Environ(), "WANT_GO_TEST_SUBPROCESS=1")

		return cmd
	}
	defer func() {
		execCommandContext = exec.CommandContext
	}()

	// Creates a fake powermax handler
	ts := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/univmax/restapi/90/system/version":
				fmt.Fprintf(w, `{ "version": "V9.2.1.2"}`)
			case "/univmax/restapi/90/sloprovisioning/symmetrix/000197900714/srp/bronze":
				w.WriteHeader(http.StatusOK)
			default:
				t.Errorf("unhandled unisphere request path: %s", r.URL.Path)
			}
		}))
	defer ts.Close()

	oldGetPowerMaxEndpoint := GetPowerMaxEndpoint
	GetPowerMaxEndpoint = func(storageSystemDetails System) string {
		return ts.URL
	}
	defer func() { GetPowerMaxEndpoint = oldGetPowerMaxEndpoint }()

	tests := map[string]func(t *testing.T) (string, int){
		"success creating role with json file": func(*testing.T) (string, int) {
			return "--role=NewRole3=powermax=000197900714=bronze=9000000", 0
		},
		"failure creating role with negative quota": func(*testing.T) (string, int) {
			return "--role=NewRole4=powermax=000197900714=bronze=-2", 1
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			roleArg, wantCode := tc(t)
			cmd := rootCmd
			cmd.SetArgs([]string{"role", "create", roleArg})
			t.Logf("%+v", cmd)
			var (
				outBuf, errBuf bytes.Buffer
			)
			cmd.SetOut(&outBuf)
			cmd.SetErr(&errBuf)

			var gotCode int
			done := make(chan struct{})
			osExit = func(code int) {
				gotCode = code
				done <- struct{}{}
				select {}
			}
			defer func() { osExit = os.Exit }()

			var err error
			go func() {
				err = cmd.Execute()
				done <- struct{}{}
			}()
			<-done
			log.Println(string(outBuf.Bytes()))
			log.Println(string(errBuf.Bytes()))
			if err != nil {
				t.Fatal(err)
			}

			if gotCode != wantCode {
				t.Errorf("exitCode: got %v, want %v", gotCode, wantCode)
			}
		})
	}
}

// This test case is intended to run as a subprocess.
func TestK3sRoleSubprocessPowerMax(t *testing.T) {
	if v := os.Getenv("WANT_GO_TEST_SUBPROCESS"); v != "1" {
		t.Skip("not being run as a subprocess")
	}

	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = os.Args[i+1:]
			break
		}
	}
	defer os.Exit(0)

	returnFile := "testdata/common-configmap.json"
	// k3s commands may be for access roles using 'common' configmap or for storage using the 'karavi-storage-secret' secret
	for _, arg := range os.Args {
		if arg == "secret/karavi-storage-secret" {
			returnFile = "testdata/kubectl_get_secret_storage_powerflex_powermax.json"
		}
	}

	file, err := os.OpenFile("/tmp/test.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		//
	}
	fmt.Fprintf(file, "RETURN_FILE: %q\n", returnFile)
	// k3s kubectl [get,create,apply]
	switch os.Args[2] {
	case "get":
		b, err := ioutil.ReadFile(returnFile)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = io.Copy(os.Stdout, bytes.NewReader(b)); err != nil {
			t.Fatal(err)
		}
	}
}
