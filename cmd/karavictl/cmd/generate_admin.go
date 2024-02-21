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
	"os"

	"github.com/spf13/cobra"
)

// NewAdminCmd creates a new admin token command
func NewAdminCmd() *cobra.Command {
	adminCmd := &cobra.Command{
		Use:   "admin",
		Short: "Generate admin token for use with CSM Authorization",
		Long:  `Generate admin token for use with CSM Authorization`,
		Run: func(cmd *cobra.Command, _ []string) {
			err := cmd.Usage()
			if err != nil {
				reportErrorAndExit(JSONOutput, os.Stderr, err)
			}
			os.Exit(1)
		},
	}

	adminCmd.AddCommand(NewAdminTokenCmd())
	return adminCmd
}
