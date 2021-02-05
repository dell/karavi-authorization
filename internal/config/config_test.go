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

package config_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"karavi-authorization/internal/config"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

func TestNew(t *testing.T) {
	v := config.New(logrus.New().WithContext(context.Background()))

	if v == nil {
		t.Fatal("expected non-nil return value")
	}
}

func TestConfig(t *testing.T) {
	t.Run("watch a file for changes", testConfigWatch)
	t.Run("reading config", testConfigReadConfig)
}

func testConfigWatch(t *testing.T) {
	log := logrus.New().WithContext(context.Background())
	log.Logger.Out = ioutil.Discard
	sut := config.New(log)
	dir, err := ioutil.TempDir("", "karavi")
	if err != nil {
		t.Fatal(err)
	}
	tmp, err := ioutil.TempFile(dir, "karavi")
	if err != nil {
		t.Fatal(err)
	}
	symLinkName := filepath.Join(filepath.Dir(tmp.Name()), "karavi-config")
	defer func() {
		_ = os.RemoveAll(dir)
	}()

	var changed bool
	done := make(chan struct{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sut.SetPath(symLinkName)
	sut.Watch(ctx)
	sut.OnChange(func(e fsnotify.Event) {
		changed = true
		done <- struct{}{}
	})
	// Create a symlink to the file
	if err := exec.Command("ln", "-fsn", tmp.Name(), symLinkName).Run(); err != nil {
		t.Fatal(err)
	}
	// Wait for the OnChange notification
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Log("change notification took too long")
	}

	if !changed {
		t.Error("expected OnChange to be called, but it was not")
	}
}

func testConfigReadConfig(t *testing.T) {
	log := logrus.New().WithContext(context.Background())
	log.Logger.Out = ioutil.Discard
	sut := config.New(log)
	tmp, err := ioutil.TempFile("", "karavi")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = os.Remove(tmp.Name())
	}()
	fmt.Fprintf(tmp, "foo")

	t.Run("success", func(t *testing.T) {
		sut.SetPath(tmp.Name())
		r, err := sut.ReadConfig()
		if err != nil {
			t.Fatal(err)
		}

		b, err := ioutil.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		if string(b) != "foo" {
			t.Error("unexpected contents when reading config")
		}
	})
	t.Run("handles file not found", func(t *testing.T) {
		tmp, err := ioutil.TempFile("", "karavi")
		if err != nil {
			t.Fatal(err)
		}
		sut.SetPath(tmp.Name())

		// remove the file before reading it
		err = os.Remove(tmp.Name())
		if err != nil {
			t.Fatal(err)
		}
		_, err = sut.ReadConfig()

		if err == nil {
			t.Error("expected non-nil err, but got nil")
		}
	})
}
