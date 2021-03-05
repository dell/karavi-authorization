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

	"github.com/spf13/cobra"
)

var roleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List roles",
	Long:  `List roles`,
	Run: func(cmd *cobra.Command, args []string) {
		roles, err := GetRoles()
		if err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("unable to list roles: %v", err))
		}

		JSONOutput(cmd.OutOrStdout(), &roles)
	},
}

func init() {
	roleCmd.AddCommand(roleListCmd)
}
