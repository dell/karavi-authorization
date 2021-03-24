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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"karavi-authorization/internal/roles"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// roleCreateCmd represents the role command
var roleCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create one or more Karavi roles",
	Long:  `Creates one or more Karavi roles`,
	Run: func(cmd *cobra.Command, args []string) {
		outFormat := "failed to create role: %+v\n"

		roleFlags, err := cmd.Flags().GetStringSlice("role")
		if err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
		}

		if len(roleFlags) == 0 {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, errors.New("no input")))
		}

		var rff roles.JSON
		for _, v := range roleFlags {
			t := strings.Split(v, "=")
			err = rff.Add(roles.NewInstance(t[0], t[1:]...))
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
			}
		}

		existingRoles, err := GetRoles()
		if err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
		}

		adding := rff.Instances()
		var dups []string
		for _, role := range adding {
			if existingRoles.Get(role.RoleKey) != nil {
				var dup bool
				if dup {
					dups = append(dups, role.Name)
				}
			}
		}
		if len(dups) > 0 {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("duplicates %+v", dups))
		}

		for _, role := range adding {
			err := validateRole(role)
			if err != nil {
				err = fmt.Errorf("%s failed validation: %+v", role.Name, err)
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
			}

			err = existingRoles.Add(role)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
			}
		}

		if err = modifyCommonConfigMap(existingRoles); err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
		}
	},
}

func init() {
	roleCmd.AddCommand(roleCreateCmd)
	roleCreateCmd.Flags().StringP("from-file", "f", "", "role data from a file")
	roleCreateCmd.Flags().StringSlice("role", []string{}, "role in the form <name>=<type>=<id>=<pool>=<quota>")
}

func modifyCommonConfigMap(roles roles.JSON) error {
	var err error

	data, err := json.MarshalIndent(&roles, "", "  ")
	if err != nil {
		return err
	}

	stdFormat := (`package karavi.common
default roles = {}
roles = ` + string(data))

	createCmd := execCommandContext(context.Background(), K3sPath,
		"kubectl",
		"create",
		"configmap",
		"common",
		"--from-literal=common.rego="+stdFormat,
		"-n", "karavi",
		"--dry-run=client",
		"-o", "yaml")
	applyCmd := execCommandContext(context.Background(), K3sPath, "kubectl", "apply", "-f", "-")

	pr, pw := io.Pipe()
	createCmd.Stdout = pw
	applyCmd.Stdin = pr
	applyCmd.Stdout = io.Discard

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

func getRolesFromFile(path string) (roles.JSON, error) {
	var roles roles.JSON

	if path == "" {
		return roles, errors.New("missing file argument")
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return roles, err
	}

	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return roles, err
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("error closing file %s: %v", f.Name(), err)
		}
	}()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return roles, err
	}

	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&roles); err != nil {
		return roles, fmt.Errorf("decoding json: %w", err)
	}
	return roles, nil
}
