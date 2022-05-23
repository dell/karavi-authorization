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
	"io"
	"io/ioutil"
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

	t.Run("get powerflex storage", func(t *testing.T) {
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"storage", "get", "--type", "powerflex", "--system-id", "542a2d5f5122210f"})
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
		cmd.SetArgs([]string{"storage", "get", "--type", "powermax", "--system-id", "000197900714"})
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
		cmd.SetArgs([]string{"storage", "get", "--type", "", "--system-id", "000197900714"})
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
		cmd.SetArgs([]string{"storage", "get", "--type", "powerflex", "--system-id", ""})
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
