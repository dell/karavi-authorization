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
	"strings"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

// NewStorageListCmd creates a new list command
func NewStorageListCmd() *cobra.Command {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List registered storage systems.",
		Long:  `Lists registered storage systems.`,
		Run: func(cmd *cobra.Command, _ []string) {
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

			storageType := flagStringValue(cmd.Flags().GetString("type"))
			addr := flagStringValue(cmd.Flags().GetString("addr"))
			insecure := flagBoolValue(cmd.Flags().GetBool("insecure"))

			var decodedSystems []byte
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

			decodedSystems, err = doStorageListRequest(context.Background(), addr, insecure, cmd, adminTknBody)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			scrubbed, err := scrubPasswords(decodedSystems)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			m := make(map[string]interface{})
			if err := yaml.Unmarshal(scrubbed, &m); err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			s := filterStorage(storageType, m)
			if err := JSONOutput(cmd.OutOrStdout(), &s); err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}
		},
	}

	listCmd.Flags().StringP("type", "t", "", "Type of storage system")
	return listCmd
}

func filterStorage(storageType string, allStorage map[string]interface{}) interface{} {
	if storageType == "" {
		return allStorage
	}

	output := make(map[string]interface{})
	for i, v := range allStorage {
		if i != storageType {
			continue
		}
		if systems, ok := v.(map[string]interface{}); ok {
			for id, system := range systems {
				output[id] = system
			}
		}
	}
	return output
}

func scrubPasswords(b []byte) ([]byte, error) {
	m := make(map[string]interface{})
	err := yaml.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}

	scrubPasswordsRecurse(m)
	return yaml.Marshal(&m)
}

func scrubPasswordsRecurse(o interface{}) {
	if o == nil {
		return
	}
	m, ok := o.(map[string]interface{})
	if !ok {
		return
	}
	for k := range m {
		if l := strings.ToLower(k); l == "pass" || l == "password" {
			m[k] = "(omitted)"
			continue
		}
		scrubPasswordsRecurse(m[k])
	}
}

func doStorageListRequest(ctx context.Context, addr string, insecure bool, cmd *cobra.Command, adminTknBody token.AdminToken) ([]byte, error) {
	client, err := CreateHTTPClient(fmt.Sprintf("https://%s", addr), insecure)
	if err != nil {
		reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
	}

	var list pb.StorageListResponse
	headers := make(map[string]string)
	headers["Authorization"] = fmt.Sprintf("Bearer %s", adminTknBody.Access)
	err = client.Get(ctx, "/proxy/storage/", headers, nil, &list)
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
				err = client.Get(ctx, "/proxy/storage/", headers, nil, &list)
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

	return list.Storage, nil
}
