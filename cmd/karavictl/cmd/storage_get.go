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
	"karavi-authorization/internal/token"
	"karavi-authorization/internal/web"
	"karavi-authorization/pb"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

var (
	errSystemTypeNotSpecified = errors.New("system type not specified")
	errSystemIDNotSpecified   = errors.New("system id not specified")
)

// NewStorageGetCmd creates a new get command
func NewStorageGetCmd() *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get details on a registered storage system.",
		Long:  `Gets details on a registered storage system.`,
		Run: func(cmd *cobra.Command, args []string) {
			errAndExit := func(err error) {
				fmt.Fprintf(cmd.ErrOrStderr(), "error: %+v\n", err)
				osExit(1)
			}

			// Convenience functions for ignoring errors whilst
			// getting flag values.
			flagStringValue := func(v string, err error) string {
				if err != nil {
					errAndExit(err)
				}
				return v
			}

			flagBoolValue := func(v bool, err error) bool {
				if err != nil {
					errAndExit(err)
				}
				return v
			}

			storType := flagStringValue(cmd.Flags().GetString("type"))
			if storType == "" {
				errAndExit(errSystemTypeNotSpecified)
			}

			sysID := flagStringValue(cmd.Flags().GetString("system-id"))
			if sysID == "" {
				errAndExit(errSystemIDNotSpecified)
			}

			addr := flagStringValue(cmd.Flags().GetString("addr"))
			if addr == "" {
				errAndExit(fmt.Errorf("address not specified"))
			}

			insecure := flagBoolValue(cmd.Flags().GetBool("insecure"))
			var decodedSystem []byte
			var err error
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

			decodedSystem, err = doStorageGetRequest(context.Background(), addr, storType, sysID, insecure, cmd, adminTknBody)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			m := make(map[string]interface{})
			if err := yaml.Unmarshal(decodedSystem, &m); err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			err = JSONOutput(cmd.OutOrStdout(), m)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("unable to format json output: %v", err))
			}
		},
	}

	getCmd.Flags().StringP("type", "t", "", "Type of storage system")
	getCmd.Flags().StringP("system-id", "s", "", "System identifier")
	return getCmd
}

func doStorageGetRequest(ctx context.Context, addr string, storageType string, systemID string, insecure bool, cmd *cobra.Command, adminTknBody token.AdminToken) ([]byte, error) {
	client, err := CreateHTTPClient(fmt.Sprintf("https://%s", addr), insecure)
	if err != nil {
		reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
	}

	query := url.Values{
		"StorageType": []string{storageType},
		"SystemId":    []string{systemID},
	}

	var resp pb.StorageGetResponse
	headers := make(map[string]string)
	headers["Authorization"] = fmt.Sprintf("Bearer %s", adminTknBody.Access)

	err = client.Get(ctx, "/proxy/storage/", headers, query, &resp)
	if err != nil {
		var jsonErr web.JSONError
		if errors.As(err, &jsonErr) {
			if jsonErr.Code == http.StatusUnauthorized {
				var adminTknResp pb.RefreshAdminTokenResponse

				headers["Authorization"] = fmt.Sprintf("Bearer %s", adminTknBody.Refresh)
				err = client.Post(ctx, "/proxy/refresh-admin", headers, nil, &adminTknBody, &adminTknResp)
				if err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				}
				// retry with refresh token
				headers["Authorization"] = fmt.Sprintf("Bearer %s", adminTknResp.AccessToken)
				err = client.Get(ctx, "/proxy/storage/", headers, query, &resp)
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

	return resp.Storage, nil
}
