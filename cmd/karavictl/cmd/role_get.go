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
	"errors"
	"fmt"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

var roleGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Get role",
	Long:  `Get role`,
	RunE: func(cmd *cobra.Command, args []string) error {

		if len(args) == 0 {
			return errors.New("role name is required")
		}

		if len(args) > 1 {
			return errors.New("expects single argument")
		}

		roles, err := GetRoles()
		if err != nil {
			return fmt.Errorf("Unable to list roles: %v", err)
		}

		roleName := args[0]

		if _, ok := roles[roleName]; !ok {
			return fmt.Errorf("role %s does not exist", roleName)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Role: %s\n", roleName)

		for _, role := range roles[roleName] {
			fmt.Fprintf(cmd.OutOrStdout(), "  Storage System: %s\n", role.StorageSystemID)
			for _, poolQuota := range role.PoolQuotas {
				fmt.Fprintf(cmd.OutOrStdout(), "    Storage Pool: %s\n", poolQuota.Pool)
				fmt.Fprintf(cmd.OutOrStdout(), "    Quota: %s\n", humanize.Bytes(uint64(poolQuota.Quota*1024)))
			}
		}

		return nil
	},
}

func init() {
	roleCmd.AddCommand(roleGetCmd)
}
