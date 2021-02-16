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
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/rexray/gocsi/csc/cmd"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"sigs.k8s.io/yaml"
)

func writeTestFile(fileName string, data []byte) (*os.File, error) {
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

func cleanUp() error {
	deleteCmd := exec.Command("k3s",
		"kubectl",
		"delete",
		"configmap",
		"common", "-n", "karavi",
	)
	if err := deleteCmd.Run(); err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	return nil
}

func createDefaultRoles() error {
	createCmd := exec.Command("k3s",
		"kubectl",
		"create",
		"configmap",
		"common",
		"-n", "karavi",
		"--from-file", "testdata/default_roles.proto",
		"--dry-run=client",
		"-o", "yaml")
	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("create: %w", err)
	}

	applyCmd := exec.Command("k3s", "kubectl", "apply", "-f", "-")
	if err := applyCmd.Run(); err != nil {
		return fmt.Errorf("create: %w", err)
	}

	return nil
}

func Test_Role_Create(t *testing.T) {
	defer cleanUp()

	testInit := func() {
		if err := cleanUp(); err != nil {
			t.Error(err)
		}
		if err := createDefaultRoles(); err != nil {
			t.Error(err)
		}
	}
	roles := map[string][]Role{
		"CSIBronzeTestingCreate": {
			Role{
				StorageSystemID: "system_id1",
				PoolQuotas: []PoolQuota{
					{Pool: "silver", Quota: 32000000},
				},
			},
			Role{
				StorageSystemID: "system_id2",
				PoolQuotas: []PoolQuota{
					{Pool: "silver", Quota: 9000000},
				},
			},
		},
	}

	type checkFn func(*testing.T, string, error)
	checkFns := func(fns ...checkFn) []checkFn { return fns }

	verifyError := func(t *testing.T, out string, err error) {
		if err == nil {
			t.Errorf("expected an error, got nil")
		}
	}

	verifyNoError := func(t *testing.T, out string, err error) {
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	}

	checkOutputStr := func(expectedOut string) func(t *testing.T, out string, err error) {
		return func(t *testing.T, out string, err error) {
			assert.Equal(t, expectedOut, out)
		}
	}

	checkWasAdded := func(oldSize int) func(t *testing.T, out string, err error) {
		return func(t *testing.T, out string, err error) {
			newRoles, err := GetRoles()
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, oldSize+1, len(newRoles))
			assert.Equal(t, reflect.DeepEqual(roles["CSIBronzeTestingCreate"], newRoles["CSIBronzeTestingCreate"]), true)
		}
	}

	tests := map[string]func(t *testing.T) (*cobra.Command, []checkFn){
		"success: JSON": func(t *testing.T) (*cobra.Command, []checkFn) {
			previousRoles, err := GetRoles()
			if err != nil {
				t.Fatal(err)
			}

			data, _ := json.Marshal(roles)
			f, err := writeTestFile("successJSON", data)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())

			var cmd = rootCmd
			cmd.SetArgs([]string{"role", "create", "-f", f.Name()})

			return cmd, checkFns(verifyNoError, checkOutputStr("Role was successfully created\n"), checkWasAdded(len(previousRoles)))
		},
		"success: Yaml": func(t *testing.T) (*cobra.Command, []checkFn) {
			previousRoles, err := GetRoles()
			if err != nil {
				t.Fatal(err)
			}

			data, _ := yaml.Marshal(roles)
			f, err := writeTestFile("successYAML", data)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())

			var cmd = rootCmd
			cmd.SetArgs([]string{"role", "create", "-f", f.Name()})

			return cmd, checkFns(verifyNoError, checkOutputStr("Role was successfully created\n"), checkWasAdded(len(previousRoles)))
		},

		"failure: role already exit": func(t *testing.T) (*cobra.Command, []checkFn) {
			previousRoles, err := GetRoles()
			if err != nil {
				t.Fatal(err)
			}
			keys := reflect.ValueOf(previousRoles).MapKeys()
			role := keys[rand.Intn(len(keys))].Interface().(string)

			rolesTmp := map[string][]Role{role: previousRoles[role]}
			data, _ := json.Marshal(rolesTmp)
			f, err := writeTestFile("failureAllReadyExist", data)
			if err != nil {
				t.Fatal(err)
			}

			defer os.Remove(f.Name())

			var cmd = rootCmd
			cmd.SetArgs([]string{"role", "create", "-f", f.Name()})

			return cmd, checkFns(verifyError, checkOutputStr("failed to create role from file: "+role+" already exist\n"))
		},
		"failure: missing file": func(t *testing.T) (*cobra.Command, []checkFn) {
			var cmd = rootCmd
			cmd.SetArgs([]string{"role", "create"})
			return cmd, checkFns(verifyError, checkOutputStr("failed to create role from file: missing file argument\n"))
		},
		"failure: error parsing file": func(t *testing.T) (*cobra.Command, []checkFn) {
			f, err := writeTestFile("failureAllBadFormat", []byte{1, 2, 3, 4})
			if err != nil {
				t.Fatal(err)
			}

			defer os.Remove(f.Name())
			var cmd = rootCmd
			cmd.SetArgs([]string{"role", "create", "-f", f.Name()})
			return cmd, checkFns(verifyError, checkOutputStr("failed to create role from file: not a valid JSON or Yaml role format. See sample roles file for more info\n"))
		},
		"failure: other error with file": func(t *testing.T) (*cobra.Command, []checkFn) {
			var cmd = rootCmd
			cmd.SetArgs([]string{"role", "create", "-f", "FileNotFound.json"})
			return cmd, checkFns(verifyError)
		},
		"failure: the storage system does not exist": func(t *testing.T) (*cobra.Command, []checkFn) {
			// Need to mock get storage system
			badRoles := map[string][]Role{
				"CSIBronzeTestingCreate": {
					Role{
						StorageSystemID: "system_id_NotFound",
						PoolQuotas: []PoolQuota{
							{Pool: "silver", Quota: 32000000},
						},
					},
				},
			}
			data, _ := json.Marshal(badRoles)
			f, err := writeTestFile("failureSSNotFound", data)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())

			var cmd = rootCmd
			cmd.SetArgs([]string{"role", "create", "-f", f.Name()})
			return cmd, checkFns(verifyError, checkOutputStr("failed to create role from file: the storage system does not exist\n"))
		},
		"failure: the specified pools do exist on the given storage system": func(t *testing.T) (*cobra.Command, []checkFn) {
			// Need to mock get storage system
			badRoles := map[string][]Role{
				"CSIBronzeTestingCreate": {
					Role{
						StorageSystemID: "system_id1",
						PoolQuotas: []PoolQuota{
							{Pool: "poolNotFound", Quota: 32000000},
						},
					},
				},
			}
			data, _ := json.Marshal(badRoles)
			f, err := writeTestFile("failurepoolNotFound", data)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())

			var cmd = rootCmd
			cmd.SetArgs([]string{"role", "create", "-f", f.Name()})
			return cmd, checkFns(verifyError, checkOutputStr("failed to create role from file:  the specified pools do exist on the given storage system\n"))
		},
		"failure: the specified quota is larger than the storage capacity": func(t *testing.T) (*cobra.Command, []checkFn) {
			// Need to mock get storage system
			badRoles := map[string][]Role{
				"CSIBronzeTestingCreate": {
					Role{
						StorageSystemID: "system_id1",
						PoolQuotas: []PoolQuota{
							{Pool: "silver", Quota: 320000000000000},
						},
					},
				},
			}
			data, _ := json.Marshal(badRoles)
			f, err := writeTestFile("failureQuotaTooBig", data)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())

			var cmd = rootCmd
			cmd.SetArgs([]string{"role", "create", "-f", f.Name()})
			return cmd, checkFns(verifyError, checkOutputStr("failed to create role from file:   the specified quota is larger than the storage capacity\n"))
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			testInit()

			cmd, checkFns := tc(t)

			b := bytes.NewBufferString("")
			cmd.SetErr(b)
			RunErr := cmd.Execute()
			out, err := ioutil.ReadAll(b)
			if err != nil {
				t.Fatal(err)
			}

			for _, checkFn := range checkFns {
				checkFn(t, string(out), RunErr)
			}
		})
	}

}

func Test_Role_Update(t *testing.T) {
	defer cleanUp()

	testInit := func() {
		cleanUp()
		createDefaultRoles()
	}
	roles := map[string][]Role{
		"CSISilver": {
			Role{
				StorageSystemID: "system_id1",
				PoolQuotas: []PoolQuota{
					{Pool: "silver", Quota: 32000000},
				},
			},
			Role{
				StorageSystemID: "system_id2",
				PoolQuotas: []PoolQuota{
					{Pool: "silver", Quota: 9000000},
				},
			},
		},
	}

	type checkFn func(*testing.T, string, error)
	checkFns := func(fns ...checkFn) []checkFn { return fns }

	verifyError := func(t *testing.T, out string, err error) {
		if err == nil {
			t.Errorf("expected an error, got nil")
		}
	}

	verifyNoError := func(t *testing.T, out string, err error) {
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	}

	checkOutputStr := func(expectedOut string) func(t *testing.T, out string, err error) {
		return func(t *testing.T, out string, err error) {
			assert.Equal(t, expectedOut, out)
		}
	}

	checkWasAdded := func(oldSize int) func(t *testing.T, out string, err error) {
		return func(t *testing.T, out string, err error) {
			newRoles, err := GetRoles()
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, oldSize, len(newRoles))
			assert.Equal(t, reflect.DeepEqual(roles["CSISilver"], newRoles["CSISilver"]), true)
		}
	}

	tests := map[string]func(t *testing.T) (*cobra.Command, []checkFn){
		"success: JSON": func(t *testing.T) (*cobra.Command, []checkFn) {
			previousRoles, err := GetRoles()
			if err != nil {
				t.Fatal(err)
			}

			data, _ := json.Marshal(roles)
			f, err := writeTestFile("successJSON", data)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())

			var cmd = rootCmd
			cmd.SetArgs([]string{"role", "update", "-f", f.Name()})

			return cmd, checkFns(verifyNoError, checkOutputStr("Role was successfully updated\n"), checkWasAdded(len(previousRoles)))
		},
		"success: Yaml": func(t *testing.T) (*cobra.Command, []checkFn) {
			previousRoles, err := GetRoles()
			if err != nil {
				t.Fatal(err)
			}

			data, _ := yaml.Marshal(roles)
			f, err := writeTestFile("successYAML", data)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())

			var cmd = rootCmd
			cmd.SetArgs([]string{"role", "update", "-f", f.Name()})

			return cmd, checkFns(verifyNoError, checkOutputStr("Role was successfully updated\n"), checkWasAdded(len(previousRoles)))
		},

		"failure: role does not exit": func(t *testing.T) (*cobra.Command, []checkFn) {
			badRoles := map[string][]Role{
				"CSISilverDoesNotExist": {
					Role{
						StorageSystemID: "system_id_NotFound",
						PoolQuotas: []PoolQuota{
							{Pool: "silver", Quota: 32000000},
						},
					},
				},
			}
			data, _ := json.Marshal(badRoles)
			f, err := writeTestFile("failureDoesNotExist", data)
			if err != nil {
				t.Fatal(err)
			}

			defer os.Remove(f.Name())

			var cmd = rootCmd
			cmd.SetArgs([]string{"role", "update", "-f", f.Name()})

			return cmd, checkFns(verifyError, checkOutputStr("failed to update role from file: CSISilverDoesNotExist role does not exit\n"))
		},
		"failure: missing file": func(t *testing.T) (*cobra.Command, []checkFn) {
			var cmd = rootCmd
			cmd.SetArgs([]string{"role", "update"})
			return cmd, checkFns(verifyError, checkOutputStr("failed to update role from file: missing file argument\n"))
		},
		"failure: error parsing file": func(t *testing.T) (*cobra.Command, []checkFn) {
			f, err := writeTestFile("failureAllBadFormat", []byte{1, 2, 3, 4})
			if err != nil {
				t.Fatal(err)
			}

			defer os.Remove(f.Name())
			var cmd = rootCmd
			cmd.SetArgs([]string{"role", "update", "-f", f.Name()})
			return cmd, checkFns(verifyError, checkOutputStr("failed to update role from file: not a valid JSON or Yaml role format. See sample roles file for more info\n"))
		},
		"failure: other error with file": func(t *testing.T) (*cobra.Command, []checkFn) {
			var cmd = rootCmd
			cmd.SetArgs([]string{"role", "update", "-f", "FileNotFound.json"})
			return cmd, checkFns(verifyError)
		},
		"failure: the storage system does not exist": func(t *testing.T) (*cobra.Command, []checkFn) {
			// Need to mock get storage system
			badRoles := map[string][]Role{
				"CSISilver": {
					Role{
						StorageSystemID: "system_id_NotFound",
						PoolQuotas: []PoolQuota{
							{Pool: "silver", Quota: 32000000},
						},
					},
				},
			}
			data, _ := json.Marshal(badRoles)
			f, err := writeTestFile("failureSSNotFound", data)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())

			var cmd = rootCmd
			cmd.SetArgs([]string{"role", "update", "-f", f.Name()})
			return cmd, checkFns(verifyError, checkOutputStr("failed to update role from file: the storage system does not exist\n"))
		},
		"failure: the specified pools do exist on the given storage system": func(t *testing.T) (*cobra.Command, []checkFn) {
			// Need to mock get storage system
			badRoles := map[string][]Role{
				"CSISilver": {
					Role{
						StorageSystemID: "system_id1",
						PoolQuotas: []PoolQuota{
							{Pool: "poolNotFound", Quota: 32000000},
						},
					},
				},
			}
			data, _ := json.Marshal(badRoles)
			f, err := writeTestFile("failurepoolNotFound", data)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())

			var cmd = rootCmd
			cmd.SetArgs([]string{"role", "update", "-f", f.Name()})
			return cmd, checkFns(verifyError, checkOutputStr("failed to update role from file:  the specified pools do exist on the given storage system\n"))
		},
		"failure: the specified quota is larger than the storage capacity": func(t *testing.T) (*cobra.Command, []checkFn) {
			// Need to mock get storage system
			badRoles := map[string][]Role{
				"CSISilver": {
					Role{
						StorageSystemID: "system_id1",
						PoolQuotas: []PoolQuota{
							{Pool: "silver", Quota: 320000000000000},
						},
					},
				},
			}
			data, _ := json.Marshal(badRoles)
			f, err := writeTestFile("failureQuotaTooBig", data)
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())

			var cmd = rootCmd
			cmd.SetArgs([]string{"role", "update", "-f", f.Name()})
			return cmd, checkFns(verifyError, checkOutputStr("failed to update role from file:   the specified quota is larger than the storage capacity\n"))
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			cleanUp()

			initFunction, expectedRoleQuotas := tc(t)

			if initFunction != nil {
				initFunction()
			}

			for name, tc := range tests {
				t.Run(name, func(t *testing.T) {
					testInit()

					cmd, checkFns := tc(t)

					b := bytes.NewBufferString("")
					cmd.SetErr(b)
					RunErr := cmd.Execute()
					out, err := ioutil.ReadAll(b)
					if err != nil {
						t.Fatal(err)
					}

					for _, checkFn := range checkFns {
						checkFn(t, string(out), RunErr)
					}
				})
			}

			err := cmd.Execute()
			assert.Nil(t, err)

			normalOut, err := ioutil.ReadAll(stdOut)
			assert.Nil(t, err)

			// read number of newlines from stdout of the command
			numberOfStdoutNewlines := len(strings.Split(strings.TrimSuffix(string(normalOut), "\n"), "\n"))
			// remove 2 header lines from stdout
			numberOfRoleQuotas := numberOfStdoutNewlines - 2
			assert.Equal(t, expectedRoleQuotas, numberOfRoleQuotas)
		})
	}
}
