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
	"fmt"
	"karavi-authorization/internal/proxy"
	"karavi-authorization/pb"
	"time"

	"github.com/spf13/cobra"
)

// NewGenerateTokenCmd creates a new token command
func NewGenerateTokenCmd() *cobra.Command {
	generateTokenCmd := &cobra.Command{
		Use:   "token",
		Short: "Generate tokens for a tenant.",
		Long:  `Generates tokens for a tenant.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			addr, err := cmd.Flags().GetString("addr")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				return err
			}

			insecure, err := cmd.Flags().GetBool("insecure")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				return err
			}

			tenant, err := cmd.Flags().GetString("tenant")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				return err
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

			client, err := CreateHTTPClient(fmt.Sprintf("https://%s", addr), insecure)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			body := proxy.GenerateTokenBody{
				Tenant:          tenant,
				AccessTokenTTL:  accExpTime.String(),
				RefreshTokenTTL: refExpTime.String(),
			}
			var resp pb.GenerateTokenResponse
			err = client.Post(context.Background(), "/proxy/tenant/token", nil, nil, &body, &resp)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			err = JSONOutput(cmd.OutOrStdout(), &resp)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				return nil
			}
			return nil
		},
	}

	generateTokenCmd.Flags().String("addr", "localhost:443", "Address of the server")
	generateTokenCmd.Flags().Duration("refresh-token-expiration", 30*24*time.Hour, "Expiration time of the refresh token, e.g. 48h")
	generateTokenCmd.Flags().Duration("access-token-expiration", time.Minute, "Expiration time of the access token, e.g. 1m30s")
	generateTokenCmd.Flags().StringP("tenant", "t", "", "Tenant name")
	generateTokenCmd.Flags().Bool("insecure", false, "For insecure connections")
	if err := generateTokenCmd.MarkFlagRequired("tenant"); err != nil {
		reportErrorAndExit(JSONOutput, generateTokenCmd.ErrOrStderr(), err)
	}
	return generateTokenCmd
}
