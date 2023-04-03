// Copyright © 2021-2023 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"karavi-authorization/internal/proxy"
	"os"

	"github.com/spf13/cobra"
)

// NewTenantRevokeCmd creates a new revoke command
func NewTenantRevokeCmd() *cobra.Command {
	tenantRevokeCmd := &cobra.Command{
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

			tenantName, err := cmd.Flags().GetString("name")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}
			isCancel, err := cmd.Flags().GetBool("cancel")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			client, err := CreateHTTPClient(fmt.Sprintf("https://%s", addr), insecure)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			body := proxy.TenantRevokeBody{
				Tenant: tenantName,
				Cancel: isCancel,
			}
			err = client.Patch(context.Background(), "/proxy/tenant/revoke", nil, nil, &body, nil)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}
		},
	}

	tenantRevokeCmd.Flags().StringP("name", "n", "", "Tenant name")
	err := tenantRevokeCmd.MarkFlagRequired("name")
	if err != nil {
		reportErrorAndExit(JSONOutput, os.Stderr, err)
	}
	tenantRevokeCmd.Flags().BoolP("cancel", "c", false, "Cancel a previous tenant revocation")
	return tenantRevokeCmd
}
