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
	"karavi-authorization/pb"
	"time"

	"github.com/spf13/cobra"
)

// generateTokenCmd represents the token command
var generateTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Generate tokens for a tenant.",
	Long:  `Generates tokens for a tenant.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		addr, err := cmd.Flags().GetString("addr")
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

		tenantClient, conn, err := CreateTenantServiceClient(addr)
		if err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			return err
		}
		defer conn.Close()

		resp, err := tenantClient.GenerateToken(context.Background(), &pb.GenerateTokenRequest{
			TenantName:      tenant,
			RefreshTokenTTL: int64(refExpTime),
			AccessTokenTTL:  int64(accExpTime),
		})
		if err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			return nil
		}

		err = JSONOutput(cmd.OutOrStdout(), &resp)
		if err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			return nil
		}
		return nil
	},
}

func init() {
	generateCmd.AddCommand(generateTokenCmd)

	generateTokenCmd.Flags().String("addr", "localhost:443", "Address of the server")
	generateTokenCmd.Flags().Duration("refresh-token-expiration", 30*24*time.Hour, "Time until the refresh token is set to expire")
	generateTokenCmd.Flags().Duration("access-token-expiration", time.Minute, "Time until the access token is set to expire")
	generateTokenCmd.Flags().StringP("tenant", "t", "", "Tenant name")
	if err := generateTokenCmd.MarkFlagRequired("tenant"); err != nil {
		panic(err)
	}
}
