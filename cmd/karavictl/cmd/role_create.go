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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/pb"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

const roleFlagSize = 5

// NewRoleCreateCmd creates a new role command
func NewRoleCreateCmd() *cobra.Command {
	roleCreateCmd := &cobra.Command{
		Use:   "create",
		Short: "Create one or more Karavi roles",
		Long:  `Creates one or more Karavi roles`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			outFormat := "failed to create role: %+v\n"

			// parse flags

			roleFlags, err := cmd.Flags().GetStringSlice("role")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
			}

			if len(roleFlags) == 0 {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, errors.New("no input")))
			}

			addr, err := cmd.Flags().GetString("addr")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
			}

			insecure, err := cmd.Flags().GetBool("insecure")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			// process role flag

			var newRole *roles.Instance
			var rff roles.JSON
			for _, v := range roleFlags {
				t := strings.Split(v, "=")
				if len(t) < roleFlagSize {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, errors.New("role does not have enough arguments")))
				}
				newRole, err = roles.NewInstance(t[0], t[1:]...)
				if err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
				}

				err = rff.Add(newRole)
				if err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
				}
			}

			if addr != "" {
				// if addr flag is specified, make a grpc request
				for _, roleInstance := range rff.Instances() {
					if err = doRoleCreateRequest(ctx, addr, insecure, roleInstance); err != nil {
						reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
					}
				}
			} else {
				// modify the k3s configuration
				existingRoles, err := GetRoles()
				if err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
				}

				adding := rff.Instances()
				var dups []string
				for _, role := range adding {
					if existingRoles.Get(role.RoleKey) != nil {
						var dup bool
						if dup {
							dups = append(dups, role.Name)
						}
					}
				}
				if len(dups) > 0 {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("duplicates %+v", dups))
				}

				for _, role := range adding {
					err := validateRole(ctx, role)
					if err != nil {
						err = fmt.Errorf("%s failed validation: %+v", role.Name, err)
						reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
					}

					err = existingRoles.Add(role)
					if err != nil {
						reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
					}
				}
				if err = modifyK3sCommonConfigMap(existingRoles); err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
				}
			}
		},
	}

	roleCreateCmd.Flags().StringSlice("role", []string{}, "role in the form <name>=<type>=<id>=<pool>=<quota>")
	return roleCreateCmd
}

func modifyK3sCommonConfigMap(roles *roles.JSON) error {
	var err error

	data, err := json.MarshalIndent(&roles, "", "  ")
	if err != nil {
		return err
	}

	stdFormat := (`package karavi.common
default roles = {}
roles = ` + string(data))

	createCmd := execCommandContext(context.Background(), K3sPath,
		"kubectl",
		"create",
		"configmap",
		"common",
		"--from-literal=common.rego="+stdFormat,
		"-n", "karavi",
		"--dry-run=client",
		"-o", "yaml")
	applyCmd := execCommandContext(context.Background(), K3sPath, "kubectl", "apply", "-f", "-")

	pr, pw := io.Pipe()
	createCmd.Stdout = pw
	applyCmd.Stdin = pr
	applyCmd.Stdout = io.Discard

	if err := createCmd.Start(); err != nil {
		return fmt.Errorf("create: %w", err)
	}
	if err := applyCmd.Start(); err != nil {
		return fmt.Errorf("apply: %w", err)
	}

	eg := errgroup.Group{}
	eg.Go(func() error {
		defer pw.Close()
		if err := createCmd.Wait(); err != nil {
			return fmt.Errorf("create wait: %w", err)
		}
		return nil
	})
	if err := applyCmd.Wait(); err != nil {
		return fmt.Errorf("apply wait: %w", err)
	}
	if err := eg.Wait(); err != nil {
		return err
	}
	return nil
}

func doRoleCreateRequest(ctx context.Context, addr string, insecure bool, role *roles.Instance) error {
	client, conn, err := CreateRoleServiceClient(addr, insecure)
	if err != nil {
		return err
	}
	defer conn.Close()

	req := &pb.RoleCreateRequest{
		Name:        role.Name,
		StorageType: role.SystemType,
		SystemId:    role.SystemID,
		Pool:        role.Pool,
		Quota:       strconv.Itoa(role.Quota),
	}

	_, err = client.Create(ctx, req)
	if err != nil {
		return err
	}

	return nil
}
