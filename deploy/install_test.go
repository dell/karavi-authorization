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

package main

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
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
	got := NewDeploymentProcess()

	if got == nil {
		t.Error("expected non-nil return value")
	}
}

func TestDeployProcess_CreateTempWorkspace(t *testing.T) {
	t.Run("it stores the created tmp dir", func(t *testing.T) {
		want := "/tmp/testing"
		ioutilTempDir = func(_, _ string) (string, error) {
			return want, nil
		}
		defer func() {
			ioutilTempDir = ioutil.TempDir
		}()
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
			ioutilTempDir = ioutil.TempDir
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
		t.Skip("TODO")
	})
	t.Run("it prints output to stderr on failure", func(t *testing.T) {
		t.Skip("TODO")
	})
}

func TestDeployProcess_CopySidecarProxyToCwd(t *testing.T) {
	t.Run("it is a noop on sticky error", func(t *testing.T) {
		t.Skip("TODO")
	})
	t.Run("it prints output to stdout", func(t *testing.T) {
		t.Skip("TODO")
	})
	t.Run("it handles failure to get cwd", func(t *testing.T) {
		t.Skip("TODO")
	})
	t.Run("it handles failure to move the file", func(t *testing.T) {
		t.Skip("TODO")
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

		tmpDir, err := ioutil.TempDir("", "deployProcess_UntarFilesTest")
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

	var tests = []struct {
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
			osRename = os.Rename
		})
		sut.Err = errors.New("test error")
		var callCount int
		osRename = func(_ string, _ string) error {
			callCount++
			return nil
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
			osRename = os.Rename
			osChmod = os.Chmod
		})
		sut.tmpDir = "/tmp/testing"
		var gotSrc, gotTgt string
		osRename = func(src string, tgt string) error {
			gotSrc, gotTgt = src, tgt
			return nil
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
			osRename = os.Rename
		})

		sut.tmpDir = "/tmp/testing"
		var callCount int
		osRename = func(_ string, _ string) error {
			callCount++
			return errors.New("moving karavictl")
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
			osRename = os.Rename
			osChmod = os.Chmod
		})

		sut.tmpDir = "/tmp/testing"
		var callCount int
		osRename = func(_ string, _ string) error {
			return nil
		}

		osChmod = func(_ string, _ fs.FileMode) error {
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

	t.Run("it is a noop on sticky error", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
			osRename = os.Rename
		})
		sut.Err = errors.New("test error")
		var callCount int
		osRename = func(_ string, _ string) error {
			callCount++
			return nil
		}

		sut.InstallK3s()

		want := 0
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
	t.Run("it moves k3s to /usr/local/bin", func(t *testing.T) {
		t.Cleanup(func() {
			sut.tmpDir = ""
			osRename = os.Rename
		})
		sut.tmpDir = "/tmp/testing"
		var gotSrc, gotTgt string
		osRename = func(src string, tgt string) error {
			gotSrc, gotTgt = src, tgt
			return nil
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
	t.Run("error in k3s move", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
			osRename = os.Rename
		})

		var callCount int
		osRename = func(_ string, _ string) error {
			callCount++
			return errors.New("moving k3s binary")
		}

		sut.InstallK3s()

		want := 1
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
	t.Run("error in chmod k3s", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
			osRename = os.Rename
			osChmod = os.Chmod
		})

		var callCount int
		osRename = func(_ string, _ string) error {
			return nil
		}
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
			osRename = os.Rename
		})
		sut.Err = errors.New("test error")
		var callCount int
		osRename = func(_ string, _ string) error {
			callCount++
			return nil
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
			osRename = os.Rename
		})
		sut.tmpDir = "/tmp/testing"
		var callCount int
		osRename = func(_ string, _ string) error {
			callCount++
			return nil
		}

		sut.CopyImagesToRancherDirs()

		want := 2
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
	t.Run("error in rancher images", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
			osRename = os.Rename
		})

		var callCount int
		osRename = func(_ string, _ string) error {
			callCount++
			return errors.New("moving rancher images")
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
			osRename = os.Rename
		})
		sut.Err = errors.New("test error")
		var callCount int
		osRename = func(_ string, _ string) error {
			callCount++
			return nil
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
			osRename = os.Rename
		})
		sut.tmpDir = "/tmp/testing"
		var callCount int
		osRename = func(_ string, _ string) error {
			callCount++
			return nil
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
			osRename = os.Rename
		})

		var callCount int
		osRename = func(_ string, _ string) error {
			callCount++
			return errors.New("moving rancher manifests")
		}

		sut.CopyManifestsToRancherDirs()

		want := 1
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
		}
	})
}

func TestDeployProcess_ExecuteK3sInstallScript(t *testing.T) {
	t.Skip("TODO")
}

func TestDeployProcess_InitKaraviPolicies(t *testing.T) {
	var testOut bytes.Buffer
	sut := buildDeployProcess(&testOut, nil)

	t.Run("it is a noop on sticky error", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
			testOut.Reset()
		})
		sut.Err = errors.New("test error")
		sut.InitKaraviPolicies()

		want := 0
		if got := len(testOut.Bytes()); got != want {
			t.Errorf("len(stdout): got = %d, want %d", got, want)
		}

	})
	t.Run("failed to create log file", func(t *testing.T) {
		t.Cleanup(func() {
			sut.Err = nil
		})
		want := errors.New("test error")
		ioutilTempFile = func(_, _ string) (*os.File, error) {
			return nil, want
		}
		defer func() {
			ioutilTempFile = ioutil.TempFile
		}()

		sut.InitKaraviPolicies()

		gotErr := errors.Unwrap(sut.Err)
		if gotErr != want {
			t.Errorf("got err = %s, want %s", gotErr, want)
		}

	})
	t.Run("failed to run policy script", func(t *testing.T) {
		t.Skip("TODO") //exec.Command
	})
	t.Run("run policy script", func(t *testing.T) {
		want := ""
		sut.InitKaraviPolicies()

		if got := string(testOut.Bytes()); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestDeployProcess_PrintFinishedMessage(t *testing.T) {
	var testOut bytes.Buffer
	sut := buildDeployProcess(&testOut, nil)

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
		sut.PrintFinishedMessage()

		want := 221
		if got := len(testOut.Bytes()); got != want {
			t.Errorf("len(stdout): got = %d, want %d", got, want)
		}

	})
}

func buildDeployProcess(stdout, stderr io.Writer) *DeployProcess {
	if stdout == nil {
		stdout = ioutil.Discard
	}
	if stderr == nil {
		stderr = ioutil.Discard
	}

	return &DeployProcess{
		stdout:    stdout,
		stderr:    stderr,
		bundleTar: &FakeFS{},
		Steps:     []StepFunc{},
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
