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
	"karavi-authorization/internal/proxy"
	"karavi-authorization/internal/token"
	"karavi-authorization/internal/web"
	"karavi-authorization/pb"
	"net/http"
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

			admTknFile, err := cmd.Flags().GetString("admin_token")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}
			if admTknFile == "" {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), errors.New("specify token file"))
			}
			accessToken, refreshToken, err := ReadAccessAdminToken(admTknFile)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			headers := make(map[string]string)
			headers["Authorization"] = fmt.Sprintf("Bearer %s", accessToken)

			err = client.Patch(context.Background(), "/proxy/tenant/revoke", headers, nil, &body, nil)
			if err != nil {
				var jsonErr web.JSONError
				if errors.As(err, &jsonErr) {
					if jsonErr.Code == http.StatusUnauthorized {
						// expired token, refresh admin token
						adminTknBody := token.AdminToken{
							Refresh: refreshToken,
							Access:  accessToken,
						}
						var adminTknResp pb.RefreshAdminTokenResponse

						headers["Authorization"] = fmt.Sprintf("Bearer %s", refreshToken)
						err = client.Post(context.Background(), "/proxy/refresh-admin", headers, nil, &adminTknBody, &adminTknResp)
						if err != nil {
							reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
						}

						// retry with refresh token
						headers["Authorization"] = fmt.Sprintf("Bearer %s", adminTknResp.AccessToken)
						err = client.Patch(context.Background(), "/proxy/tenant/revoke", headers, nil, &body, nil)
						if err != nil {
							reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
						}
					} else {
						reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
					}
				} else {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				}
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
