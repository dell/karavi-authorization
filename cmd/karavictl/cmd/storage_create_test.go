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

	"github.com/spf13/cobra"
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

	t.Run("happy path", func(t *testing.T) {
		systemInstancesTestDataPath = "testdata/powerflex_api_types_System_instances_testing123.json"
		setDefaultFlags(t, storageCreateCmd)
		setFlag(t, storageCreateCmd, "endpoint", ts.URL)
		setFlag(t, storageCreateCmd, "system-id", "testing123")
		storageCreateCmd.Run(storageCreateCmd, nil)
	})

	t.Run("prevents duplicate system registration", func(t *testing.T) {
		systemInstancesTestDataPath = "testdata/powerflex_api_types_System_instances_542a2d5f5122210f.json"
		setDefaultFlags(t, storageCreateCmd)
		setFlag(t, storageCreateCmd, "endpoint", ts.URL)
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
		setFlag(t, storageCreateCmd, "endpoint", ts.URL)
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

func setDefaultFlags(t *testing.T, cmd *cobra.Command) {
	setFlag(t, storageCreateCmd, "type", "powerflex")
	setFlag(t, storageCreateCmd, "user", "admin")
	setFlag(t, storageCreateCmd, "pass", "password")
	setFlag(t, storageCreateCmd, "insecure", "true")
}

func setFlag(t *testing.T, cmd *cobra.Command, name, value string) {
	t.Helper()
	if err := cmd.Flags().Set(name, value); err != nil {
		t.Fatal(err)
	}
}
