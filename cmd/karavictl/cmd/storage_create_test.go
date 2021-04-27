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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/spf13/cobra"
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
			fmt.Printf("UNISPHERE REQUEST RECEIVED:%v\n", r.URL.Path)
			switch r.URL.Path {
			case "/univmax/restapi/90/system/version":
				fmt.Fprintf(w, `{ "version": "V9.2.1.2"}`)
			case "/univmax/restapi/90/system/symmetrix":
				b, err := ioutil.ReadFile(systemInstancesTestDataPath)
				if err != nil {
					t.Error(err)
					return
				}
				if _, err := io.Copy(w, bytes.NewReader(b)); err != nil {
					t.Error(err)
					return
				}
			case "/univmax/restapi/90/system/symmetrix/testing1":
				fmt.Fprintf(w, `{ "symmetrix": 
                [ {
                    "symmetrixId": "000000000001",
                    "device_count": 285.0,
                    "ucode": "5978.711.711",
                    "model": "PowerMax_2000",
                    "local": true 
                } ],
                "success": true }"`)
			case "/univmax/restapi/90/system/symmetrix/testing2":
				fmt.Fprintf(w, `{ "symmetrix": 
                [ {
                    "symmetrixId": "000000000002",
                    "device_count": 285.0,
                    "ucode": "5978.703.704",
                    "model": "PowerMax_2000",
                    "local": true 
                } ],
                "success": true }"`)
			case "/univmax/restapi/90/system/symmetrix/testing3":
				fmt.Fprintf(w, `{ "symmetrix": 
                [ {
                    "symmetrixId": "000000000003",
                    "device_count": 285.0,
                    "ucode": "5978.434.435",
                    "model": "PowerMax_2000",
                    "local": true 
                } ],
                "success": true }"`)
			case "/univmax/restapi/90/system/symmetrix/testing4":
				fmt.Fprintf(w, `{ "symmetrix": 
                [ {
                    "symmetrixId": "000000000003",
                    "device_count": 285.0,
                    "ucode": "5978.434.435",
                    "model": "VMAX250F",
                    "local": true 
                } ],
                "success": true }"`)
			case "/univmax/restapi/90/system/symmetrix/testing5":
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

	t.Run("happy path powerflex", func(t *testing.T) {
		systemInstancesTestDataPath = "testdata/powerflex_api_types_System_instances_testing123.json"
		setDefaultFlags(t, storageCreateCmd)
		setFlag(t, storageCreateCmd, "endpoint", pfts.URL)
		setFlag(t, storageCreateCmd, "system-id", "testing123")
		setFlag(t, storageCreateCmd, "type", "powerflex")
		storageCreateCmd.Run(storageCreateCmd, nil)
	})

	t.Run("happy path unisphere all", func(t *testing.T) {
		systemInstancesTestDataPath = "testdata/unisphere_api_types_System_instances_testing.json"
		setDefaultFlags(t, storageCreateCmd)
		setFlag(t, storageCreateCmd, "endpoint", usts.URL)
		setFlag(t, storageCreateCmd, "type", "powermax")
		storageCreateCmd.Run(storageCreateCmd, nil)
	})

	t.Run("happy path unisphere allowlist", func(t *testing.T) {
		systemInstancesTestDataPath = "testdata/unisphere_api_types_System_instances_testing.json"
		setDefaultFlags(t, storageCreateCmd)
		setFlag(t, storageCreateCmd, "endpoint", usts.URL)
		setFlag(t, storageCreateCmd, "system-id", "testing1,testing2")
		setFlag(t, storageCreateCmd, "type", "powermax")
		storageCreateCmd.Run(storageCreateCmd, nil)
	})

	t.Run("prevents duplicate system registration", func(t *testing.T) {
		systemInstancesTestDataPath = "testdata/powerflex_api_types_System_instances_542a2d5f5122210f.json"
		setDefaultFlags(t, storageCreateCmd)
		setFlag(t, storageCreateCmd, "endpoint", pfts.URL)
		setFlag(t, storageCreateCmd, "system-id", "542a2d5f5122210f")
		var out bytes.Buffer
		storageCreateCmd.SetErr(&out)

		done := make(chan struct{})
		wantExitCode := 1
		var gotExitCode int
		osExit = func(c int) {
			gotExitCode = c
			done <- struct{}{} // allows the test to progress
			done <- struct{}{} // stop this function returning
		}
		defer func() { osExit = os.Exit }()

		go storageCreateCmd.Run(storageCreateCmd, nil)
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
		setDefaultFlags(t, storageCreateCmd)
		setFlag(t, storageCreateCmd, "endpoint", pfts.URL)
		setFlag(t, storageCreateCmd, "system-id", "missing-system-id")
		var out bytes.Buffer
		storageCreateCmd.SetErr(&out)

		done := make(chan struct{})
		wantExitCode := 1
		var gotExitCode int
		osExit = func(c int) {
			gotExitCode = c
			done <- struct{}{} // allows the test to progress
			done <- struct{}{} // but stop this function returning
		}
		defer func() { osExit = os.Exit }()

		go storageCreateCmd.Run(storageCreateCmd, nil)
		<-done

		if gotExitCode != wantExitCode {
			t.Errorf("got exit code %d, want %d", gotExitCode, wantExitCode)
		}
		wantToContain := "not found"
		if !strings.Contains(string(out.Bytes()), wantToContain) {
			t.Errorf("expected output to contain %q", wantToContain)
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

func setDefaultFlags(t *testing.T, cmd *cobra.Command) {
	setFlag(t, storageCreateCmd, "type", "powerflex")
	setFlag(t, storageCreateCmd, "user", "admin")
	setFlag(t, storageCreateCmd, "password", "password")
	setFlag(t, storageCreateCmd, "insecure", "true")
}
