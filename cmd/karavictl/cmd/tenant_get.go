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
	"errors"
	"fmt"
	"karavi-authorization/pb"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

// NewTenantGetCmd creates a new get command
func NewTenantGetCmd() *cobra.Command {
	tenantGetCmd := &cobra.Command{
		Use:   "get",
		Short: "Get a tenant resource within Karavi",
		Long:  `Gets a tenant resource within Karavi`,
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

			client, err := CreateHttpClient(fmt.Sprintf("https://%s", addr), insecure)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			query := url.Values{
				"name": []string{name},
			}

			var tenant pb.Tenant
			err = client.Get(context.Background(), "/proxy/tenant/get", nil, query, &tenant)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			err = jsonOutputEmitEmpty(cmd.ErrOrStderr(), &tenant)
			if err != nil {
				reportErrorAndExit(jsonOutput, cmd.ErrOrStderr(), err)
			}
		},
	}

	tenantGetCmd.Flags().StringP("name", "n", "", "Tenant name")
	return tenantGetCmd
}
