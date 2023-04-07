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
	"fmt"
	"io"
	"io/ioutil"
	"karavi-authorization/cmd/karavictl/cmd/api"
	"karavi-authorization/cmd/karavictl/cmd/api/mocks"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"

	"golang.org/x/term"
)

// This test case is intended to run as a subprocess.
func TestK3sSubprocess(t *testing.T) {
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

	// k3s kubectl [get,create,apply]
	switch os.Args[2] {
	case "get":
		b, err := ioutil.ReadFile("testdata/kubectl_get_secret_storage.json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = io.Copy(os.Stdout, bytes.NewReader(b)); err != nil {
			t.Fatal(err)
		}
	case "create":
		// exit code 0
	case "apply":
		// exit code 0
	}
}

func TestStorageCreateCmd(t *testing.T) {
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(
			context.Background(),
			os.Args[0],
			append([]string{
				"-test.run=TestK3sSubprocess",
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
	pfts := httptest.NewTLSServer(
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
				t.Errorf("unhandled powerflex request path: %s", r.URL.Path)
			}
		}))
	defer pfts.Close()

	// Creates a fake unisphere handler with the ability
	// to control the response to api/types/System/instances.
	usts := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/univmax/restapi/version":
				fmt.Fprintf(w, `{ "version": "V10.0.0.1"}`)
			case "/univmax/restapi/100/system/symmetrix":
				b, err := ioutil.ReadFile(systemInstancesTestDataPath)
				if err != nil {
					t.Error(err)
					return
				}
				if _, err := io.Copy(w, bytes.NewReader(b)); err != nil {
					t.Error(err)
					return
				}
			case "/univmax/restapi/100/system/symmetrix/testing1":
				fmt.Fprintf(w, `{ "symmetrix": 
                [ {
                    "symmetrixId": "000000000001",
                    "device_count": 285.0,
                    "ucode": "5978.711.711",
                    "model": "PowerMax_2000",
                    "local": true 
                } ],
                "success": true }"`)
			case "/univmax/restapi/100/system/symmetrix/testing2":
				fmt.Fprintf(w, `{ "symmetrix": 
                [ {
                    "symmetrixId": "000000000002",
                    "device_count": 285.0,
                    "ucode": "5978.703.704",
                    "model": "PowerMax_2000",
                    "local": true 
                } ],
                "success": true }"`)
			case "/univmax/restapi/100/system/symmetrix/testing3":
				fmt.Fprintf(w, `{ "symmetrix": 
                [ {
                    "symmetrixId": "000000000003",
                    "device_count": 285.0,
                    "ucode": "5978.434.435",
                    "model": "PowerMax_2000",
                    "local": true 
                } ],
                "success": true }"`)
			case "/univmax/restapi/100/system/symmetrix/testing4":
				fmt.Fprintf(w, `{ "symmetrix": 
                [ {
                    "symmetrixId": "000000000003",
                    "device_count": 285.0,
                    "ucode": "5978.434.435",
                    "model": "VMAX250F",
                    "local": true 
                } ],
                "success": true }"`)
			case "/univmax/restapi/100/system/symmetrix/testing5":
				fmt.Fprintf(w, `{ "symmetrix": 
                [ {
                    "symmetrixId": "000000000003",
                    "device_count": 285.0,
                    "ucode": "5978.434.435",
                    "model": "VMAX250F",
                    "local": true 
                } ],
                "success": true }"`)
			default:
				t.Errorf("unhandled unisphere request path: %s", r.URL.Path)
			}
		}))

	defer usts.Close()

	ofsts := httptest.NewTLSServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/platform/latest/":
				fmt.Fprintf(w, `{ "latest": "6"}`)
			case "/platform/3/cluster/config/":
				b, err := ioutil.ReadFile(systemInstancesTestDataPath)
				if err != nil {
					t.Error(err)
					return
				}
				if _, err := io.Copy(w, bytes.NewReader(b)); err != nil {
					t.Error(err)
					return
				}
			case "/session/1/session/":
				w.WriteHeader(http.StatusCreated)
			default:
				t.Errorf("unhandled onefs request path: %s", r.URL.Path)
			}
		}))

	defer ofsts.Close()

	t.Run("happy path powerflex", func(t *testing.T) {
		systemInstancesTestDataPath = "testdata/powerflex_api_types_System_instances_testing123.json"
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"storage", "create", "--endpoint", pfts.URL, "--system-id", "testing123", "--type", "powerflex", "--user", "admin", "--password", "password"})
		cmd.Run(cmd, nil)
	})

	t.Run("happy path unisphere all", func(t *testing.T) {
		systemInstancesTestDataPath = "testdata/unisphere_api_types_System_instances_testing.json"
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"storage", "create", "--endpoint", usts.URL, "--system-id", "", "--type", "powermax", "--user", "admin", "--password", "password"})
		cmd.Run(cmd, nil)
	})

	t.Run("happy path unisphere allowlist", func(t *testing.T) {
		systemInstancesTestDataPath = "testdata/unisphere_api_types_System_instances_testing.json"
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"storage", "create", "--endpoint", usts.URL, "--system-id", "testing1,testing2", "--type", "powermax", "--user", "admin", "--password", "password"})
		cmd.Run(cmd, nil)
	})

	t.Run("happy path onefs", func(t *testing.T) {
		systemInstancesTestDataPath = "testdata/onefs_api_types_System_instances_testing.json"
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"storage", "create", "--endpoint", ofsts.URL, "--system-id", "abcd1234", "--type", "powerscale", "--user", "admin", "--password", "password", "--insecure", "--array-insecure"})
		cmd.Run(cmd, nil)
	})

	t.Run("prevents duplicate system registration", func(t *testing.T) {
		systemInstancesTestDataPath = "testdata/powerflex_api_types_System_instances_542a2d5f5122210f.json"
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"storage", "create", "--endpoint", pfts.URL, "--system-id", "542a2d5f5122210f", "--type", "powerflex", "--user", "admin", "--password", "password"})
		var out bytes.Buffer
		cmd.SetErr(&out)

		done := make(chan struct{})
		wantExitCode := 1
		var gotExitCode int
		osExit = func(c int) {
			gotExitCode = c
			done <- struct{}{} // allows the test to progress
			done <- struct{}{} // stop this function returning
		}
		defer func() { osExit = os.Exit }()

		go cmd.Run(cmd, nil)
		<-done

		if gotExitCode != wantExitCode {
			t.Errorf("got exit code %d, want %d", gotExitCode, wantExitCode)
		}
		wantToContain := "is already registered"
		if !strings.Contains(string(out.Bytes()), wantToContain) {
			t.Errorf("expected output to contain %q", wantToContain)
		}
	})

	t.Run("system not found", func(t *testing.T) {
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"storage", "create", "--endpoint", pfts.URL, "--system-id", "missing-system-id", "--type", "powerflex", "--user", "admin", "--password", "password"})
		var out bytes.Buffer
		cmd.SetErr(&out)

		done := make(chan struct{})
		wantExitCode := 1
		var gotExitCode int
		osExit = func(c int) {
			gotExitCode = c
			done <- struct{}{} // allows the test to progress
			done <- struct{}{} // but stop this function returning
		}
		defer func() { osExit = os.Exit }()

		go cmd.Run(cmd, nil)
		<-done

		if gotExitCode != wantExitCode {
			t.Errorf("got exit code %d, want %d", gotExitCode, wantExitCode)
		}
		wantToContain := "not found"
		if !strings.Contains(string(out.Bytes()), wantToContain) {
			t.Errorf("expected output to contain %q\nactual output: %v", wantToContain, string(out.Bytes()))
		}
	})
}

func Test_readPassword(t *testing.T) {
	afterEach := func() {
		termReadPassword = term.ReadPassword
		osExit = os.Exit
	}
	t.Run("it prompts for a password", func(t *testing.T) {
		defer afterEach()
		termReadPassword = func(fd int) ([]byte, error) {
			return []byte("test"), nil
		}
		var (
			in bytes.Buffer
			v  string
		)
		prompt := "prompt: "

		readPassword(&in, prompt, &v)

		want := []byte(prompt + "\n")
		if got := in.Bytes(); !bytes.Equal(got, want) {
			t.Errorf("prompt: got %#v, want %#v", string(got), string(want))
		}
	})
	t.Run("it handles term failure", func(t *testing.T) {
		defer afterEach()
		termReadPassword = func(fd int) ([]byte, error) {
			return nil, errors.New("test error")
		}
		done := make(chan struct{})
		var statusCode int
		osExit = func(c int) {
			statusCode = c
			done <- struct{}{}
			done <- struct{}{} // stop this function returning
		}
		go func() {
			readPassword(ioutil.Discard, "prompt", new(string))
		}()
		<-done

		want := 1
		if got := statusCode; got != want {
			t.Errorf("statuscode: got %d, want %d", got, want)
		}
	})
}

func TestStorageCreateHandler(t *testing.T) {
	afterFn := func() {
		CreateHTTPClient = createHTTPClient
		JSONOutput = jsonOutput
		osExit = os.Exit
	}

	t.Run("it requests creation of a storage", func(t *testing.T) {
		defer afterFn()
		var gotCalled bool
		CreateHTTPClient = func(addr string, insecure bool) (api.Client, error) {
			return &mocks.FakeClient{
				PostFn: func(ctx context.Context, path string, headers map[string]string, query url.Values, body, resp interface{}) error {
					gotCalled = true
					return nil
				},
			}, nil
		}
		JSONOutput = func(w io.Writer, _ interface{}) error {
			return nil
		}
		osExit = func(code int) {
		}
		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetOutput(&gotOutput)
		cmd.SetArgs([]string{"storage", "create", "--addr", "https://storage-service.com", "--endpoint", "https://0.0.0.0:443", "--system-id", "testing123", "--type", "powerflex", "--user", "admin", "--password", "password", "--insecure", "--array-insecure"})
		cmd.Execute()

		if !gotCalled {
			t.Error("expected Create to be called, but it wasn't")
		}
	})
	t.Run("it requires a valid storage server connection", func(t *testing.T) {
		defer afterFn()
		CreateHTTPClient = func(addr string, insecure bool) (api.Client, error) {
			return nil, errors.New("failed to create storage: test error")
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
		cmd.SetArgs([]string{"storage", "create", "--addr", "https://storage-service.com", "--endpoint", "https://0.0.0.0:443", "--system-id", "testing123", "--type", "powerflex", "--user", "admin", "--password", "password", "--insecure", "--array-insecure"})
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
		wantErrMsg := "failed to create storage: test error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
	t.Run("it handles server errors", func(t *testing.T) {
		defer afterFn()
		CreateHTTPClient = func(addr string, insecure bool) (api.Client, error) {
			return nil, errors.New("failed to create storage: test error")
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
		rootCmd.SetArgs([]string{"storage", "create", "--addr", "https://storage-service.com", "--endpoint", "https://0.0.0.0:443", "--system-id", "testing123", "--type", "powerflex", "--user", "admin", "--password", "password", "--insecure", "--array-insecure"})

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
		wantErrMsg := "failed to create storage: test error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
}
