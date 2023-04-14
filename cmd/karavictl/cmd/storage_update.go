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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"karavi-authorization/internal/token"
	"karavi-authorization/internal/web"
	"karavi-authorization/pb"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"
)

// NewStorageUpdateCmd creates a new update command
func NewStorageUpdateCmd() *cobra.Command {
	storageUpdateCmd := &cobra.Command{
		Use:   "update",
		Short: "Update a registered storage system.",
		Long:  `Updates a registered storage system.`,
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
			verifyInput := func(v string) string {
				inputText := flagStringValue(cmd.Flags().GetString(v))
				if strings.TrimSpace(inputText) == "" {
					errAndExit(fmt.Errorf("no input provided: %s", v))
				}
				return inputText
			}

			// Gather the inputs

			addr := flagStringValue(cmd.Flags().GetString("addr"))
			if addr == "" {
				errAndExit(fmt.Errorf("address not specified"))
			}

			insecure := flagBoolValue(cmd.Flags().GetBool("insecure"))

			input := input{
				Type:          verifyInput("type"),
				Endpoint:      verifyInput("endpoint"),
				SystemID:      verifyInput("system-id"),
				User:          verifyInput("user"),
				Password:      flagStringValue(cmd.Flags().GetString("password")),
				ArrayInsecure: flagBoolValue(cmd.Flags().GetBool("array-insecure")),
			}

			// Parse the URL and prepare for a password prompt.
			urlWithUser, err := url.Parse(input.Endpoint)
			if err != nil {
				errAndExit(err)
			}

			urlWithUser.Scheme = "https"
			urlWithUser.User = url.User(input.User)

			// If the password was not provided...
			prompt := fmt.Sprintf("Enter password for %v: ", urlWithUser)
			// If the password was not provided...
			if pf := cmd.Flags().Lookup("password"); !pf.Changed {
				// Get password from stdin
				readPassword(cmd.ErrOrStderr(), prompt, &input.Password)
			}

			// Sanitize the endpoint
			epURL, err := url.Parse(input.Endpoint)
			if err != nil {
				errAndExit(err)
			}
			epURL.Scheme = "https"

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

			if addr != "" {
				err := doStorageUpdateRequest(ctx, addr, input, insecure, cmd, adminTknBody)
				if err != nil {
					errAndExit(err)
				}
			} else {
				k3sCmd := execCommandContext(ctx, K3sPath, "kubectl", "get",
					"--namespace=karavi",
					"--output=json",
					"secret/karavi-storage-secret")

			body := &pb.StorageUpdateRequest{
				StorageType: input.Type,
				Endpoint:    input.Endpoint,
				SystemId:    input.SystemID,
				UserName:    input.User,
				Password:    input.Password,
				Insecure:    input.ArrayInsecure,
			}

			err = client.Patch(context.Background(), "/proxy/storage/update", nil, nil, body, nil)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}
		},
	}

	storageUpdateCmd.Flags().StringP("type", "t", "", "Type of storage system")
	err := storageUpdateCmd.MarkFlagRequired("type")
	if err != nil {
		reportErrorAndExit(JSONOutput, storageUpdateCmd.ErrOrStderr(), err)
	}
	storageUpdateCmd.Flags().StringP("endpoint", "e", "", "Endpoint of REST API gateway")
	err = storageUpdateCmd.MarkFlagRequired("endpoint")
	if err != nil {
		reportErrorAndExit(JSONOutput, storageUpdateCmd.ErrOrStderr(), err)
	}
	storageUpdateCmd.Flags().StringP("system-id", "s", "", "System identifier")
	err = storageUpdateCmd.MarkFlagRequired("system-id")
	if err != nil {
		reportErrorAndExit(JSONOutput, storageUpdateCmd.ErrOrStderr(), err)
	}
	storageUpdateCmd.Flags().StringP("user", "u", "", "Username")
	err = storageUpdateCmd.MarkFlagRequired("user")
	if err != nil {
		reportErrorAndExit(JSONOutput, storageUpdateCmd.ErrOrStderr(), err)
	}
	storageUpdateCmd.Flags().StringP("password", "p", "", "Specify password, or omit to use stdin")
	storageUpdateCmd.Flags().BoolP("array-insecure", "a", false, "Array insecure skip verify")

	return storageUpdateCmd
}

func doStorageUpdateRequest(ctx context.Context, addr string, system input, insecure bool, cmd *cobra.Command, adminTknBody token.AdminToken) error {
	client, err := CreateHTTPClient(fmt.Sprintf("https://%s", addr), insecure)
	if err != nil {
		reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
	}

	body := &pb.StorageUpdateRequest{
		StorageType: system.Type,
		Endpoint:    system.Endpoint,
		SystemId:    system.SystemID,
		UserName:    system.User,
		Password:    system.Password,
		Insecure:    system.ArrayInsecure,
	}

	headers := make(map[string]string)
	headers["Authorization"] = fmt.Sprintf("Bearer %s", adminTknBody.Access)

	err = client.Patch(context.Background(), "/proxy/storage/", headers, nil, body, nil)
	if err != nil {
		var jsonErr web.JSONError
		if errors.As(err, &jsonErr) {
			if jsonErr.Code == http.StatusUnauthorized {
				var adminTknResp pb.RefreshAdminTokenResponse

				headers["Authorization"] = fmt.Sprintf("Bearer %s", adminTknBody.Refresh)
				err = client.Post(context.Background(), "/proxy/refresh-admin", headers, nil, &adminTknBody, &adminTknResp)
				if err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				}
				// retry with refresh token
				headers["Authorization"] = fmt.Sprintf("Bearer %s", adminTknResp.AccessToken)
				err = client.Patch(context.Background(), "/proxy/storage/", headers, nil, body, nil)
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

	return nil
}
