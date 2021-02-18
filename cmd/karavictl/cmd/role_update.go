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

// roleUpdateCmd represents the update command
var roleUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update one or more Karavi roles",
	Long:  `Updates one or more Karavi roles`,
	RunE: func(cmd *cobra.Command, args []string) error {
		outFormat := "failed to update role from file: %+v\n"

		fromFile, err := cmd.Flags().GetString("from-file")
		if err != nil {
			return fmt.Errorf(outFormat, err)
		}

		roles, err := getRolesFromFile(fromFile)
		if err != nil {
			return fmt.Errorf(outFormat, err)
		}

		existingRoles, err := GetRoles()
		if err != nil {
			return fmt.Errorf(outFormat, err)
		}

		for name, rls := range roles {
			if _, ok := existingRoles[name]; !ok {
				err = fmt.Errorf("%s role does not exist. Try create command", name)
				return fmt.Errorf(outFormat, err)
			}
			for i := range rls {
				err = validateRole(rls[i])
				if err != nil {
					err = fmt.Errorf("%s failed validation: %+v", name, err)
					return fmt.Errorf(outFormat, err)
				}
			}
			existingRoles[name] = rls
		}

		if err = modifyCommonConfigMap(existingRoles); err != nil {
			return fmt.Errorf(outFormat, err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Role was successfully updated")
		return nil
	},
}

func init() {
	roleCmd.AddCommand(roleUpdateCmd)
	roleUpdateCmd.Flags().StringP("from-file", "f", "", "role data from a file")
}
