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
	"karavi-authorization/cmd/karavictl/cmd/api"
	"karavi-authorization/cmd/karavictl/cmd/api/mocks"
	"net/url"
	"os"
	"testing"
)

func TestTenantList(t *testing.T) {
	afterFn := func() {
		CreateHTTPClient = createHTTPClient
		JSONOutput = jsonOutput
		osExit = os.Exit
	}

	t.Run("it requests listing of a tenants", func(t *testing.T) {
		defer afterFn()
		var gotCalled bool
		CreateHTTPClient = func(addr string, insecure bool) (api.Client, error) {
			return &mocks.FakeClient{
				GetFn: func(ctx context.Context, path string, headers map[string]string, query url.Values, resp interface{}) error {
					gotCalled = true
					b := []byte(`{"tenants": [{"name": "test"}]}`)
					err := json.Unmarshal(b, resp)
					if err != nil {
						t.Fatal(err)
					}
					return nil
				},
			}, nil
		}
		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetOutput(&gotOutput)
		cmd.SetArgs([]string{"tenant", "list"})
		cmd.Execute()

		if !gotCalled {
			t.Error("expected ListTenant to be called, but it wasn't")
		}
	})
	t.Run("it requires a valid tenant server connection", func(t *testing.T) {
		defer afterFn()
		CreateHTTPClient = func(addr string, insecure bool) (api.Client, error) {
			return nil, errors.New("test error")
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
		cmd.SetArgs([]string{"tenant", "list"})
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
		wantErrMsg := "test error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
	t.Run("it handles server errors", func(t *testing.T) {
		defer afterFn()
		CreateHTTPClient = func(addr string, insecure bool) (api.Client, error) {
			return &mocks.FakeClient{
				GetFn: func(ctx context.Context, path string, headers map[string]string, query url.Values, resp interface{}) error {
					return errors.New("test error")
				},
			}, nil
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
		//tenantListCmd := NewTenantListCmd()

		rootCmd.SetErr(&gotOutput)
		rootCmd.SetArgs([]string{"tenant", "list"})

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
		wantErrMsg := "test error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
}
