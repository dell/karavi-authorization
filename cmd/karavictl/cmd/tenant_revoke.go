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

// tenantRevokeCmd represents the revoke command
var tenantRevokeCmd = &cobra.Command{
	Use:   "revoke",
	Short: "Revoke tenant access to Karavi Authorization.",
	Long:  `Revokes tenant access to Karavi Authorization.`,
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

		tenantName, err := cmd.Flags().GetString("name")
		if err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
		}
		isCancel, err := cmd.Flags().GetBool("cancel")
		if err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
		}

		var resp interface{}
		switch {
		case isCancel:
			resp, err = tenantClient.CancelRevokeTenant(context.Background(), &pb.CancelRevokeTenantRequest{
				TenantName: tenantName,
			})
		default:
			resp, err = tenantClient.RevokeTenant(context.Background(), &pb.RevokeTenantRequest{
				TenantName: tenantName,
			})
		}
		if err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
		}

		if err := JSONOutput(cmd.OutOrStdout(), &resp); err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
		}
	},
}

func init() {
	tenantCmd.AddCommand(tenantRevokeCmd)

	tenantRevokeCmd.Flags().StringP("name", "n", "", "Tenant name")
	tenantRevokeCmd.MarkFlagRequired("name")
	tenantRevokeCmd.Flags().BoolP("cancel", "c", false, "Cancel a previous tenant revocation")
}
