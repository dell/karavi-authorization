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

package config

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
)

// Config is
type Config struct {
	log      *logrus.Entry
	path     string
	mu       sync.Mutex // guards callback
	callback func(fsnotify.Event)
	decoder  func() interface{}
}

var noopCallback = func(_ fsnotify.Event) {}

func New(log *logrus.Entry) *Config {
	return &Config{
		log:      log,
		callback: noopCallback,
	}
}

func (c *Config) SetPath(p string) {
	c.path = filepath.Clean(p)
}

func (c *Config) Watch(ctx context.Context) <-chan error {
	errors := make(chan error, 1)

	go func() {
		watcher, err := fsnotify.NewWatcher()
		if err != nil {
			errors <- err
			return
		}
		defer watcher.Close()

		dir, _ := filepath.Split(filepath.Clean(c.path))
		currSymLinkTgt, _ := filepath.EvalSymlinks(c.path)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case event, ok := <-watcher.Events:
					if !ok {
						c.log.Println("config: watcher closed")
						return
					}

					eventName := filepath.Clean(event.Name)
					tmpSymLinkTgt, _ := filepath.EvalSymlinks(c.path)
					writeOrCreateMask := fsnotify.Write | fsnotify.Create

					if (eventName == currSymLinkTgt && event.Op&writeOrCreateMask != 0) || tmpSymLinkTgt != "" && tmpSymLinkTgt != currSymLinkTgt {
						currSymLinkTgt = tmpSymLinkTgt

						c.mu.Lock()
						c.callback(event)
						c.mu.Unlock()
					}
				case <-ctx.Done():
					errors <- ctx.Err()
					return
				}
			}
		}()

		err = watcher.Add(dir)
		if err != nil {
			errors <- err
			return
		}
		wg.Wait()
	}()
	return errors
}

func (c *Config) OnChange(f func(fsnotify.Event)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callback = f
}

func (c *Config) ReadConfig() (io.Reader, error) {
	b, err := ioutil.ReadFile(c.path)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(b), nil
}
