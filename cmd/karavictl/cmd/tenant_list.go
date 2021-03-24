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

// tenantListCmd represents the list command
var tenantListCmd = &cobra.Command{
	Use:   "list",
	Short: "List a tenant resource within Karavi",
	Long:  `Lists tenant resources within Karavi`,
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

		list, err := tenantClient.ListTenant(context.Background(), &pb.ListTenantRequest{})
		if err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
		}

		if err := JSONOutput(cmd.OutOrStdout(), &list); err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
		}
	},
}

func init() {
	tenantCmd.AddCommand(tenantListCmd)
}
