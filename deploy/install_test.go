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

package main

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
)

var noopCreateDir = func(_ string) error {
	return nil
}

func init() {
	createDir = noopCreateDir
}

func TestRun(t *testing.T) {
	t.Run("happy path", func(t *testing.T) {
		dp := buildDeployProcess(nil, nil)
		dp.Steps = append(dp.Steps, func() {})

		err := run(dp)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("returns any error", func(t *testing.T) {
		dp := buildDeployProcess(nil, nil)
		wantErr := errors.New("test error")
		dp.Err = wantErr

		gotErr := run(dp)

		if gotErr != wantErr {
			t.Errorf("got err = %v, want %v", gotErr, wantErr)
		}
	})
}

func TestNewDeployProcess(t *testing.T) {
	got := NewDeploymentProcess(nil, nil, nil)

	if got == nil {
		t.Error("expected non-nil return value")
	}
}

func TestDeployProcess_CheckRootPermissions(t *testing.T) {
	var testOut, testErr bytes.Buffer
	sut := buildDeployProcess(&testOut, &testErr)
	afterEach := func() {
		osGeteuid = os.Geteuid
		testOut.Reset()
		testErr.Reset()
	}
	t.Run("it returns an error if not effectively ran as root", func(t *testing.T) {
		defer afterEach()
		osGeteuid = func() int {
			return 1000 // non-root.
		}

		sut.CheckRootPermissions()

		want := ErrNeedRoot
		if got := sut.Err; got != want {
			t.Errorf("got err = %v, want %v", got, want)
		}
	})
	t.Run("it determines the uid/gid when ran with sudo", func(t *testing.T) {
		defer afterEach()
		osGeteuid = func() int {
			return 0 // pretend to be effectively root.
		}
		tests := []struct {
			name         string
			givenSudoUID string
			givenSudoGID string
			expectUID    int
			expectGID    int
		}{
			{"only SUDO_UID set", "1000", "", 0, 0},
			{"only SUDO_GID set", "", "1000", 0, 0},
			{"neither set", "", "", 0, 0},
			{"both set with valid values", "1000", "1000", 1000, 1000},
			{"SUDO_UID is NaN", "NaN", "1000", 0, 0},
			{"SUDO_GID is NaN", "1000", "NaN", 0, 0},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				defer func() {
					osLookupEnv = os.LookupEnv
					sut.processOwnerUID = 0
					sut.processOwnerGID = 0
				}()

				osLookupEnv = func(env string) (string, bool) {
					switch env {
					case EnvSudoUID:
						if tt.givenSudoUID == "" {
							return "", false
						}
						return tt.givenSudoUID, true
					case EnvSudoGID:
						if tt.givenSudoGID == "" {
							return "", false
						}
						return tt.givenSudoGID, true
					default:
						return "", false
					}
				}

				sut.CheckRootPermissions()

				gotUID, gotGID := sut.processOwnerUID, sut.processOwnerGID
				wantUID, wantGID := tt.expectUID, tt.expectGID
				if gotUID != wantUID && gotGID != wantGID {
					t.Errorf("%s: got [%v,%v], want [%v,%v]", tt.name, gotUID, gotGID, wantUID, wantGID)
				}
			})
		}
	})
}

func TestDeployProcess_CreateTempWorkspace(t *testing.T) {
	afterEach := func() {
		ioutilTempDir = os.MkdirTemp
	}
	t.Run("it is a noop on sticky error", func(t *testing.T) {
		defer afterEach()
		var callCount int
		ioutilTempDir = func(_, _ string) (string, error) {
			callCount++
			return "", nil
		}
		sut := buildDeployProcess(nil, nil)
		sut.Err = errors.New("test error")

		sut.CreateTempWorkspace()

		want := 0
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %v", got, want)
		}
	})
	t.Run("it stores the created tmp dir", func(t *testing.T) {
		defer afterEach()
		want := "/tmp/testing"
		ioutilTempDir = func(_, _ string) (string, error) {
			return want, nil
		}
		sut := buildDeployProcess(nil, nil)

		sut.CreateTempWorkspace()

		if got := sut.tmpDir; got != want {
			t.Errorf("got tmpDir = %s, want %s", got, want)
		}
	})
	t.Run("it stores the created tmp dir", func(t *testing.T) {
		want := errors.New("test error")
		ioutilTempDir = func(_, _ string) (string, error) {
			return "", want
		}
		defer func() {
			ioutilTempDir = os.MkdirTemp
		}()
		sut := buildDeployProcess(nil, nil)

		sut.CreateTempWorkspace()

		gotErr := errors.Unwrap(sut.Err)
		if gotErr != want {
			t.Errorf("got err = %s, want %s", gotErr, want)
		}
	})
}

func TestDeployProcess_Cleanup(t *testing.T) {
	var testOut, testErr bytes.Buffer
	sut := buildDeployProcess(&testOut, &testErr)
	afterEach := func() {
		osRemoveAll = os.RemoveAll
		testOut.Reset()
		testErr.Reset()
		sut.Err = nil
		sut.tmpDir = ""
	}
	t.Run("it is a noop on sticky error", func(t *testing.T) {
		defer afterEach()
		sut.Err = errors.New("test error")
		var callCount int
		osRemoveAll = func(_ string) error {
			callCount++
			return nil
		}

		sut.Cleanup()

		want := 0
		if got := callCount; got != want {
			t.Errorf("got callCount = %d, want %d", got, want)
		}
	})
	t.Run("it removes the intended tmpdir", func(t *testing.T) {
		defer afterEach()
		sut.tmpDir = "/tmp/testing"
		var dirPassedForCleaning string
		osRemoveAll = func(d string) error {
			dirPassedForCleaning = d
			return nil
		}

		sut.Cleanup()

		want := sut.tmpDir
		if got := dirPassedForCleaning; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
	t.Run("it continues on failure but prints warning", func(t *testing.T) {
		defer afterEach()
		sut.tmpDir = "/tmp/testing"
		givenErr := errors.New("test error")
		osRemoveAll = func(_ string) error {
			return givenErr
		}

		sut.Cleanup()

		if got := sut.Err; got != nil {
			t.Errorf("got err = %v, but wanted nil", got)
		}
		wantMsg := "error: cleaning up temporary dir: /tmp/testing"
		if got := string(testErr.Bytes()); got != wantMsg {
			t.Errorf("got msg = %q, want %q", got, wantMsg)
		}
	})
}

func TestDeployProcess_RemoveSecretManifest(t *testing.T) {
	var testOut, testErr bytes.Buffer
	sut := buildDeployProcess(&testOut, &testErr)
	afterEach := func() {
		osRemove = os.Remove
		testOut.Reset()
		testErr.Reset()
		sut.Err = nil
	}
	t.Run("it is a noop on sticky error", func(t *testing.T) {
		defer afterEach()
		sut.Err = errors.New("test error")
		var callCount int
		osRemove = func(_ string) error {
			callCount++
			return nil
		}

		sut.RemoveSecretManifest()

		want := 0
		if got := callCount; got != want {
			t.Errorf("got callCount = %d, want %d", got, want)
		}
	})
	t.Run("it removes the intended secret file", func(t *testing.T) {
		defer afterEach()
		osRemove = func(_ string) error {
			return nil
		}

		sut.RemoveSecretManifest()

		if got := sut.Err; got != nil {
			t.Errorf("got err = %s, want nil", got)
		}
	})
	t.Run("it continues on failure but prints warning", func(t *testing.T) {
		defer afterEach()
		fName := "karavi-storage-secret.yaml"
		givenErr := errors.New(fName)
		osRemove = func(_ string) error {
			return givenErr
		}

		sut.RemoveSecretManifest()

		if got := sut.Err; got != nil {
			t.Errorf("got err = %v, but wanted nil", got)
		}
		wantMsg := fmt.Sprintln("error: cleaning up secret file:", fName)
		if got := string(testErr.Bytes()); got != wantMsg {
			t.Errorf("got msg = %q, want %q", got, wantMsg)
		}
	})
}

func TestDeployProcess_ChownK3sKubeConfig(t *testing.T) {
	sut := buildDeployProcess(nil, nil)
	afterEach := func() {
		osChown = os.Chown
		sut.Err = nil
		sut.processOwnerUID = RootUID
		sut.processOwnerGID = RootUID
	}

	t.Run("it is a noop on sticky error", func(t *testing.T) {
		defer afterEach()
		sut.Err = errors.New("test error")
		var callCount int
		osChown = func(_ string, _, _ int) error {
			callCount++
			return nil
		}

		sut.ChownK3sKubeConfig()

		want := 0
		if got := callCount; got != want {
			t.Errorf("got callCount = %d, want %d", got, want)
		}
	})
	t.Run("it is a noop when ran as pure root", func(t *testing.T) {
		defer afterEach()
		var callCount int
		osChown = func(_ string, _, _ int) error {
			callCount++
			return nil
		}
		sut.processOwnerUID = RootUID
		sut.processOwnerGID = RootUID

		sut.ChownK3sKubeConfig()

		want := 0
		if got := callCount; got != want {
			t.Errorf("got callCount = %d, want %d", got, want)
		}
	})
	t.Run("it chown's the kubeconfig file successfully", func(t *testing.T) {
		var gotUID, gotGID int
		osChown = func(_ string, uid, gid int) error {
			gotUID, gotGID = uid, gid
			return nil
		}
		sut.processOwnerUID = 1000
		sut.processOwnerGID = 1000

		sut.ChownK3sKubeConfig()

		wantUID, wantGID := 1000, 1000
		if gotUID != wantUID && gotGID != wantGID {
			t.Errorf("chown: got [%d,%d], want [%d,%d]", gotUID, gotGID, wantUID, wantGID)
		}
	})
	t.Run("it handles failure to chown the kubeconfig", func(t *testing.T) {
		err := errors.New("test error")
		osChown = func(_ string, uid, gid int) error {
			return err
		}
		sut.processOwnerUID = 1000
		sut.processOwnerGID = 1000

		sut.ChownK3sKubeConfig()

		want := err
		if got := errors.Unwrap(sut.Err); got != want {
			t.Errorf("got err = %v, want %v", got, want)
		}
	})
}

func TestDeployProcess_CopySidecarProxyToCwd(t *testing.T) {
	var testOut bytes.Buffer
	sut := buildDeployProcess(&testOut, nil)

	t.Run("it is a noop on sticky error", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
			sidecarImageTar = "sidecar-proxy-"
		})
		sut.Err = errors.New("test error")
		sut.CopySidecarProxyToCwd()

		want := 0
		if got := len(testOut.Bytes()); got != want {
			t.Errorf("len(stdout): got = %d, want %d", got, want)
		}
	})
	t.Run("it prints output to stdout", func(t *testing.T) {
		t.Cleanup(func() {
			sut.tmpDir = ""
			sut.Err = nil
			sidecarImageTar = "sidecar-proxy-"
		})
		sut.tmpDir = "/tmp/testing"

		sut.CopySidecarProxyToCwd()

		want := 69
		if got := len(testOut.Bytes()); got != want {
			t.Errorf("len(stdout): got = %d, want %d", got, want)
		}
	})
	t.Run("it handles failure to get cwd", func(t *testing.T) {
		t.Cleanup(func() {
			sut.tmpDir = ""
			sut.Err = nil
			osGetwd = os.Getwd
			sidecarImageTar = "sidecar-proxy-"
		})
		sut.tmpDir = "/tmp/testing"

		var callCount int
		osGetwd = func() (string, error) {
			callCount++
			return "", errors.New("test error")
		}

		sut.CopySidecarProxyToCwd()

		want := 1
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
	t.Run("it handles failure to move the file", func(t *testing.T) {
		t.Cleanup(func() {
			sut.tmpDir = ""
			sut.Err = nil
			execCommand = exec.Command
			sidecarImageTar = "sidecar-proxy-"
		})
		sut.tmpDir = "/tmp/testing"
		var callCount int
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			callCount++
			return exec.Command("false")
		}
		sut.CopySidecarProxyToCwd()

		want := 1
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
	t.Run("it handles failure to find sidecar", func(t *testing.T) {
		t.Cleanup(func() {
			sut.tmpDir = ""
			sut.Err = nil
			osGetwd = os.Getwd
			sidecarImageTar = "sidecar-proxy-"
			filepathWalkDir = filepath.WalkDir
		})
		sut.tmpDir = "/tmp/testing"
		filepathWalkDir = func(_ string, _ fs.WalkDirFunc) error {
			return errors.New("test error")
		}
		var callCount int
		osGetwd = func() (string, error) {
			callCount++
			return "", nil
		}
		sut.CopySidecarProxyToCwd()

		want := 0
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
	t.Run("it copies sidecar successfully", func(t *testing.T) {
		t.Cleanup(func() {
			sut.tmpDir = ""
			sut.Err = nil
			execCommand = exec.Command
			sidecarImageTar = "sidecar-proxy-"
		})
		sut.tmpDir = "./testdata"
		var callCount int
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			callCount++
			return exec.Command("true")
		}
		sut.CopySidecarProxyToCwd()

		want := 1
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
}

func TestDeployProcess_UntarFiles(t *testing.T) {
	var testOut, testErr bytes.Buffer
	sut := buildDeployProcess(&testOut, &testErr)
	before := func() {
		testOut.Reset()
		testErr.Reset()
		sut.Err = nil
		sut.bundleTar = &FakeFS{}

		tmpDir, err := os.MkdirTemp("", "deployProcess_UntarFilesTest")
		if err != nil {
			t.Fatal(err)
		}
		sut.tmpDir = tmpDir
	}
	after := func(sut *DeployProcess) {
		if err := os.RemoveAll(sut.tmpDir); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("it handles failure to open the bundle file", func(t *testing.T) {
		before()
		defer after(sut)
		sut.bundleTar = &FakeFS{ReturnErr: errors.New("test error")}

		sut.UntarFiles()

		want := "opening gzip file: test error"
		if got := sut.Err.Error(); got != want {
			t.Errorf("got err = %+v, want %+v", got, want)
		}
	})
	t.Run("it handles failure reading the gzip file", func(t *testing.T) {
		before()
		defer after(sut)
		gzipNewReader = func(_ io.Reader) (*gzip.Reader, error) {
			return nil, errors.New("test error")
		}
		defer func() {
			gzipNewReader = gzip.NewReader
		}()

		sut.UntarFiles()

		want := "creating gzip reader: test error"
		if got := sut.Err.Error(); got != want {
			t.Errorf("got err = %+v, want %+v", got, want)
		}
	})
	t.Run("it is a noop when there is an error", func(t *testing.T) {
		before()
		defer after(sut)
		sut.Err = errors.New("test error")

		sut.UntarFiles()

		{
			want := 0
			if got := len(testOut.Bytes()); got != want {
				t.Errorf("len(stdout): got = %d, want %d", got, want)
			}
		}
		{
			want := 0
			if got := len(testErr.Bytes()); got != want {
				t.Errorf("len(stderr): got = %d, want %d", got, want)
			}
		}
	})
	t.Run("happy path", func(t *testing.T) {
		before()
		defer after(sut)

		sut.UntarFiles()

		want := "Extracting files...Done!\n"
		if got := string(testOut.Bytes()); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		_, err := os.Stat(filepath.Join(sut.tmpDir, "dummy"))
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestDeployProcess_CreateRancherDirs(t *testing.T) {
	defer func() {
		createDir = noopCreateDir
	}()
	var testOut, testErr bytes.Buffer
	sut := buildDeployProcess(&testOut, &testErr)

	tests := []struct {
		name          string
		givenErr      error
		wantCallCount int
	}{
		{
			"creates zero directories",
			errors.New("test error"),
			0,
		},
		{
			"creates two directories",
			nil,
			2,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sut.Err = tt.givenErr
			var callCount int
			createDir = func(_ string) error {
				callCount++
				return nil
			}

			sut.CreateRancherDirs()

			if got := callCount; got != tt.wantCallCount {
				t.Errorf("got callCount %d, want %d", got, tt.wantCallCount)
			}
		})
	}
}

func TestDeployProcess_InstallKaravictl(t *testing.T) {
	sut := buildDeployProcess(nil, nil)

	t.Run("it is a noop on sticky error", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
			execCommand = exec.Command
		})
		sut.Err = errors.New("test error")
		var callCount int
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			callCount++
			return exec.Command("true")
		}

		sut.InstallKaravictl()

		want := 0
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
	t.Run("it moves karavictl to /usr/local/bin", func(t *testing.T) {
		t.Cleanup(func() {
			sut.tmpDir = ""
			execCommand = exec.Command
			osChmod = os.Chmod
		})
		sut.tmpDir = "/tmp/testing"
		var gotSrc, gotTgt string
		execCommand = func(_ string, args ...string) *exec.Cmd {
			gotSrc, gotTgt = args[2], args[3]
			return exec.Command("true")
		}
		osChmod = func(_ string, _ fs.FileMode) error {
			return nil
		}

		sut.InstallKaravictl()

		wantSrc := filepath.Join(sut.tmpDir, "karavictl")
		if gotSrc != wantSrc {
			t.Errorf("got srcfile %s, want %s", gotSrc, wantSrc)
		}
		wantTgt := "/usr/local/bin/karavictl"
		if gotTgt != wantTgt {
			t.Errorf("got tgtfile %s, want %s", gotTgt, wantTgt)
		}
	})
	t.Run("error in karavictl move", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
			execCommand = exec.Command
		})

		sut.tmpDir = "/tmp/testing"
		var callCount int
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			callCount++
			return exec.Command("true")
		}

		sut.InstallKaravictl()

		want := 1
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
	t.Run("error in karavictl chmod", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
			execCommand = exec.Command
			osChmod = os.Chmod
		})

		sut.tmpDir = "/tmp/testing"
		var callCount int
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			return exec.Command("true")
		}

		osChmod = func(_ string, _ fs.FileMode) error {
			callCount++
			return errors.New("chmod karavictl")
		}

		sut.InstallKaravictl()

		want := 1
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
}

func TestDeployProcess_InstallK3s(t *testing.T) {
	sut := buildDeployProcess(nil, nil)

	afterEach := func() {
		sut.tmpDir = ""
		sut.Err = nil
		execCommand = exec.Command
		osChmod = os.Chmod
	}

	t.Run("it is a noop on sticky error", func(t *testing.T) {
		defer afterEach()
		sut.Err = errors.New("test error")
		var callCount int
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			callCount++
			return exec.Command("true")
		}

		sut.InstallK3s()

		want := 0
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
	t.Run("it moves k3s to /usr/local/bin", func(t *testing.T) {
		defer afterEach()

		tgtDir, err := ioutilTempDir("", "")
		if err != nil {
			t.Fatal(err)
		}

		var createdFile, openedFile *os.File
		defer func() {
			err = os.Remove(openedFile.Name())
			if err != nil {
				t.Fatal(err)
			}
			err = os.RemoveAll(tgtDir)
			if err != nil {
				t.Fatal(err)
			}
		}()

		sut.tmpDir = "/tmp/testing"
		var gotSrc, gotTgt string
		osCreate = func(name string) (*os.File, error) {
			var err error
			createdFile, err = os.Create(filepath.Join(tgtDir, filepath.Base(name)))
			if err != nil {
				t.Fatal(err)
			}

			gotTgt = name
			return createdFile, nil
		}
		osChmod = func(_ string, _ fs.FileMode) error {
			return nil
		}
		osOpenFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
			var err error
			// openedFile, err = os.Create(filepath.Join(os.TempDir(), filepath.Base(name)))
			openedFile, err = ioutil.TempFile(os.TempDir(), "")
			if err != nil {
				t.Fatal(err)
			}

			gotSrc = name
			return openedFile, nil
		}

		sut.InstallK3s()

		wantSrc := filepath.Join(sut.tmpDir, "k3s")
		if gotSrc != wantSrc {
			t.Errorf("got srcfile %s, want %s", gotSrc, wantSrc)
		}
		wantTgt := "/usr/local/bin/k3s"
		if gotTgt != wantTgt {
			t.Errorf("got tgtfile %s, want %s", gotTgt, wantTgt)
		}
	})
	t.Run("it handles failure to move the k3s binary", func(t *testing.T) {
		defer afterEach()
		sut.Err = nil
		givenErr := errors.New("test error")
		osCreate = func(name string) (*os.File, error) {
			return nil, givenErr
		}

		sut.InstallK3s()

		want := givenErr
		if got := errors.Unwrap(sut.Err); got != want {
			t.Errorf("got err = %v, want %v", got, want)
		}
	})
	t.Run("error in chmod k3s", func(t *testing.T) {
		defer afterEach()

		tgtDir, err := ioutilTempDir("", "")
		if err != nil {
			t.Fatal(err)
		}

		var createdFile, openedFile *os.File
		defer func() {
			err = os.Remove(openedFile.Name())
			if err != nil {
				t.Fatal(err)
			}
			err = os.RemoveAll(tgtDir)
			if err != nil {
				t.Fatal(err)
			}
		}()

		osCreate = func(name string) (*os.File, error) {
			var err error
			createdFile, err = os.Create(filepath.Join(tgtDir, filepath.Base(name)))
			if err != nil {
				t.Fatal(err)
			}
			return createdFile, nil
		}

		osOpenFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
			var err error
			// openedFile, err = os.Create(filepath.Join(os.TempDir(), filepath.Base(name)))
			openedFile, err = ioutil.TempFile(os.TempDir(), "")
			if err != nil {
				t.Fatal(err)
			}
			return openedFile, nil
		}

		var callCount int
		osChmod = func(_ string, _ fs.FileMode) error {
			callCount++
			return errors.New("chmod k3s")
		}

		sut.InstallK3s()

		want := 1
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
}

func TestDeployProcess_CopyImagesToRancherDirs(t *testing.T) {
	sut := buildDeployProcess(nil, nil)

	t.Run("it is a noop on sticky error", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
			execCommand = exec.Command
		})
		sut.Err = errors.New("test error")
		var callCount int
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			callCount++
			return exec.Command("true")
		}

		sut.CopyImagesToRancherDirs()

		want := 0
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
	t.Run("copy ranchers images /var/lib/rancher/k3s/agent/images", func(t *testing.T) {
		t.Cleanup(func() {
			sut.tmpDir = ""
			execCommand = exec.Command
		})
		sut.tmpDir = "/tmp/testing"
		var callCount int
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			callCount++
			return exec.Command("true")
		}

		sut.CopyImagesToRancherDirs()

		want := 3
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
	t.Run("error in rancher images", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
			execCommand = exec.Command
		})

		var callCount int
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			callCount++
			return exec.Command("false")
		}

		sut.CopyImagesToRancherDirs()

		want := 1
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
}

func TestDeployProcess_CopyManifestsToRancherDirs(t *testing.T) {
	sut := buildDeployProcess(nil, nil)

	t.Run("it is a noop on sticky error", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
			execCommand = exec.Command
		})
		sut.Err = errors.New("test error")
		var callCount int
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			callCount++
			return exec.Command("true")
		}

		sut.CopyManifestsToRancherDirs()

		want := 0
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
	t.Run("copy ranchers manifests /var/lib/rancher/k3s/server/manifests", func(t *testing.T) {
		t.Cleanup(func() {
			sut.tmpDir = ""
			execCommand = exec.Command
		})
		sut.tmpDir = "/tmp/testing"
		sut.manifests = []string{"credShieldDeploymentManifest", "credShieldIngressManifest"}
		var callCount int
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			callCount++
			return exec.Command("true")
		}

		sut.CopyManifestsToRancherDirs()

		want := 2
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
	t.Run("error in rancher manifests", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
			execCommand = exec.Command
		})

		var callCount int
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			callCount++
			return exec.Command("false")
		}

		sut.CopyManifestsToRancherDirs()

		want := 1
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
}

func Test_config(t *testing.T) {
	configDir = "testdata/"
	sut := config()

	if sut == nil {
		t.Errorf("expected a non-nil config, but it was nil")
	}
}

func TestDeployProcess_WriteConfigSecretManifest(t *testing.T) {
	sut := buildDeployProcess(nil, nil)

	afterEach := func() {
		osOpenFile = os.OpenFile
		yamlMarshalSettings = realYamlMarshalSettings
		yamlMarshalSecret = realYamlMarshalSecret
		sut.Err = nil
	}

	t.Run("it is a noop on sticky error", func(t *testing.T) {
		defer afterEach()
		var callCount int
		osOpenFile = func(_ string, _ int, _ os.FileMode) (*os.File, error) {
			callCount++
			return nil, nil
		}
		sut.Err = errors.New("test error")

		sut.WriteConfigSecretManifest()

		want := 0
		if got := callCount; got != want {
			t.Errorf("got callCount = %v, want %v", got, want)
		}
	})
	t.Run("it writes config to a secret manifest", func(t *testing.T) {
		defer afterEach()
		tmpDir, err := os.MkdirTemp("", "WriteConfigSecretManifest")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)
		var configPath string
		osOpenFile = func(path string, _ int, _ os.FileMode) (*os.File, error) {
			configPath = filepath.Join(tmpDir, path)
			if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
				t.Fatal(err)
			}
			return os.Create(configPath)
		}
		sut.cfg.Set("foo", "bar")

		sut.WriteConfigSecretManifest()

		if sut.Err != nil {
			t.Fatalf("got err = %v, want nil", sut.Err)
		}
		got, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile("testdata/karavi-config-secret.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", string(got), string(want))
		}
	})
	t.Run("it handles file creation failure", func(t *testing.T) {
		defer afterEach()
		wantErr := errors.New("test error")
		osOpenFile = func(_ string, _ int, _ os.FileMode) (*os.File, error) {
			return nil, wantErr
		}

		sut.WriteConfigSecretManifest()

		want := wantErr
		if got := errors.Unwrap(sut.Err); got != want {
			t.Errorf("got err %v, want %v", got, want)
		}
	})
	t.Run("it handles file writing failure", func(t *testing.T) {
		defer afterEach()
		osOpenFile = func(_ string, _ int, _ os.FileMode) (*os.File, error) {
			// Return a nil file to force #Write to return an error.
			return nil, nil
		}

		sut.WriteConfigSecretManifest()

		want := os.ErrInvalid
		if got := errors.Unwrap(sut.Err); got != want {
			t.Errorf("got err %v, want %v", got, want)
		}
	})
	t.Run("it handles settings marshal failure", func(t *testing.T) {
		defer afterEach()
		wantErr := errors.New("test error")
		yamlMarshalSettings = func(_ *map[string]interface{}) ([]byte, error) {
			return nil, wantErr
		}

		sut.WriteConfigSecretManifest()

		want := wantErr
		if got := errors.Unwrap(sut.Err); got != want {
			t.Errorf("got err %v, want %v", got, want)
		}
	})
	t.Run("it handles secret marshal failure", func(t *testing.T) {
		defer afterEach()
		wantErr := errors.New("test error")
		yamlMarshalSecret = func(_ *corev1.Secret) ([]byte, error) {
			return nil, wantErr
		}

		sut.WriteConfigSecretManifest()

		want := wantErr
		if got := errors.Unwrap(sut.Err); got != want {
			t.Errorf("got err %v, want %v", got, want)
		}
	})
}

func TestDeployProcess_WriteStorageSecretManifest(t *testing.T) {
	sut := buildDeployProcess(nil, nil)

	afterEach := func() {
		execCommand = exec.Command
		yamlMarshalSettings = realYamlMarshalSettings
		yamlMarshalSecret = realYamlMarshalSecret
		sut.Err = nil
	}

	t.Run("it is a noop on sticky error", func(t *testing.T) {
		defer afterEach()
		var callCount int
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			callCount++
			return nil
		}
		sut.Err = errors.New("test error")

		sut.WriteStorageSecretManifest()

		want := 0
		if got := callCount; got != want {
			t.Errorf("got callCount = %v, want %v", got, want)
		}
	})
	t.Run("it writes config to a storage secret manifest", func(t *testing.T) {
		defer afterEach()
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			return exec.Command("false") // return a failure
		}
		tmpDir, err := os.MkdirTemp("", "WriteStorageSecretManifest")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)
		var configPath string
		osOpenFile = func(path string, _ int, _ os.FileMode) (*os.File, error) {
			configPath = filepath.Join(tmpDir, path)
			if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
				t.Fatal(err)
			}
			return os.Create(configPath)
		}

		sut.WriteStorageSecretManifest()

		if sut.Err != nil {
			t.Fatalf("got err = %v, want nil", sut.Err)
		}
		got, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile("testdata/karavi-storage-secret.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got:\n%v\nwant:\n%v\n", string(got), string(want))
		}
	})
	t.Run("it handles file creation failure", func(t *testing.T) {
		defer afterEach()
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			return exec.Command("false") // return a failure
		}
		wantErr := errors.New("test error")
		osOpenFile = func(_ string, _ int, _ os.FileMode) (*os.File, error) {
			return nil, wantErr
		}

		sut.WriteStorageSecretManifest()

		want := wantErr
		if got := errors.Unwrap(sut.Err); got != want {
			t.Errorf("got err %v, want %v", got, want)
		}
	})
	t.Run("it handles file writing failure", func(t *testing.T) {
		defer afterEach()
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			return exec.Command("false") // return a failure
		}
		osOpenFile = func(_ string, _ int, _ os.FileMode) (*os.File, error) {
			// Return a nil file to force #Write to return an error.
			return nil, nil
		}

		sut.WriteStorageSecretManifest()

		want := os.ErrInvalid
		if got := errors.Unwrap(sut.Err); got != want {
			t.Errorf("got err %v, want %v", got, want)
		}
	})
	t.Run("it handles secret marshal failure", func(t *testing.T) {
		defer afterEach()
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			return exec.Command("false") // return a failure
		}
		wantErr := errors.New("test error")
		yamlMarshalSecret = func(_ *corev1.Secret) ([]byte, error) {
			return nil, wantErr
		}

		sut.WriteStorageSecretManifest()

		want := wantErr
		if got := errors.Unwrap(sut.Err); got != want {
			t.Errorf("got err %v, want %v", got, want)
		}
	})
	t.Run("it skips creation if secret already exists", func(t *testing.T) {
		defer afterEach()
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			return exec.Command("true")
		}
		var callCount int
		osOpenFile = func(_ string, _ int, _ os.FileMode) (*os.File, error) {
			callCount++
			return nil, nil
		}

		sut.WriteStorageSecretManifest()

		want := 0
		if got := callCount; got != want {
			t.Errorf("got callCount = %v, want %v", got, want)
		}
	})
}

func TestDeployProcess_WriteConfigMapManifest(t *testing.T) {
	sut := buildDeployProcess(nil, nil)

	afterEach := func() {
		osOpenFile = os.OpenFile
		yamlMarshalSettings = realYamlMarshalSettings
		yamlMarshalSecret = realYamlMarshalSecret
		sut.Err = nil
	}

	t.Run("it is a noop on sticky error", func(t *testing.T) {
		defer afterEach()
		var callCount int
		osOpenFile = func(_ string, _ int, _ os.FileMode) (*os.File, error) {
			callCount++
			return nil, nil
		}
		sut.Err = errors.New("test error")

		sut.WriteConfigMapManifest()

		want := 0
		if got := callCount; got != want {
			t.Errorf("got callCount = %v, want %v", got, want)
		}
	})
	t.Run("it writes config to a configMap manifest", func(t *testing.T) {
		defer afterEach()
		tmpDir, err := os.MkdirTemp("", "WriteConfigMapManifest")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir)
		var configPath string
		osOpenFile = func(path string, _ int, _ os.FileMode) (*os.File, error) {
			configPath = filepath.Join(tmpDir, path)
			if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
				t.Fatal(err)
			}
			return os.Create(configPath)
		}
		sut.cfg.Set("proxy.loglevel", "debug")

		sut.WriteConfigMapManifest()

		if sut.Err != nil {
			t.Fatalf("got err = %v, want nil", sut.Err)
		}
		got, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile("testdata/karavi-configmap.yaml")
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", string(got), string(want))
		}
	})
	t.Run("it handles file creation failure", func(t *testing.T) {
		defer afterEach()
		wantErr := errors.New("test error")
		osOpenFile = func(_ string, _ int, _ os.FileMode) (*os.File, error) {
			return nil, wantErr
		}

		sut.WriteConfigMapManifest()

		want := wantErr
		if got := errors.Unwrap(sut.Err); got != want {
			t.Errorf("got err %v, want %v", got, want)
		}
	})
	t.Run("it handles file writing failure", func(t *testing.T) {
		defer afterEach()
		osOpenFile = func(_ string, _ int, _ os.FileMode) (*os.File, error) {
			// Return a nil file to force #Write to return an error.
			return nil, nil
		}

		sut.WriteConfigMapManifest()

		want := os.ErrInvalid
		if got := errors.Unwrap(sut.Err); got != want {
			t.Errorf("got err %v, want %v", got, want)
		}
	})
	t.Run("it handles settings marshal failure", func(t *testing.T) {
		defer afterEach()
		wantErr := errors.New("test error")
		yamlMarshalSettings = func(_ *map[string]interface{}) ([]byte, error) {
			return nil, wantErr
		}

		sut.WriteConfigMapManifest()

		want := wantErr
		if got := errors.Unwrap(sut.Err); got != want {
			t.Errorf("got err %v, want %v", got, want)
		}
	})
	t.Run("it handles marshal failure", func(t *testing.T) {
		defer afterEach()
		wantErr := errors.New("test error")
		yamlMarshalConfigMap = func(_ *corev1.ConfigMap) ([]byte, error) {
			return nil, wantErr
		}

		sut.WriteConfigMapManifest()

		want := wantErr
		if got := errors.Unwrap(sut.Err); got != want {
			t.Errorf("got err %v, want %v", got, want)
		}
	})
}

func TestDeployProcess_ExecuteK3sInstallScript(t *testing.T) {
	var testOut, testErr bytes.Buffer
	sut := buildDeployProcess(&testOut, &testErr)

	afterEach := func() {
		sut.Err = nil
		sut.tmpDir = ""
		testOut.Reset()
		testErr.Reset()
		osChmod = os.Chmod
		ioutilTempFile = os.CreateTemp
		execCommand = exec.Command
	}
	t.Run("it is a noop on sticky error", func(t *testing.T) {
		defer afterEach()
		sut.Err = errors.New("test error")

		sut.ExecuteK3sInstallScript()

		if got := len(testOut.Bytes()); got != 0 {
			t.Errorf("got output = %q, wanted no output", string(testOut.Bytes()))
		}
	})
	t.Run("it handles failure to chmod the script", func(t *testing.T) {
		defer afterEach()
		givenErr := errors.New("test error")
		osChmod = func(_ string, _ fs.FileMode) error {
			return givenErr
		}

		sut.ExecuteK3sInstallScript()

		want := givenErr
		if got := errors.Unwrap(sut.Err); got != want {
			t.Errorf("got err = %v, want %v", got, want)
		}
	})
	t.Run("it handles failure to create the tmp log file", func(t *testing.T) {
		defer afterEach()
		osChmod = func(_ string, _ fs.FileMode) error {
			return nil
		}
		givenErr := errors.New("test error")
		ioutilTempFile = func(_, _ string) (*os.File, error) {
			return nil, givenErr
		}

		sut.ExecuteK3sInstallScript()

		want := givenErr
		if got := errors.Unwrap(sut.Err); got != want {
			t.Errorf("got err = %v, want %v", got, want)
		}
	})
	t.Run("it handles failure to run the script", func(t *testing.T) {
		defer afterEach()
		osChmod = func(_ string, _ fs.FileMode) error {
			return nil
		}
		tmpFile, err := os.CreateTemp("", "testExecuteK3sInstallScript")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(tmpFile.Name())
		ioutilTempFile = func(_, _ string) (*os.File, error) {
			return tmpFile, nil
		}
		execCommand = func(_ string, _ ...string) *exec.Cmd {
			return exec.Command("false") // calling "false" will simulate a failure.
		}

		sut.ExecuteK3sInstallScript()

		if got := sut.Err; got == nil {
			t.Errorf("got err = %v, want non-nil", got)
		}
	})
}

func TestDeployProcess_PrintFinishedMessage(t *testing.T) {
	var testOut bytes.Buffer
	sut := buildDeployProcess(&testOut, nil)
	sidecarImageTar = "sidecar-proxy-1.0.0.tar"

	t.Run("it is a noop on sticky error", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
		})
		sut.Err = errors.New("test error")
		sut.PrintFinishedMessage()

		want := 0
		if got := len(testOut.Bytes()); got != want {
			t.Errorf("len(stdout): got = %d, want %d", got, want)
		}
	})
	t.Run("it prints the finished message", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
			sidecarImageTar = "sidecar-proxy-"
		})
		sut.PrintFinishedMessage()

		want := 220
		if got := len(testOut.Bytes()); got != want {
			t.Errorf("len(stdout): got = %d, want %d", got, want)
		}
	})
}

func buildDeployProcess(stdout, stderr io.Writer) *DeployProcess {
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}

	return &DeployProcess{
		stdout:    stdout,
		stderr:    stderr,
		bundleTar: &FakeFS{},
		cfg:       viper.New(),
		Steps:     []StepFunc{},
		manifests: []string{},
	}
}

type FakeFS struct {
	ReturnErr error
}

// Open opens the named file.
//
// When Open returns an error, it should be of type *PathError
// with the Op field set to "open", the Path field set to name,
// and the Err field describing the problem.
//
// Open should reject attempts to open names that do not satisfy
// ValidPath(name), returning a *PathError with Err set to
// ErrInvalid or ErrNotExist.
func (f *FakeFS) Open(_ string) (fs.File, error) {
	if f.ReturnErr != nil {
		return nil, f.ReturnErr
	}
	return os.Open("testdata/fake-bundle.tar.gz")
}

func TestDeployProcess_AddCertificate(t *testing.T) {
	var testOut bytes.Buffer
	sut := buildDeployProcess(&testOut, nil)
	certData := make(map[string]string)
	certData["foo"] = "bar"
	certData["foo2"] = "bar2"
	certData["foo3"] = "bar3"

	t.Run("it is a noop on sticky error", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
		})
		sut.Err = errors.New("test error")

		sut.AddCertificate()

		want := 0
		if got := len(testOut.Bytes()); got != want {
			t.Errorf("len(stdout): got = %d, want %d", got, want)
		}
	})
	t.Run("no certificate info in config file", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
			sut.manifests = []string{}
		})
		sut.manifests = nil

		sut.AddCertificate()

		if got := sut.manifests; got == nil {
			t.Errorf("manifests: got = %s, want not nil", got)
		}
	})
	t.Run("certificate files not listed", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
		})
		sut.cfg.Set("certificate", "foo")

		sut.AddCertificate()

		if got := sut.Err; got == nil {
			t.Errorf("Error: got = %s, want not nil", got)
		}
	})
	t.Run("certificate file type unknown", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
		})
		sut.cfg.Set("certificate", certData)

		sut.AddCertificate()

		if got := sut.Err; got == nil {
			t.Errorf("Error: got = %s, want not nil", got)
		}
	})
	t.Run("certificate file read error", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
		})
		sut.cfg.Set("certificate", certData)

		sut.AddCertificate()

		if got := sut.Err; got == nil {
			t.Errorf("Error: got = %s, want not nil", got)
		}
	})
	t.Run("certificate file write error", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
			sut.tmpDir = ""
			ioutilReadFile = os.ReadFile
		})
		sut.cfg.Set("certificate", certData)
		sut.tmpDir = "testData"
		ioutilReadFile = func(_ string) ([]byte, error) {
			return []byte{}, nil
		}

		sut.AddCertificate()

		if got := sut.Err; got == nil {
			t.Errorf("Error: got = %s, want not nil", got)
		}
	})
	t.Run("adds certificate to manifests", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
			sut.manifests = []string{}
			ioutilReadFile = os.ReadFile
			ioutilWriteFile = os.WriteFile
		})
		sut.cfg.Set("certificate", certData)
		ioutilReadFile = func(_ string) ([]byte, error) {
			return []byte{}, nil
		}
		ioutilWriteFile = func(_ string, _ []byte, _ os.FileMode) error {
			return nil
		}

		sut.AddCertificate()

		if got := sut.manifests; got == nil {
			t.Errorf("manifests: got = %s, want not nil", got)
		}
	})
}

func TestDeployProcess_AddHostName(t *testing.T) {
	var testOut bytes.Buffer
	sut := buildDeployProcess(&testOut, nil)
	hostName := "foo.com"

	t.Run("it is a noop on sticky error", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
		})
		sut.Err = errors.New("test error")

		sut.AddHostName()

		want := 0
		if got := len(testOut.Bytes()); got != want {
			t.Errorf("len(stdout): got = %d, want %d", got, want)
		}
	})
	t.Run("missing hostName configuration", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
		})

		sut.AddHostName()

		if got := sut.Err; got == nil {
			t.Errorf("Error: got = %s, want not nil", got)
		}
	})
	t.Run("ingress file read error", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
		})
		sut.cfg.Set("hostName", hostName)
		sut.tmpDir = "testData"

		sut.AddHostName()

		if got := sut.Err; got == nil {
			t.Errorf("Error: got = %s, want not nil", got)
		}
	})
	t.Run("ingress file write error", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
			ioutilReadFile = os.ReadFile
		})
		sut.cfg.Set("hostName", hostName)
		sut.tmpDir = "testData"
		ioutilReadFile = func(_ string) ([]byte, error) {
			return []byte{}, nil
		}

		sut.AddHostName()

		if got := sut.Err; got == nil {
			t.Errorf("Error: got = %s, want not nil", got)
		}
	})
}
