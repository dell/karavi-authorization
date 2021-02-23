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

func TestDeployProcess(t *testing.T) {
	t.Run("UntarFiles", testDeployProcess_UntarFiles)
	t.Run("CreateRequiredDirsForK3s", testDeployProcess_CreateRequiredDirsForK3s)
}

func testDeployProcess_UntarFiles(t *testing.T) {
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

		want := "Extracting files...Done\n"
		if got := string(testOut.Bytes()); got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		_, err := os.Stat(filepath.Join(sut.tmpDir, "dummy"))
		if err != nil {
			t.Fatal(err)
		}
	})
}

func testDeployProcess_CreateRequiredDirsForK3s(t *testing.T) {
	var testOut, testErr bytes.Buffer
	sut := buildDeployProcess(&testOut, &testErr)

	t.Run("it is a noop on previous error", func(t *testing.T) {
		sut.Err = errors.New("test error")
		var callCount int
		createDir = func(_ string) error {
			callCount++
			return nil
		}
		defer func() {
			createDir = noopCreateDir
		}()

		sut.CreateRequiredDirsForK3s()

		want := 0
		if got := callCount; got != want {
			t.Errorf("got callCount %d, want %d", got, want)
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

	noopFunc := func() {}
	return &DeployProcess{
		stdout:                  stdout,
		stderr:                  stderr,
		bundleTar:               &FakeFS{},
		CreateTempWorkspaceFunc: noopFunc,
		UntarFilesFunc:          noopFunc,
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
