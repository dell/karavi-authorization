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
	"karavi-authorization/pb"

	"github.com/spf13/cobra"
)

// createRoleBindingCmd represents the rolebinding command
var createRoleBindingCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a rolebinding between role and tenant",
	Long:  `Creates a rolebinding between role and tenant`,
	Run: func(cmd *cobra.Command, args []string) {
		addr, err := cmd.Flags().GetString("addr")
		if err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
		}

		insecure, err := cmd.Flags().GetBool("insecure")
		if err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
		}

		tenantClient, conn, err := CreateTenantServiceClient(addr, insecure)
		if err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
		}
		defer conn.Close()

		tenant, err := cmd.Flags().GetString("tenant")
		if err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
		}
		role, err := cmd.Flags().GetString("role")
		if err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
		}

		_, err = tenantClient.BindRole(context.Background(), &pb.BindRoleRequest{
			TenantName: tenant,
			RoleName:   role,
		})
		if err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
		}
	},
}

func init() {
	rolebindingCmd.AddCommand(createRoleBindingCmd)

	createRoleBindingCmd.Flags().StringP("tenant", "t", "", "Tenant name")
	createRoleBindingCmd.Flags().StringP("role", "r", "", "Role name")
}
