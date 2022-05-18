// Copyright © 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"fmt"

	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/pb"

	"github.com/spf13/cobra"
)

// NewRoleGetCmd creates a new role get command
func NewRoleGetCmd() *cobra.Command {
	roleGetCmd := &cobra.Command{
		Use:   "get",
		Short: "Get role",
		Long:  `Get role`,
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

			roleName, err := cmd.Flags().GetString("name")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			var out map[string]interface{}
			if addr != "" {
				out, err = doRoleGetRequest(ctx, addr, insecure, roleName)
				if err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				}
			} else {
				r, err := GetRoles()
				if err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("unable to list roles: %v", err))
				}

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

				if err := json.NewDecoder(&buf).Decode(&out); err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				}
				for k := range out {
					if k != roleName {
						delete(out, k)
					}
				}
			}

			err = JSONOutput(cmd.OutOrStdout(), out)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("unable to format json output: %v", err))
			}
		},
	}

	roleGetCmd.Flags().StringP("name", "n", "", "role name")
	return roleGetCmd
}

func doRoleGetRequest(ctx context.Context, addr string, insecure bool, name string) (map[string]interface{}, error) {
	client, conn, err := CreateRoleServiceClient(addr, insecure)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	resp, err := client.Get(ctx, &pb.RoleGetRequest{Name: name})
	if err != nil {
		return nil, err
	}

	var m map[string]interface{}
	err = json.Unmarshal(resp.Role, &m)
	if err != nil {
		return nil, err
	}

	return m, nil
}
