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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"karavi-authorization/cmd/karavictl/cmd/mocks"
	"karavi-authorization/cmd/karavictl/cmd/types"

	"github.com/golang/mock/gomock"

	"github.com/stretchr/testify/assert"
)

func getDefaultRoles() map[string][]types.Role {
	roles := make(map[string][]types.Role)
	roles["role-1"] = []types.Role{
		{
			StorageSystemID: "storage-1",
			PoolQuotas: []types.PoolQuota{
				{
					Pool:  "pool-1",
					Quota: 10000,
				},
			},
		},
	}
	roles["role-2"] = []types.Role{
		{
			StorageSystemID: "storage-2",
			PoolQuotas: []types.PoolQuota{
				{
					Pool:  "pool-2",
					Quota: 20000,
				},
			},
		},
	}

	return roles
}

func Test_Unit_RoleList(t *testing.T) {
	tests := map[string]func(t *testing.T) (RoleGetter, int, *gomock.Controller){
		"success listing default role quotas": func(*testing.T) (RoleGetter, int, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			roleGetter := mocks.NewMockRoleGetter(ctrl)
			roles := getDefaultRoles()
			roleGetter.EXPECT().GetRoles().Return(roles, nil).Times(1)
			return roleGetter, 2, ctrl
		},
		"success listing 0 roles": func(*testing.T) (RoleGetter, int, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			roleGetter := mocks.NewMockRoleGetter(ctrl)
			roles := make(map[string][]types.Role)
			roleGetter.EXPECT().GetRoles().Return(roles, nil).Times(1)
			return roleGetter, 0, ctrl
		}}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			cleanUp()

			roleGetter, expectedRoleQuotas, ctrl := tc(t)

			cmd := NewRoleListCommand(roleGetter)

			// cmd.SetArgs([]string{"role", "list"})

			stdOut := bytes.NewBufferString("")
			cmd.SetOutput(stdOut)

			err := cmd.Execute()
			assert.Nil(t, err)

			normalOut, err := ioutil.ReadAll(stdOut)
			assert.Nil(t, err)

			// read number of newlines from stdout of the command
			numberOfStdoutNewlines := len(strings.Split(strings.TrimSuffix(string(normalOut), "\n"), "\n"))
			// remove 2 header lines from stdout
			numberOfRoleQuotas := numberOfStdoutNewlines - 2
			assert.Equal(t, expectedRoleQuotas, numberOfRoleQuotas)

			ctrl.Finish()
		})
	}
}

func Test_Unit_RoleGet(t *testing.T) {
	tests := map[string]func(t *testing.T) (RoleGetter, []string, bool, *gomock.Controller){
		"success getting existing role": func(*testing.T) (RoleGetter, []string, bool, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			roleGetter := mocks.NewMockRoleGetter(ctrl)
			roles := getDefaultRoles()
			roleGetter.EXPECT().GetRoles().Return(roles, nil).Times(1)
			return roleGetter, []string{"role-1"}, false, ctrl
		},
		"error getting role that doesn't exist": func(*testing.T) (RoleGetter, []string, bool, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			roleGetter := mocks.NewMockRoleGetter(ctrl)
			roles := getDefaultRoles()
			roleGetter.EXPECT().GetRoles().Return(roles, nil).Times(1)
			return roleGetter, []string{"non-existing-role"}, true, ctrl
		},
		"error passing no role to the command": func(*testing.T) (RoleGetter, []string, bool, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			roleGetter := mocks.NewMockRoleGetter(ctrl)
			roleGetter.EXPECT().GetRoles().Times(0)
			return roleGetter, []string{}, true, ctrl
		},
		"error passing multiple roles to the command": func(*testing.T) (RoleGetter, []string, bool, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			roleGetter := mocks.NewMockRoleGetter(ctrl)
			roleGetter.EXPECT().GetRoles().Times(0)
			return roleGetter, []string{"role-1", "role-2"}, true, ctrl
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			cleanUp()

			roleGetter, rolesToGet, expectError, ctrl := tc(t)

			cmd := NewRoleGetCommand(roleGetter)
			args := []string{}
			for _, role := range rolesToGet {
				args = append(args, role)
			}
			cmd.SetArgs(args)

			stdOut := bytes.NewBufferString("")
			cmd.SetOutput(stdOut)

			err := cmd.Execute()

			if expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
			}
			ctrl.Finish()
		})
	}
}

func Test_Unit_RoleDelete(t *testing.T) {
	tests := map[string]func(t *testing.T) (RoleGetter, ConfigMapUpdater, []string, bool, *gomock.Controller){
		"success deleting existing role": func(*testing.T) (RoleGetter, ConfigMapUpdater, []string, bool, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			roleGetter := mocks.NewMockRoleGetter(ctrl)
			configMapUpdater := mocks.NewMockConfigMapUpdater(ctrl)
			roles := getDefaultRoles()
			roleGetter.EXPECT().GetRoles().Return(roles, nil).Times(1)
			configMapUpdater.EXPECT().ModifyCommonConfigMap(gomock.Any()).Return(nil).Times(1)
			return roleGetter, configMapUpdater, []string{"role-1"}, false, ctrl
		},
		"error deleting role that doesn't exist": func(*testing.T) (RoleGetter, ConfigMapUpdater, []string, bool, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			roleGetter := mocks.NewMockRoleGetter(ctrl)
			configMapUpdater := mocks.NewMockConfigMapUpdater(ctrl)
			roles := getDefaultRoles()
			roleGetter.EXPECT().GetRoles().Return(roles, nil).Times(1)
			configMapUpdater.EXPECT().ModifyCommonConfigMap(gomock.Any()).Return(nil).Times(0)
			return roleGetter, configMapUpdater, []string{"non-existing-role"}, true, ctrl
		},
		"error passing no role to the command": func(*testing.T) (RoleGetter, ConfigMapUpdater, []string, bool, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			roleGetter := mocks.NewMockRoleGetter(ctrl)
			configMapUpdater := mocks.NewMockConfigMapUpdater(ctrl)
			roles := getDefaultRoles()
			roleGetter.EXPECT().GetRoles().Return(roles, nil).Times(0)
			configMapUpdater.EXPECT().ModifyCommonConfigMap(gomock.Any()).Return(nil).Times(0)
			return roleGetter, configMapUpdater, []string{}, true, ctrl
		},
		"error passing multiple roles to the command": func(*testing.T) (RoleGetter, ConfigMapUpdater, []string, bool, *gomock.Controller) {
			ctrl := gomock.NewController(t)
			roleGetter := mocks.NewMockRoleGetter(ctrl)
			configMapUpdater := mocks.NewMockConfigMapUpdater(ctrl)
			roles := getDefaultRoles()
			roleGetter.EXPECT().GetRoles().Return(roles, nil).Times(0)
			configMapUpdater.EXPECT().ModifyCommonConfigMap(gomock.Any()).Return(nil).Times(0)
			return roleGetter, configMapUpdater, []string{"role-1", "role-2"}, true, ctrl
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {

			cleanUp()

			roleGetter, configMapUpdater, rolesToDelete, expectError, ctrl := tc(t)

			// roleStore := &RoleStore{}
			// roles, err := roleStore.GetRoles()
			// assert.Nil(t, err)
			// numberOfRolesBeforeDelete := len(roles)

			cmd := NewRoleDeleteCommand(roleGetter, configMapUpdater)
			args := []string{}
			for _, role := range rolesToDelete {
				args = append(args, role)
			}
			cmd.SetArgs(args)

			stdOut := bytes.NewBufferString("")
			cmd.SetOutput(stdOut)

			err := cmd.Execute()

			if expectError {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				// roleStore := &RoleStore{}
				// roles, err := roleStore.GetRoles()
				// assert.Nil(t, err)
				// numberOfRolesAfterDelete := len(roles)
				// assert.Equal(t, numberOfRolesBeforeDelete-1, numberOfRolesAfterDelete)
			}

			ctrl.Finish()
		})
	}
}

var (
	mux    *http.ServeMux
	server *httptest.Server
)

func setup() func() {
	mux = http.NewServeMux()
	server = httptest.NewServer(mux)

	return func() {
		server.Close()
	}
}
