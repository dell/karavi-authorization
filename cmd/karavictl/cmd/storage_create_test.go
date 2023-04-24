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

package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"karavi-authorization/cmd/karavictl/cmd/api"
	"karavi-authorization/cmd/karavictl/cmd/api/mocks"
	"net/url"
	"os"
	"testing"

	"golang.org/x/term"
)

func Test_readPassword(t *testing.T) {
	afterEach := func() {
		termReadPassword = term.ReadPassword
		osExit = os.Exit
	}
	t.Run("it prompts for a password", func(t *testing.T) {
		defer afterEach()
		termReadPassword = func(fd int) ([]byte, error) {
			return []byte("test"), nil
		}
		var (
			in bytes.Buffer
			v  string
		)
		prompt := "prompt: "

		readPassword(&in, prompt, &v)

		want := []byte(prompt + "\n")
		if got := in.Bytes(); !bytes.Equal(got, want) {
			t.Errorf("prompt: got %#v, want %#v", string(got), string(want))
		}
	})
	t.Run("it handles term failure", func(t *testing.T) {
		defer afterEach()
		termReadPassword = func(fd int) ([]byte, error) {
			return nil, errors.New("test error")
		}
		done := make(chan struct{})
		var statusCode int
		osExit = func(c int) {
			statusCode = c
			done <- struct{}{}
			done <- struct{}{} // stop this function returning
		}
		go func() {
			readPassword(ioutil.Discard, "prompt", new(string))
		}()
		<-done

		want := 1
		if got := statusCode; got != want {
			t.Errorf("statuscode: got %d, want %d", got, want)
		}
	})
}

func TestStorageCreateHandler(t *testing.T) {
	afterFn := func() {
		CreateHTTPClient = createHTTPClient
		JSONOutput = jsonOutput
		osExit = os.Exit
		ReadAccessAdminToken = readAccessAdminToken
	}

	t.Run("it requests creation of a storage", func(t *testing.T) {
		defer afterFn()
		var gotCalled bool
		CreateHTTPClient = func(addr string, insecure bool) (api.Client, error) {
			return &mocks.FakeClient{
				PostFn: func(ctx context.Context, path string, headers map[string]string, query url.Values, body, resp interface{}) error {
					gotCalled = true
					return nil
				},
			}, nil
		}
		ReadAccessAdminToken = func(afile string) (string, string, error) {
			return "AUnumberTokenIsNotWorkingman", "AUnumberTokenIsNotWorkingman", nil
		}
		JSONOutput = func(w io.Writer, _ interface{}) error {
			return nil
		}
		osExit = func(code int) {
		}
		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetOutput(&gotOutput)
		cmd.SetArgs([]string{"--admin-token", "admin.yaml", "storage", "create", "--addr", "https://storage-service.com", "--endpoint", "https://0.0.0.0:443", "--system-id", "testing123", "--type", "powerflex", "--user", "admin", "--password", "password", "--insecure", "--array-insecure"})
		cmd.Execute()

		if !gotCalled {
			t.Error("expected Create to be called, but it wasn't")
		}
	})
	t.Run("it requires a valid storage server connection", func(t *testing.T) {
		defer afterFn()
		CreateHTTPClient = func(addr string, insecure bool) (api.Client, error) {
			return nil, errors.New("failed to create storage: test error")
		}
		ReadAccessAdminToken = func(afile string) (string, string, error) {
			return "AUnumberTokenIsNotWorkingman", "AUnumberTokenIsNotWorkingman", nil
		}
		var gotCode int
		done := make(chan struct{})
		osExit = func(code int) {
			gotCode = code
			done <- struct{}{}
			done <- struct{}{} // we can't let this function return
		}
		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetErr(&gotOutput)
		cmd.SetArgs([]string{"--admin-token", "admin.yaml", "storage", "create", "--addr", "https://storage-service.com", "--endpoint", "https://0.0.0.0:443", "--system-id", "testing123", "--type", "powerflex", "--user", "admin", "--password", "password", "--insecure", "--array-insecure"})
		go cmd.Execute()
		<-done

		wantCode := 1
		if gotCode != wantCode {
			t.Errorf("got exit code %d, want %d", gotCode, wantCode)
		}
		var gotErr CommandError
		if err := json.NewDecoder(&gotOutput).Decode(&gotErr); err != nil {
			t.Fatal(err)
		}
		wantErrMsg := "failed to create storage: test error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
	t.Run("it handles server errors", func(t *testing.T) {
		defer afterFn()
		CreateHTTPClient = func(addr string, insecure bool) (api.Client, error) {
			return nil, errors.New("failed to create storage: test error")
		}
		ReadAccessAdminToken = func(afile string) (string, string, error) {
			return "AUnumberTokenIsNotWorkingman", "AUnumberTokenIsNotWorkingman", nil
		}
		var gotCode int
		done := make(chan struct{})
		osExit = func(code int) {
			gotCode = code
			done <- struct{}{}
			done <- struct{}{} // we can't let this function return
		}
		var gotOutput bytes.Buffer

		rootCmd := NewRootCmd()
		rootCmd.SetErr(&gotOutput)
		rootCmd.SetArgs([]string{"--admin-token", "admin.yaml", "storage", "create", "--addr", "https://storage-service.com", "--endpoint", "https://0.0.0.0:443", "--system-id", "testing123", "--type", "powerflex", "--user", "admin", "--password", "password", "--insecure", "--array-insecure"})

		go rootCmd.Execute()
		<-done

		wantCode := 1
		if gotCode != wantCode {
			t.Errorf("got exit code %d, want %d", gotCode, wantCode)
		}
		var gotErr CommandError
		if err := json.NewDecoder(&gotOutput).Decode(&gotErr); err != nil {
			t.Fatal(err)
		}
		wantErrMsg := "failed to create storage: test error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
}
