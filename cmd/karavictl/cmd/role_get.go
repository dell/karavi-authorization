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
	"encoding/json"
	"errors"
	"fmt"
	"karavi-authorization/internal/token"
	"karavi-authorization/internal/web"
	"karavi-authorization/pb"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
)

// NewRoleGetCmd creates a new role get command
func NewRoleGetCmd() *cobra.Command {
	roleGetCmd := &cobra.Command{
		Use:   "get",
		Short: "Get CSM role",
		Long:  `Get CSM role`,
		Run: func(cmd *cobra.Command, _ []string) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			addr, err := cmd.Flags().GetString("addr")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}
			if addr == "" {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("address not specified"))
			}

			insecure, err := cmd.Flags().GetBool("insecure")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			roleName, err := cmd.Flags().GetString("name")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			var out map[string]interface{}
			admTknFile, err := cmd.Flags().GetString("admin-token")
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
			adminTknBody := token.AdminToken{
				Refresh: refreshToken,
				Access:  accessToken,
			}

			out, err = doRoleGetRequest(ctx, addr, insecure, roleName, cmd, adminTknBody)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			err = JSONOutput(cmd.OutOrStdout(), out)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("unable to format json output: %v", err))
			}
		},
	}

	roleGetCmd.Flags().StringP("name", "n", "", "role name")
	return roleGetCmd
}

func doRoleGetRequest(ctx context.Context, addr string, insecure bool, name string, cmd *cobra.Command, adminTknBody token.AdminToken) (map[string]interface{}, error) {
	client, err := CreateHTTPClient(fmt.Sprintf("https://%s", addr), insecure)
	if err != nil {
		reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
	}

	query := url.Values{
		"name": []string{name},
	}

	var role pb.RoleGetResponse
	headers := make(map[string]string)
	headers["Authorization"] = fmt.Sprintf("Bearer %s", adminTknBody.Access)
	err = client.Get(ctx, "/proxy/roles", headers, query, &role)
	if err != nil {
		var jsonErr web.JSONError
		if errors.As(err, &jsonErr) {
			if jsonErr.Code == http.StatusUnauthorized {
				// refresh admin token
				var adminTknResp pb.RefreshAdminTokenResponse
				headers["Authorization"] = fmt.Sprintf("Bearer %s", adminTknBody.Refresh)
				err = client.Post(ctx, "/proxy/refresh-admin", headers, nil, &adminTknBody, &adminTknResp)
				if err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				}
				// retry with refresh token
				headers["Authorization"] = fmt.Sprintf("Bearer %s", adminTknResp.AccessToken)
				err = client.Get(ctx, "/proxy/roles", headers, query, &role)
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

	var m map[string]interface{}
	err = json.Unmarshal(role.GetRole(), &m)
	if err != nil {
		reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
	}

	return m, nil
}
