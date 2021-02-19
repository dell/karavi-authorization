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
		switch {
		case fromFile != "":
			if err := updateRolesFromFile(fromFile); err != nil {
				fmt.Fprintf(os.Stderr, "failed to create role from file: %+v\n", err)
				os.Exit(1)
			}
		default:
			fmt.Fprintln(os.Stderr, "missing file argument")
			os.Exit(1)
		}
	},
}

func init() {
	roleCmd.AddCommand(roleCreateCmd)

	roleCreateCmd.Flags().StringP("from-file", "f", "", "role data from a file")
}

func updateRolesFromFile(path string) error {
	path, err := filepath.Abs(path)
	if err != nil {
		return err
	}

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
