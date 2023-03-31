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
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

// NewTenantDeleteCmd creates a new delete command
func NewTenantDeleteCmd() *cobra.Command {
	tenantDeleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a tenant resource within Karavi",
		Long:  `Deletes a tenant resource within Karavi`,
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

			client, err := CreateHTTPClient(fmt.Sprintf("https://%s", addr), insecure)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			query := url.Values{
				"name": []string{name},
			}

			err = client.Delete(context.Background(), "/proxy/tenant/delete", nil, query, nil)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}
		},
	}

	tenantDeleteCmd.Flags().StringP("name", "n", "", "Tenant name")
	return tenantDeleteCmd
}
