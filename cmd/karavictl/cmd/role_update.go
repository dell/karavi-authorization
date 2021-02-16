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
	"os"

	"github.com/spf13/cobra"
)

// roleUpdateCmd represents the update command
var roleUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update one or more Karavi roles",
	Long:  `Updates one or more Karavi roles`,
	Run: func(cmd *cobra.Command, args []string) {
		fromFile, _ := cmd.Flags().GetString("from-file")
		if err := modifyRolesFromFile(fromFile, false); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "failed to update role from file: %+v\n", err)
			os.Exit(1)
		}
		//roleCreateCmd.Run(cmd, append(cmd.Flags().Args(), "isUpdating"))
	},
}

func init() {
	roleCmd.AddCommand(roleUpdateCmd)
	roleUpdateCmd.Flags().StringP("from-file", "f", "", "role data from a file")
}
