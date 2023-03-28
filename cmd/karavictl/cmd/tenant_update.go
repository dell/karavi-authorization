// Copyright © 2023 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"karavi-authorization/internal/proxy"
	"strings"

	"github.com/spf13/cobra"
)

// NewTenantUpdateCmd creates a new update command for tenant
func NewTenantUpdateCmd() *cobra.Command {
	tenantUpdateCmd := &cobra.Command{
		Use:              "update",
		TraverseChildren: true,
		Short:            "Update a tenant resource within CSM Authorization",
		Long:             `Updates a tenant resource within CSM Authorization`,
		Run: func(cmd *cobra.Command, args []string) {
			addr, err := cmd.Flags().GetString("addr")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			insecure, err := cmd.Flags().GetBool("insecure")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			name, err := cmd.Flags().GetString("name")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}
			if strings.TrimSpace(name) == "" {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), errors.New("empty name not allowed"))
			}

			approveSdc, err := cmd.Flags().GetBool("approvesdc")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			client, err := CreateHttpClient(fmt.Sprintf("https://%s", addr), insecure)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			body := proxy.CreateTenantBody{
				Tenant:     name,
				ApproveSdc: approveSdc,
			}
			err = client.Patch(context.Background(), "/proxy/tenant/update", nil, nil, body, nil)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}
		},
	}

	tenantUpdateCmd.Flags().StringP("name", "n", "", "Tenant name")
	tenantUpdateCmd.Flags().Bool("approvesdc", true, "To allow/deny SDC approval requests")
	return tenantUpdateCmd
}
