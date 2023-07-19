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

// NewRoleBindingCmd creates a new rolebinding command
func NewRoleBindingCmd() *cobra.Command {
	rolebindingCmd := &cobra.Command{
		Use:   "rolebinding",
		Short: "Manage role bindings",
		Long:  `Management for role bindings`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmd.Usage(); err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("error: %+v", err))
			}
			os.Exit(1)
		},
	}

	rolebindingCmd.PersistentFlags().StringP("admin-token", "f", "", "Path to admin token file; required")
	rolebindingCmd.PersistentFlags().String("addr", "", "Address of the CSM Authorization Proxy Server; required")
	rolebindingCmd.PersistentFlags().Bool("insecure", false, "Skip certificate validation of the CSM Authorization Proxy Server")

	err := rolebindingCmd.MarkPersistentFlagRequired("admin-token")
	if err != nil {
		reportErrorAndExit(JSONOutput, rolebindingCmd.ErrOrStderr(), err)
	}

	err = rolebindingCmd.MarkPersistentFlagRequired("addr")
	if err != nil {
		reportErrorAndExit(JSONOutput, rolebindingCmd.ErrOrStderr(), err)
	}

	rolebindingCmd.AddCommand(NewCreateRoleBindingCmd())
	rolebindingCmd.AddCommand(NewDeleteRoleBindingCmd())
	return rolebindingCmd
}
