/*
Copyright © 2020 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// createTenantCmd represents the tenant command
var createTenantCmd = &cobra.Command{
	Use:   "tenant",
	Short: "Create a tenant resource within Karavi.",
	Long:  `Creates a tenant resource within Karavi.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Creating a tenant resource")
	},
}

func init() {
	createCmd.AddCommand(createTenantCmd)
}