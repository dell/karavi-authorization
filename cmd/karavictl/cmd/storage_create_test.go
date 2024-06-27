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
	"karavi-authorization/cmd/karavictl/cmd/api"
	"karavi-authorization/cmd/karavictl/cmd/api/mocks"
	"karavi-authorization/pb"
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
		termReadPassword = func(_ int) ([]byte, error) {
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
		termReadPassword = func(_ int) ([]byte, error) {
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
			readPassword(io.Discard, "prompt", new(string))
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
		termReadPassword = term.ReadPassword
	}

	t.Run("it requests creation of a storage", func(t *testing.T) {
		defer afterFn()
		var gotCalled bool
		CreateHTTPClient = func(_ string, _ bool) (api.Client, error) {
			return &mocks.FakeClient{
				PostFn: func(_ context.Context, _ string, _ map[string]string, _ url.Values, _, _ interface{}) error {
					gotCalled = true
					return nil
				},
			}, nil
		}
		ReadAccessAdminToken = func(_ string) (string, string, error) {
			return "AUnumberTokenIsNotWorkingman", "AUnumberTokenIsNotWorkingman", nil
		}
		JSONOutput = func(_ io.Writer, _ interface{}) error {
			return nil
		}
		osExit = func(_ int) {
		}
		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetOutput(&gotOutput)
		cmd.SetArgs([]string{"storage", "create", "--endpoint", "https://0.0.0.0:443", "--system-id", "testing123", "--type", "powerflex", "--user", "admin", "--password", "password", "--insecure", "--array-insecure", "--admin-token", "admin.yaml", "--addr", "proxy.com"})
		cmd.Execute()

		if !gotCalled {
			t.Error("expected Create to be called, but it wasn't")
		}
	})
	t.Run("it requests creation of a storage without the password flag", func(t *testing.T) {
		defer afterFn()
		termReadPassword = func(_ int) ([]byte, error) {
			return []byte("password"), nil
		}

		var gotCalled bool
		var gotPassword string
		CreateHTTPClient = func(_ string, _ bool) (api.Client, error) {
			return &mocks.FakeClient{
				PostFn: func(_ context.Context, _ string, _ map[string]string, _ url.Values, body interface{}, _ interface{}) error {
					gotCalled = true
					storageCreateRequest, ok := body.(**pb.StorageCreateRequest)
					if !ok {
						t.Fatalf("unexpected type %T for request body", body)
					}
					gotPassword = (*storageCreateRequest).Password
					return nil
				},
			}, nil
		}
		ReadAccessAdminToken = func(_ string) (string, string, error) {
			return "AUnumberTokenIsNotWorkingman", "AUnumberTokenIsNotWorkingman", nil
		}
		JSONOutput = func(_ io.Writer, _ interface{}) error {
			return nil
		}
		osExit = func(_ int) {
		}
		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetOutput(&gotOutput)
		cmd.SetIn(bytes.NewBufferString("password"))
		cmd.SetArgs([]string{"storage", "create", "--endpoint", "https://0.0.0.0:443", "--system-id", "testing123", "--type", "powerflex", "--user", "admin", "--insecure", "--array-insecure", "--admin-token", "admin.yaml", "--addr", "proxy.com"})
		cmd.Execute()

		if !gotCalled {
			t.Error("expected Create to be called, but it wasn't")
		}

		if gotPassword != "password" {
			t.Errorf("expected password %s, got %s", "password", gotPassword)
		}
	})
	t.Run("it requires a valid storage server connection", func(t *testing.T) {
		defer afterFn()
		CreateHTTPClient = func(_ string, _ bool) (api.Client, error) {
			return nil, errors.New("failed to create storage: test error")
		}
		ReadAccessAdminToken = func(_ string) (string, string, error) {
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
		cmd.SetArgs([]string{"storage", "create", "--endpoint", "https://0.0.0.0:443", "--system-id", "testing123", "--type", "powerflex", "--user", "admin", "--password", "password", "--insecure", "--array-insecure", "--admin-token", "admin.yaml", "--addr", "proxy.com"})
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
		CreateHTTPClient = func(_ string, _ bool) (api.Client, error) {
			return nil, errors.New("failed to create storage: test error")
		}
		ReadAccessAdminToken = func(_ string) (string, string, error) {
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
		rootCmd.SetArgs([]string{"storage", "create", "--endpoint", "https://0.0.0.0:443", "--system-id", "testing123", "--type", "powerflex", "--user", "admin", "--password", "password", "--insecure", "--array-insecure", "--admin-token", "admin.yaml", "--addr", "proxy.com"})

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
