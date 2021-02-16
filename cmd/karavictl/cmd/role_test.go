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
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

var _ = (func() interface{} {
	_testing = true
	return nil
}())

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

func cleanUp() {
	// delete config map
}

// Test_RoleCreateSuccess
// E CONST
// for each test
//    run test
//    e + x
//    e - x
//    e = E
// E

// integration test
func Test_RoleCreateSuccess(t *testing.T) {
	// check success new role gets added
	// the correct role gets added
	previousRoles, err := GetRoles()
	if err != nil {
		t.Fatal(err)
	}

	roles := map[string][]Role{
		"CSIBronzeTesting": {
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

	data, _ := json.Marshal(roles)
	f, err := writeTestFile("success", data)
	defer os.Remove(f.Name())

	var cmd = roleCreateCmd
	cmd.Flags().String("from-file", f.Name(), "role data from a file")

	b := bytes.NewBufferString("")
	cmd.SetOut(b)
	cmd.SetErr(b)
	cmd.Execute()
	out, err := ioutil.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}

	newRoles, err := GetRoles()
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "Successfully added role\n", string(out))
	assert.Equal(t, len(previousRoles)+1, len(newRoles))
	assert.Equal(t, reflect.DeepEqual(roles["CSIBronzeTesting"], newRoles["CSIBronzeTesting"]), true)

	// delete role
}

func Test_MissingFileError(t *testing.T) {
	var cmd = roleCreateCmd

	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	cmd.Execute()
	out, err := ioutil.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "failed to create role from file: missing file argument", string(out))
}

func Test_BadFileFormatError(t *testing.T) {
	f, err := writeTestFile("success", []byte{1, 2, 3, 4})
	defer os.Remove(f.Name())

	var cmd = roleCreateCmd
	cmd.Flags().String("from-file", f.Name(), "role data from a file")
	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	cmd.Execute()
	out, err := ioutil.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}
	assert.NotEmpty(t, string(out))
}
func Test_BadFileError(t *testing.T) {
	var cmd = roleCreateCmd
	cmd.Flags().String("from-file", "FileNotFound.json", "role data from a file")
	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	cmd.Execute()
	out, err := ioutil.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "failed to create role from file: not a valid JSON or Yaml role format. See sample roles file for more info", string(out))
}

func Test_RoleRoleExist(t *testing.T) {
	// check success new role gets added
	// the correct role gets added
	allRoles, err := GetRoles()
	if err != nil {
		t.Fatal(err)
	}

	keys := reflect.ValueOf(allRoles).MapKeys()
	roleName := keys[rand.Intn(len(keys))].Interface().(string)

	roles := map[string][]Role{roleName: allRoles[roleName]}

	data, _ := json.Marshal(roles)
	f, err := writeTestFile("success", data)
	defer os.Remove(f.Name())

	var cmd = roleCreateCmd
	cmd.Flags().String("from-file", f.Name(), "role data from a file")

	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	cmd.Execute()
	out, err := ioutil.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "failed to create role from file: "+roleName+"already exist\n", string(out))
}
func Test_RoleUpdate(t *testing.T) {
	t.Skip("TODO")

	// check success role is update gets added
	// the correct role gets update only
	// creating role that already exist gives error
	// any error during creating shows mics bad files
}

func Test_RoleList(t *testing.T) {
	var cmd = rootCmd
	cmd.SetArgs([]string{"role", "list"})

	b := bytes.NewBufferString("")
	cmd.SetErr(b)
	cmd.Execute()
	out, err := ioutil.ReadAll(b)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(string(out))

	t.Fatal("lol")

}
