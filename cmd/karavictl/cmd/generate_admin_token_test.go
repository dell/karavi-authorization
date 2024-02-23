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
	"encoding/json"
	"io"
	"os"
	"testing"
)

func TestGenerateAdminToken(t *testing.T) {
	afterFn := func() {
		JSONOutput = jsonOutput
		osExit = os.Exit
	}
	t.Run("it requests creation of admin token", func(t *testing.T) {
		defer afterFn()
		JSONOutput = func(_ io.Writer, _ interface{}) error {
			return nil
		}
		osExit = func(_ int) {
		}
		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetOutput(&gotOutput)
		cmd.SetArgs([]string{"admin", "token", "--name", "testname", "-s", "password"})
		cmd.Execute()

		if len(gotOutput.Bytes()) != 0 {
			t.Errorf("expected zero output but got %q", string(gotOutput.Bytes()))
		}
	})
	t.Run("it requires a valid name argument", func(t *testing.T) {
		defer afterFn()
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
		rootCmd.SetArgs([]string{"admin", "token"})
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
		wantErrMsg := "empty admin name not allowed"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
}
