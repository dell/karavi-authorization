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

package cmd

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_Unit_RoleDelete(t *testing.T) {

	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmd := exec.CommandContext(
			context.Background(),
			os.Args[0],
			append([]string{
				"-test.run=TestK3sRoleSubprocess",
				"--",
				name}, args...)...)
		cmd.Env = append(os.Environ(), "WANT_GO_TEST_SUBPROCESS=1")

		return cmd
	}
	defer func() {
		execCommandContext = exec.CommandContext
	}()

	tests := map[string]func(t *testing.T) ([]string, int){
		"success deleting existing role": func(*testing.T) ([]string, int) {
			return []string{"CSIGold"}, 0
		},
		"error deleting role that doesn't exist": func(*testing.T) ([]string, int) {
			return []string{"non-existing-role"}, 1
		},
		"error passing no role to the command": func(*testing.T) ([]string, int) {
			return []string{}, 1
		},
		"error passing multiple roles to the command": func(*testing.T) ([]string, int) {
			return []string{"role-1", "role-2"}, 1
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			rolesToDelete, wantCode := tc(t)

			cmd := rootCmd
			args := []string{"role", "delete"}
			for _, role := range rolesToDelete {
				args = append(args, role)
			}
			cmd.SetArgs(args)

			var gotCode int
			done := make(chan struct{})
			if wantCode == 1 {
				defer func() { osExit = os.Exit }()
				osExit = func(code int) {
					gotCode = code
					done <- struct{}{}
					done <- struct{}{}
				}

				go cmd.Execute()
				<-done
			} else {
				osExit = os.Exit
				cmd.Execute()
			}

			assert.Equal(t, wantCode, gotCode)
		})
	}
}
