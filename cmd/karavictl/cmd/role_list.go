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
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

var roleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List roles",
	Long:  `List roles`,
	RunE: func(cmd *cobra.Command, args []string) error {
		roles, err := GetRoles()
		if err != nil {
			return fmt.Errorf("Unable to list roles: %v", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%20s", "Role")
		fmt.Fprintf(cmd.OutOrStdout(), "%20s", "Storage System")
		fmt.Fprintf(cmd.OutOrStdout(), "%20s", "Storage Pool")
		fmt.Fprintf(cmd.OutOrStdout(), "%20s", "Quota")
		fmt.Fprintln(cmd.OutOrStdout(), "")

		fmt.Fprintf(cmd.OutOrStdout(), "%20s", "----")
		fmt.Fprintf(cmd.OutOrStdout(), "%20s", "--------------")
		fmt.Fprintf(cmd.OutOrStdout(), "%20s", "------------")
		fmt.Fprintf(cmd.OutOrStdout(), "%20s", "-----")
		fmt.Fprintln(cmd.OutOrStdout(), "")

		for roleName, roleDetails := range roles {
			for _, role := range roleDetails {
				for _, poolQuota := range role.PoolQuotas {
					fmt.Fprintf(cmd.OutOrStdout(), "%20s", roleName)
					fmt.Fprintf(cmd.OutOrStdout(), "%20s", role.StorageSystemID)
					fmt.Fprintf(cmd.OutOrStdout(), "%20s", poolQuota.Pool)
					fmt.Fprintf(cmd.OutOrStdout(), "%20s", humanize.Bytes(uint64(poolQuota.Quota*1024)))
					fmt.Fprintln(cmd.OutOrStdout(), "")
				}
			}
		}
		return nil
	},
}

func init() {
	roleCmd.AddCommand(roleListCmd)
}
