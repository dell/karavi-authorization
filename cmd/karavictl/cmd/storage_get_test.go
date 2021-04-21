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
		setFlag(t, getCmd, "type", "powerflex")
		setFlag(t, getCmd, "system-id", "542a2d5f5122210f")
		var out bytes.Buffer
		getCmd.SetOut(&out)
		getCmd.Run(getCmd, nil)

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
		setFlag(t, getCmd, "type", "powermax")
		setFlag(t, getCmd, "system-id", "000197900714")
		var out bytes.Buffer
		getCmd.SetOut(&out)
		getCmd.Run(getCmd, nil)

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
		setFlag(t, getCmd, "type", "")
		setFlag(t, getCmd, "system-id", "000197900714")
		var out bytes.Buffer
		getCmd.SetErr(&out)

		done := make(chan struct{})
		wantExitCode := 1
		var gotExitCode int
		osExit = func(c int) {
			gotExitCode = c
			done <- struct{}{} // allows the test to progress
			done <- struct{}{} // stop this function returning
		}
		defer func() { osExit = os.Exit }()

		go getCmd.Run(getCmd, nil)
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
		setFlag(t, getCmd, "type", "powerflex")
		setFlag(t, getCmd, "system-id", "")
		var out bytes.Buffer
		getCmd.SetErr(&out)

		done := make(chan struct{})
		wantExitCode := 1
		var gotExitCode int
		osExit = func(c int) {
			gotExitCode = c
			done <- struct{}{} // allows the test to progress
			done <- struct{}{} // stop this function returning
		}
		defer func() { osExit = os.Exit }()

		go getCmd.Run(getCmd, nil)
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
