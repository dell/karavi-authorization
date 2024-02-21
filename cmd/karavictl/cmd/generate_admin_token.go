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
	"karavi-authorization/internal/token"
	"karavi-authorization/internal/token/jwx"
	"karavi-authorization/pb"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

// NewAdminTokenCmd creates a new token command
func NewAdminTokenCmd() *cobra.Command {
	adminTokenCmd := &cobra.Command{
		Use:   "token",
		Short: "Generate tokens for an admin.",
		Long:  `Generates tokens for an admin.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			adminName, err := cmd.Flags().GetString("name")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				return err
			}
			if strings.TrimSpace(adminName) == "" {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), errors.New("empty admin name not allowed"))
			}

			refExpTime, err := cmd.Flags().GetDuration("refresh-token-expiration")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				return err
			}

			accExpTime, err := cmd.Flags().GetDuration("access-token-expiration")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				return err
			}

			secret, err := cmd.Flags().GetString("jwt-signing-secret")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				return err
			}

			// If the password was not provided...
			prompt := fmt.Sprintf("Enter JWT Signing Secret: ")
			// If the password was not provided...
			if pf := cmd.Flags().Lookup("jwt-signing-secret"); !pf.Changed {
				// Get password from stdin
				readPassword(cmd.ErrOrStderr(), prompt, &secret)
			}

			resp, err := jwx.GenerateAdminToken(context.Background(), &pb.GenerateAdminTokenRequest{
				AdminName:         adminName,
				JWTSigningSecret:  secret,
				RefreshExpiration: int64(refExpTime),
				AccessExpiration:  int64(accExpTime),
			})
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				return nil
			}

			admintoken := token.AdminToken{}
			if err := yaml.Unmarshal(resp.Token, &admintoken); err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				return nil
			}
			err = JSONOutput(cmd.OutOrStdout(), &admintoken)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				return nil
			}
			return nil
		},
	}

	adminTokenCmd.Flags().StringP("name", "n", "", "Admin name")
	adminTokenCmd.Flags().StringP("jwt-signing-secret", "s", "", "Specify JWT signing secret, or omit to use stdin")
	adminTokenCmd.Flags().Duration("refresh-token-expiration", 30*24*time.Hour, "Expiration time of the refresh token, e.g. 48h")
	adminTokenCmd.Flags().Duration("access-token-expiration", time.Minute, "Expiration time of the access token, e.g. 1m30s")
	return adminTokenCmd
}
