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
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
)

func TestStorageListCmd(t *testing.T) {
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(
			context.Background(),
			os.Args[0],
			append([]string{
				"-test.run=TestK3sSubprocessStorageList",
				"--",
				name}, args...)...)
		cmd.Env = append(os.Environ(), "WANT_GO_TEST_SUBPROCESS=1")

		return cmd
	}
	defer func() {
		execCommandContext = exec.CommandContext
	}()

	t.Run("list all storage", func(t *testing.T) {
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"storage", "list", "--type="})
		cmd.Run(cmd, nil)
	})

	t.Run("list powerflex storage", func(t *testing.T) {
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"storage", "list", "--type=powerflex"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.Run(cmd, nil)

		var sysType SystemType
		err := json.Unmarshal(out.Bytes(), &sysType)
		if err != nil {
			t.Fatal(err)
		}

		if len(sysType) != 1 {
			t.Errorf("expected one storage response, got %d", len(sysType))
		}

		if _, ok := sysType["542a2d5f5122210f"]; !ok {
			t.Errorf("expected powerflex id 542a2d5f5122210f, id does not exist")
		}
	})

	t.Run("list powermax storage", func(t *testing.T) {
		cmd := NewRootCmd()
		cmd.SetArgs([]string{"storage", "list", "--type=powermax"})
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.Run(cmd, nil)

		var sysType SystemType
		err := json.Unmarshal(out.Bytes(), &sysType)
		if err != nil {
			t.Fatal(err)
		}

		if len(sysType) != 1 {
			t.Errorf("expected one storage response, got %d", len(sysType))
		}

		if _, ok := sysType["000197900714"]; !ok {
			t.Errorf("expected powermax id 000197900714, id does not exist")
		}
	})
}

func TestK3sSubprocessStorageList(t *testing.T) {
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
