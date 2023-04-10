// Copyright © 2021-2022 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"fmt"
	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/pb"

	"github.com/spf13/cobra"
)

// NewRoleListCmd creates a new role list command
func NewRoleListCmd() *cobra.Command {
	roleListCmd := &cobra.Command{
		Use:   "list",
		Short: "List CSM roles",
		Long:  `List CSM roles`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			addr, err := cmd.Flags().GetString("addr")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			insecure, err := cmd.Flags().GetBool("insecure")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			var configuredRoles *roles.JSON
			if addr != "" {
				configuredRoles, err = doRoleListRequest(ctx, addr, insecure, cmd)
				if err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				}
			} else {
				configuredRoles, err = GetRoles()
				if err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("unable to list roles: %v", err))
				}
			}
			readRole := roles.TransformReadable(configuredRoles)
			err = JSONOutput(cmd.OutOrStdout(), &readRole)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("unable to format json output: %v", err))
			}
		},
	}
	return roleListCmd
}

func doRoleListRequest(ctx context.Context, addr string, insecure bool, cmd *cobra.Command) (*roles.JSON, error) {
	client, err := CreateHTTPClient(fmt.Sprintf("https://%s", addr), insecure)
	if err != nil {
		reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
	}

	var list pb.RoleListResponse

	err = client.Get(ctx, "/proxy/roles", nil, nil, &list)
	if err != nil {
		reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
	}

	r := roles.NewJSON()
	err = r.UnmarshalJSON(list.Roles)
	if err != nil {
		reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
	}

	return &r, nil
}
