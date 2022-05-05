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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"karavi-authorization/internal/role-service/roles"

	"github.com/spf13/cobra"
)

// NewRoleGetCmd creates a new role get command
func NewRoleGetCmd() *cobra.Command {
	roleGetCmd := &cobra.Command{
		Use:   "get",
		Short: "Get role",
		Long:  `Get role`,
		Run: func(cmd *cobra.Command, args []string) {

			if len(args) == 0 {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), errors.New("role name is required"))
			}

			if len(args) > 1 {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), errors.New("expects single argument"))
			}

			r, err := GetRoles()
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("unable to list roles: %v", err))
			}

			roleName := strings.TrimSpace(args[0])

			matches := []roles.Instance{}
			r.Select(func(r roles.Instance) {
				if r.Name == roleName {
					matches = append(matches, r)
				}
			})
			if len(matches) == 0 {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("role %s does not exist", roleName))
			}

			var buf bytes.Buffer
			if err := json.NewEncoder(&buf).Encode(&r); err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}
			var m map[string]interface{}
			if err := json.NewDecoder(&buf).Decode(&m); err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}
			for k := range m {
				if k != roleName {
					delete(m, k)
				}
			}

			err = JSONOutput(cmd.OutOrStdout(), m)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("unable to format json output: %v", err))
			}
		},
	}
	return roleGetCmd
}
