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
		fromFile, err := cmd.Flags().GetString("from-file")
		if err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
		}

		var rff roles.JSON
		switch {
		case fromFile != "":
			var err error
			rff, err = getRolesFromFile(fromFile)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
			}

		case len(roleFlags) != 0:
			for _, v := range roleFlags {
				t := strings.Split(v, "=")
				rff.Add(roles.NewInstance(t[0], t[1:]...))
			}
		default:
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, errors.New("no input")))
		}

		existingRoles, err := GetRoles()
		if err != nil {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
		}

		for _, role := range rff.Instances() {
			if _, ok := existingRoles.Roles[role.Name]; ok {
				err = fmt.Errorf("%s already exist. Try update command", role.Name)
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
			}

			err := validateRole(role)
			if err != nil {
				err = fmt.Errorf("%s failed validation: %+v", role.Name, err)
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
			}

			existingRoles.Add(role)
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

	data, err := json.MarshalIndent(roles.Roles, "", "  ")
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

func getRolesFromFile(path string) (roles.JSON, error) {
	var roles roles.JSON

	if path == "" {
		return roles, errors.New("missing file argument")
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return roles, err
	}

	f, err := os.Open(path)
	if err != nil {
		return roles, err
	}
	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		return roles, err
	}

	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&roles.Roles); err != nil {
		return roles, fmt.Errorf("decoding json: %w", err)
	}
	return roles, nil
}
