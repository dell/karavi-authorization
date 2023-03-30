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
	"fmt"
	"karavi-authorization/internal/token"
	"karavi-authorization/internal/token/jwx"
	"time"

	"github.com/spf13/cobra"
)

var (
	refreshTokenTTL int64
	accessTokenTTL  int64
	password        string
)

// NewGenerateAdminTokenCmd creates a new token command
func NewAdminTokenCmd() *cobra.Command {
	adminTokenCmd := &cobra.Command{
		Use:   "token",
		Short: "Generate tokens for a admin.",
		Long:  `Generates tokens for a admin.`,
		RunE: func(cmd *cobra.Command, args []string) error {

			adminName, err := cmd.Flags().GetString("name")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				return err
			}
			refExpTime, err := cmd.Flags().GetDuration("refresh-token-expiration")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				return err
			}
			if refExpTime <= 0 {
				refreshTokenTTL = int64(24 * time.Hour)
			}

			accExpTime, err := cmd.Flags().GetDuration("access-token-expiration")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				return err
			}
			if accExpTime <= 0 {
				accessTokenTTL = int64(30 * time.Minute)
			}

			password, err := cmd.Flags().GetString("password")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				return err
			}

			// If the password was not provided...
			prompt := fmt.Sprintf("Enter password: ")
			// If the password was not provided...
			if pf := cmd.Flags().Lookup("password"); !pf.Changed {
				// Get password from stdin
				readPassword(cmd.ErrOrStderr(), prompt, &password)
			}

			tm := jwx.NewTokenManager(jwx.HS256)
			// Generate the token.
			s, err := token.CreateAdminSecret(tm, token.Config{
				AdminName:         adminName,
				Subject:           "admin",
				Roles:             nil,
				JWTSigningSecret:  password,
				RefreshExpiration: time.Duration(refreshTokenTTL),
				AccessExpiration:  time.Duration(accessTokenTTL),
			})
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				return nil
			}

			err = JSONOutput(cmd.OutOrStdout(), &s)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				return nil
			}
			return nil
		},
	}

	adminTokenCmd.Flags().StringP("name", "t", "", "Admin name")
	adminTokenCmd.Flags().StringP("password", "p", "", "Specify password, or omit to use stdin")
	adminTokenCmd.Flags().Duration("refresh-token-expiration", 30*24*time.Hour, "Expiration time of the refresh token, e.g. 48h")
	adminTokenCmd.Flags().Duration("access-token-expiration", time.Minute, "Expiration time of the access token, e.g. 1m30s")
	if err := adminTokenCmd.MarkFlagRequired("name"); err != nil {
		reportErrorAndExit(JSONOutput, adminTokenCmd.ErrOrStderr(), err)
	}
	return adminTokenCmd
}
