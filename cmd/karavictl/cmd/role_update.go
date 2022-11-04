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
	"errors"
	"fmt"
	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/pb"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// NewRoleUpdateCmd creates a new update command
func NewRoleUpdateCmd() *cobra.Command {
	roleUpdateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update the quota of one or more CSM roles",
		Long:  `Updates the quota of one or more CSM roles`,
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

			outFormat := "failed to update role: %+v\n"

			roleFlags, err := cmd.Flags().GetStringSlice("role")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
			}

			var rff roles.JSON
			if len(roleFlags) == 0 {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, errors.New("no input")))
			}

			for _, v := range roleFlags {
				t := strings.Split(v, "=")

				newrole, err := roles.NewInstance(t[0], t[1:]...)
				if err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
				}
				err = rff.Add(newrole)
				if err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
				}
			}

			if addr != "" {
				// if addr flag is specified, make a grpc request
				for _, roleInstance := range rff.Instances() {
					if err = doRoleUpdateRequest(ctx, addr, insecure, roleInstance); err != nil {
						reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
					}
				}
			} else {
				existingRoles, err := GetRoles()
				if err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
				}

				for _, rls := range rff.Instances() {
					if existingRoles.Get(rls.RoleKey) == nil {
						reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, "only role quota can be updated"))
					}

					err = validateRole(ctx, rls)
					if err != nil {
						reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("%s failed validation: %+v", rls.Name, err))
					}

					err = existingRoles.Remove(rls)
					if err != nil {
						reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("%s failed to update: %+v", rls.Name, err))
					}
					err := existingRoles.Add(rls)
					if err != nil {
						reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("adding %s failed: %+v", rls.Name, err))
					}
				}
				if err = modifyK3sCommonConfigMap(existingRoles); err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
				}
			}
		},
	}
	roleUpdateCmd.Flags().StringSlice("role", []string{}, "role in the form <name>=<type>=<id>=<pool>=<quota>")
	return roleUpdateCmd
}

func doRoleUpdateRequest(ctx context.Context, addr string, insecure bool, role *roles.Instance) error {
	client, conn, err := CreateRoleServiceClient(addr, insecure)
	if err != nil {
		return err
	}
	defer conn.Close()

	req := &pb.RoleUpdateRequest{
		Name:        role.Name,
		StorageType: role.SystemType,
		SystemId:    role.SystemID,
		Pool:        role.Pool,
		Quota:       strconv.Itoa(role.Quota),
	}

	_, err = client.Update(ctx, req)
	if err != nil {
		return err
	}

	return nil
}
