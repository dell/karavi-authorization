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
	"fmt"
	"karavi-authorization/internal/role-service/roles"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"github.com/valyala/fastjson"
)

// ReadableInstance embeds a RoleKey and adds additional data, e.g. the
// quota.
type ReadableInstance struct {
	Role  roles.RoleKey
	Quota string
}

// ReadableJSON is the outer wrapper for performing JSON operations
// on a collection of role instances.
type ReadableJSON struct {
	m map[roles.RoleKey]*ReadableInstance
}

// NewRoleListCmd creates a new role list command
func NewRoleListCmd() *cobra.Command {
	roleListCmd := &cobra.Command{
		Use:   "list",
		Short: "List roles",
		Long:  `List roles`,
		Run: func(cmd *cobra.Command, args []string) {
			rolesmap, err := GetRoles()
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("unable to list roles: %v", err))
			}

			readRole := transformReadable(rolesmap)
			err = JSONOutput(cmd.OutOrStdout(), &readRole)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("unable to format json output: %v", err))
			}
		},
	}
	return roleListCmd
}

func transformReadable(rolesmap *roles.JSON) *ReadableJSON {

	readableroles := &ReadableJSON{}

	if readableroles.m == nil {
		readableroles.m = make(map[roles.RoleKey]*ReadableInstance)
	}

	for k, v := range rolesmap.M {
		ins := &ReadableInstance{
			Role: k,
		}
		// quota is stored as kilobytes, so convert back to bytes before returning
		ins.Quota = humanize.Bytes(uint64(v.Quota) * 1000)
		ins.Role = v.RoleKey
		readableroles.m[k] = ins
	}

	return readableroles
}

// MarshalJSON marshals the JSON value into JSON.
// It adds extra maps around each type of data to
// help describe it.
func (j *ReadableJSON) MarshalJSON() ([]byte, error) {

	m := make(map[string]interface{})

	initMap := func(m interface{}, key string) map[string]interface{} {
		t := m.(map[string]interface{})
		if _, ok := t[key]; !ok {
			t[key] = make(map[string]interface{})
		}
		ret := t[key].(map[string]interface{})
		return ret
	}

	for k, v := range j.m {
		// role names
		if _, ok := m[k.Name]; !ok {
			m[k.Name] = make(map[string]interface{})
		}
		// system types
		st := initMap(m[k.Name], "system_types")
		if _, ok := st[k.SystemType]; !ok {
			st[k.SystemType] = make(map[string]interface{})
		}
		// system ids
		sid := initMap(st[k.SystemType], "system_ids")
		if _, ok := sid[k.SystemID]; !ok {
			sid[k.SystemID] = make(map[string]interface{})
		}
		// pools
		p := initMap(sid[k.SystemID], "pool_quotas")
		if _, ok := p[k.Pool]; !ok {
			p[k.Pool] = make(map[string]interface{})
		}
		// pool quotas
		p[k.Pool] = v.Quota
	}

	return json.Marshal(&m)
}

// UnmarshalJSON unmarshals the given bytes into this
// JSON value.
func (j *ReadableJSON) UnmarshalJSON(b []byte) error {

	if j.m == nil {
		j.m = make(map[roles.RoleKey]*ReadableInstance)
	}
	var p fastjson.Parser

	v, err := p.ParseBytes(b)
	if err != nil {
		return err
	}
	o, err := v.Object()
	if err != nil {
		return err
	}

	o.Visit(func(k1 []byte, v1 *fastjson.Value) {
		// k1 = name
		v1.GetObject("system_types").Visit(func(k2 []byte, v2 *fastjson.Value) {
			// k2 = system type
			v2.GetObject("system_ids").Visit(func(k3 []byte, v3 *fastjson.Value) {
				// k3 = system id
				v3.GetObject("pool_quotas").Visit(func(k4 []byte, v4 *fastjson.Value) {
					//n, err := v4.Int()
					if err != nil {
						return
					}
					r := ReadableInstance{
						Role: roles.RoleKey{
							Name:       string(k1),
							SystemType: string(k2),
							SystemID:   string(k3),
							Pool:       string(k4),
						},

						Quota: v4.String(),
					}
					j.m[r.Role] = &r
				})
			})
		})
	})

	return nil
}
