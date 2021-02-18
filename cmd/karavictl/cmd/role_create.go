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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"sigs.k8s.io/yaml"
)

// roleCreateCmd represents the role command
var roleCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create one or more Karavi roles",
	Long:  `Creates one or more Karavi roles`,
	RunE: func(cmd *cobra.Command, args []string) error {
		outFormat := "failed to create role from file: %+v\n"

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
			if _, ok := existingRoles[name]; ok {
				err = fmt.Errorf("%s already exist. Try update command", name)
				return fmt.Errorf(outFormat, err)
			}

			for i := range rls {
				// validate each role
				err = validateRole(rls[i])
				if err != nil {
					err = fmt.Errorf("%s failed validation: %+v\n", name, err)
					return fmt.Errorf(outFormat, err)
				}
			}
			existingRoles[name] = rls
		}

		if err = modifyCommonConfigMap(existingRoles); err != nil {
			return fmt.Errorf(outFormat, err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Role was successfully created")
		return nil

	},
}

func init() {
	roleCmd.AddCommand(roleCreateCmd)
	roleCreateCmd.Flags().StringP("from-file", "f", "", "role data from a file")
}

func validateRole(role Role) error {
	/*storage, err := GetAuthStorageSystem()
		return fmt.Errorf("failed get authorized storage systems: %+v\n", err)
	}

	// using map to optimize search
	search := make(map[string]map[string]int64)
	for sysID, poolQs := range storage {
		m := make(map[string]int64)
		for _, pq := range poolQs {
			m[pq.Pool] = pq.Quota
		}
		search[sysID] = m
	}

	if _, ok := search[role.StorageSystemID]; !ok {
		return errors.New("storage systems does not exit and/or is not authorized")
	}

	for _, rl := range role.PoolQuotas {
		if qt, ok := search[role.StorageSystemID][rl.Pool]; ok {
			if rl.Quota <= qt {
				return nil
			}
			return errors.New("the specified quota is larger than the storage capacity")
		}

	}
	return errors.New("the specified pools do exist on the given storage system")*/
	return nil
}

func writeToFile(fileName string, data []byte) (*os.File, error) {
	tempFile, err := ioutil.TempFile("", fileName)
	if err != nil {
		return nil, err
	}
	_, err = tempFile.Write(data)
	if err != nil {
		return nil, err
	}
	return tempFile, nil
}

func modifyCommonConfigMap(roles map[string][]Role) error {
	var err error

	data, err := json.MarshalIndent(roles, "", "  ")
	if err != nil {
		return err
	}
	stdFormat := (`package karavi.common
default roles = {}
roles = ` + string(data))

	createCmd := exec.Command("k3s",
		"kubectl",
		"create",
		"configmap",
		"common",
		"--from-literal=common.rego="+stdFormat,
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

func getRolesFromFile(path string) (map[string][]Role, error) {
	if path == "" {
		return nil, errors.New("missing file argument")
	}

	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, _ := ioutil.ReadAll(f)

	var roles map[string][]Role

	if err = json.Unmarshal(b, &roles); err != nil {
		err = yaml.Unmarshal(b, &roles)
		if err != nil {
			return nil, fmt.Errorf("not a valid JSON or Yaml role format: %+v", err) //err
		}
	}
	return roles, nil
}
