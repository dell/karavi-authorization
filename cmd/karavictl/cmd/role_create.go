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
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// roleCreateCmd represents the role command
var roleCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create one or more Karavi roles",
	Long:  `Creates one or more Karavi roles`,
	Run: func(cmd *cobra.Command, args []string) {
		// kg create configmap volumes-delete -f ./volumes_delete.rego -n karavi --dry-run=client -o yaml | kg apply -f -
		fromFile, _ := cmd.Flags().GetString("from-file")
		if err := modifyRolesFromFile(fromFile, true); err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "failed to create role from file: %+v\n", err)
			os.Exit(1)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "Successfully added role")
		}
	},
}

func init() {
	if _testing {
		return
	}
	roleCmd.AddCommand(roleCreateCmd)
	roleCreateCmd.Flags().StringP("from-file", "f", "", "role data from a file")
}

func modifyRolesFromFile(path string, isCreating bool) error {
	if path == "" {
		return errors.New("missing file argument")
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	/*

		package karavi.common

		default roles = {}
		roles = {
			"CSIBronze": {
				"pools": ["bronze"],
				"quota": 9000000
			},
			"CSISilver": {
				"pools": ["silver"],
				"quota": 16000000
			},
			"CSIGold": {
				"pools": ["gold"],
				"quota": 32000000
			}
		}
	*/

	// TODO
	// isCreating = false
	// 1. Open JSON or YAML into as struct
	// 2. if isCreating and role is already exist, return error
	// 3. if !isCreating and role

	createCmd := exec.Command("k3s",
		"kubectl",
		"create",
		"configmap",
		"common",
		"--from-file="+path,
		"-n", "karavi",
		"--dry-run=client",
		"-o", "yaml")
	applyCmd := exec.Command("k3s", "kubectl", "apply", "-f", "-")

	pr, pw := io.Pipe()
	createCmd.Stdout = pw
	applyCmd.Stdin = pr
	applyCmd.Stdout = os.Stdout

	if err := createCmd.Start(); err != nil {
		return fmt.Errorf("create: %w", err)
	}
	if err := applyCmd.Start(); err != nil {
		return fmt.Errorf("apply: %w", err)
	}

	eg := errgroup.Group{}
	eg.Go(func() error {
		defer pw.Close()
		if err := createCmd.Wait(); err != nil {
			return fmt.Errorf("create wait: %w", err)
		}
		return nil
	})
	if err := applyCmd.Wait(); err != nil {
		return fmt.Errorf("apply wait: %w", err)
	}
	if err := eg.Wait(); err != nil {
		return err
	}
	return nil
}
