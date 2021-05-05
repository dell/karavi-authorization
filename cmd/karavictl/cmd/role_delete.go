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
	"errors"
	"fmt"
	"karavi-authorization/internal/roles"
	"strings"

	"github.com/spf13/cobra"
)

// NewRoleDeleteCmd creates a new role delete command
func NewRoleDeleteCmd() *cobra.Command {
	roleDeleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete role",
		Long:  `Delete role`,
		Run: func(cmd *cobra.Command, args []string) {
			roleFlags, err := cmd.Flags().GetStringSlice("role")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			if len(roleFlags) == 0 {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), errors.New("no roles given"))
			}

			existing, err := GetRoles()
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("unable to get roles: %v", err))
			}

			matched := make(map[roles.Instance]struct{})
			for _, v := range roleFlags {
				t := strings.Split(v, "=")
				r, err := roles.NewInstance(t[0], t[1:]...)
				if err != nil {
					continue
				}
				existing.Select(func(e roles.Instance) {
					if strings.Contains(e.RoleKey.String(), r.RoleKey.String()) {
						matched[e] = struct{}{}
					}
				})
			}

			if len(matched) == 0 {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("no roles to delete"))
			}

			for k := range matched {
				err = existing.Remove(&k)
				if err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				}
			}

			err = modifyCommonConfigMap(existing)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("unable to delete role: %v", err))
			}
		},
	}
	roleDeleteCmd.Flags().StringSlice("role", []string{}, "role in the form <name>=<type>=<id>=<pool>")
	return roleDeleteCmd
}
