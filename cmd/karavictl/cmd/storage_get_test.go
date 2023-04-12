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
	"io/ioutil"
	"karavi-authorization/cmd/karavictl/cmd/api"
	"karavi-authorization/cmd/karavictl/cmd/api/mocks"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestStorageGetCmd(t *testing.T) {
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(
			context.Background(),
			os.Args[0],
			append([]string{
				"-test.run=TestK3sSubprocessStorageGet",
				"--",
				name}, args...)...)
		cmd.Env = append(os.Environ(), "WANT_GO_TEST_SUBPROCESS=1")

		return cmd
	}
	defer func() {
		execCommandContext = exec.CommandContext
	}()

	ReadAccessAdminToken = func(afile string) (string, error) {
		return "AUnumberTokenIsNotWorkingman", nil
	}
	t.Run("get powerflex storage", func(t *testing.T) {
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"--admin_token", "admin.yaml", "storage", "get", "--type", "powerflex", "--system-id", "542a2d5f5122210f"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.Run(cmd, nil)

		var sys System
		err := json.Unmarshal(out.Bytes(), &sys)
		if err != nil {
			t.Fatal(err)
		}

		if &sys == nil {
			t.Errorf("expected non-nil powerflex system")
		}

	})

	t.Run("get powermax storage", func(t *testing.T) {
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"--admin_token", "admin.yaml", "storage", "get", "--type", "powermax", "--system-id", "000197900714"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.Run(cmd, nil)

		var sys System
		err := json.Unmarshal(out.Bytes(), &sys)
		if err != nil {
			t.Fatal(err)
		}

		if &sys == nil {
			t.Errorf("expected non-nil powermax system")
		}
	})

	t.Run("no storage type", func(t *testing.T) {
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"--admin_token", "admin.yaml", "storage", "get", "--type", "", "--system-id", "000197900714"})
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
		wantToContain := "system type not specified"
		if !strings.Contains(string(out.Bytes()), wantToContain) {
			t.Errorf("expected output to contain %q", wantToContain)
		}
	})

	t.Run("no storage id", func(t *testing.T) {
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"--admin_token", "admin.yaml", "storage", "get", "--type", "powerflex", "--system-id", ""})
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
		wantToContain := "system id not specified"
		if !strings.Contains(string(out.Bytes()), wantToContain) {
			t.Errorf("expected output to contain %q", wantToContain)
		}
	})
}

func TestK3sSubprocessStorageGet(t *testing.T) {
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
		b, err := ioutil.ReadFile("testdata/kubectl_get_secret_storage_powerflex_powermax.json")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = io.Copy(os.Stdout, bytes.NewReader(b)); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStorageGetHandler(t *testing.T) {
	afterFn := func() {
		CreateHTTPClient = createHTTPClient
		JSONOutput = jsonOutput
		osExit = os.Exit
		ReadAccessAdminToken = readAccessAdminToken
	}

	t.Run("it requests getting a storage", func(t *testing.T) {
		defer afterFn()
		var gotCalled bool
		CreateHTTPClient = func(addr string, insecure bool) (api.Client, error) {
			return &mocks.FakeClient{
				GetFn: func(ctx context.Context, path string, headers map[string]string, query url.Values, resp interface{}) error {
					gotCalled = true
					storage := `{"powerflex":{"11e4e7d35817bd0f":{"User":"admin","Password":"test","Endpoint":"https://10.0.0.1","Insecure":false}}}`
					err := json.Unmarshal([]byte(storage), resp)
					if err != nil {
						t.Fatal(err)
					}
					return nil
				},
			}, nil
		}
		ReadAccessAdminToken = func(afile string) (string, error) {
			return "AUnumberTokenIsNotWorkingman", nil
		}

		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetOutput(&gotOutput)
		cmd.SetArgs([]string{"--admin_token", "admin.yaml", "storage", "get", "--addr", "https://storage-service.com", "--system-id", "11e4e7d35817bd0f", "--type", "powerflex", "--insecure"})
		cmd.Execute()

		if !gotCalled {
			t.Error("expected Get to be called, but it wasn't")
		}
	})
	t.Run("it requires a valid storage server connection", func(t *testing.T) {
		defer afterFn()
		CreateHTTPClient = func(addr string, insecure bool) (api.Client, error) {
			return nil, errors.New("failed to get storage: test error")
		}
		ReadAccessAdminToken = func(afile string) (string, error) {
			return "AUnumberTokenIsNotWorkingman", nil
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
		cmd.SetArgs([]string{"--admin_token", "admin.yaml", "storage", "get", "--addr", "https://storage-service.com", "--system-id", "11e4e7d35817bd0f", "--type", "powerflex", "--insecure"})
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
		wantErrMsg := "failed to get storage: test error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
	t.Run("it handles server errors", func(t *testing.T) {
		defer afterFn()
		CreateHTTPClient = func(addr string, insecure bool) (api.Client, error) {
			return nil, errors.New("failed to get storage: test error")
		}
		ReadAccessAdminToken = func(afile string) (string, error) {
			return "AUnumberTokenIsNotWorkingman", nil
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
		rootCmd.SetArgs([]string{"--admin_token", "admin.yaml", "storage", "get", "--addr", "https://storage-service.com", "--system-id", "testing123", "--type", "powerflex", "--insecure"})

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
		wantErrMsg := "failed to get storage: test error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
}
