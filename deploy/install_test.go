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
		})
		sut.tmpDir = "/tmp/testing"
		var gotSrc, gotTgt string
		osRename = func(src string, tgt string) error {
			gotSrc, gotTgt = src, tgt
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
