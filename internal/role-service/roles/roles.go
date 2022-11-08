// Copyright © 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
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

// Package roles provides functions and types for managing role data.
package roles

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/dustin/go-humanize"
	"github.com/valyala/fastjson"
)

// RoleKey represents a unique key for a role, comprised of its
// struct fields. When used as a key type for a map value, this
// works to enforce the rule of only specifying a given pool
// once per role entry.
type RoleKey struct {
	Name       string
	SystemType string
	SystemID   string
	Pool       string
}

// Instance embeds a RoleKey and adds additional data, e.g. the
// quota.
type Instance struct {
	RoleKey
	Quota int
}

// JSON is the outer wrapper for performing JSON operations
// on a collection of role instances.
type JSON struct {
	mu sync.Mutex // guards m
	M  map[RoleKey]*Instance
}

// String returns the string representation of a RoleKey.
func (r *RoleKey) String() string {
	sb := strings.Builder{}
	fmt.Fprintf(&sb, "%s", r.Name)
	if strings.TrimSpace(r.SystemType) == "" {
		return sb.String()
	}
	fmt.Fprintf(&sb, "=%s", r.SystemType)
	if strings.TrimSpace(r.SystemID) == "" {
		return sb.String()
	}
	fmt.Fprintf(&sb, "=%s", r.SystemID)
	if strings.TrimSpace(r.Pool) == "" {
		return sb.String()
	}
	fmt.Fprintf(&sb, "=%s", r.Pool)
	return sb.String()
}

// NewJSON builds a new JSON value with an allocated map.
func NewJSON() JSON {
	return JSON{
		M: make(map[RoleKey]*Instance),
	}
}

// NewInstance builds a new role. Its arguments expect the following
// format:
// - role: name of the role
// - parts[0]: system type
// - parts[1]: system id
// - parts[2]: pool name
// - parts[3]: quota
func NewInstance(role string, parts ...string) (*Instance, error) {
	ins := &Instance{}
	ins.Name = role
	for i, v := range parts {
		switch i {
		case 0: // system type
			ins.SystemType = v
		case 1: // system id
			ins.SystemID = v
		case 2: // pool name
			ins.Pool = v
		case 3: // quota
			// if quota can be converted to an integer, set units to kilobytes
			if _, err := strconv.Atoi(v); err == nil {
				v = fmt.Sprintf("%s KB", v)
			}
			n, err := humanize.ParseBytes(v)
			if err != nil {
				return nil, err
			}
			// store quota in kilobytes
			ins.Quota = int(n / 1000)
		}

	}
	return ins, nil
}

// Get returns an *Instance associated with the given key.
func (j *JSON) Get(k RoleKey) *Instance {
	j.mu.Lock()
	defer j.mu.Unlock()

	if v, ok := j.M[k]; ok {
		return v
	}
	return nil
}

// Select visits each known role instance and passes it
// to the provided function.
func (j *JSON) Select(fn func(r Instance)) {
	j.mu.Lock()
	defer j.mu.Unlock()

	for _, v := range j.M {
		fn(*v)
	}
}

// Instances returns each role instance in a slice.
func (j *JSON) Instances() []*Instance {
	j.mu.Lock()
	defer j.mu.Unlock()

	var ret []*Instance
	for _, v := range j.M {
		ret = append(ret, v)
	}
	return ret
}

// Add attempts to add the given role instance into the
// collection.
func (j *JSON) Add(v *Instance) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.M == nil {
		j.M = make(map[RoleKey]*Instance)
	}
	if _, ok := j.M[v.RoleKey]; ok {
		return fmt.Errorf("%q is duplicated", v.RoleKey)
	}

	j.M[v.RoleKey] = v

	return nil
}

// Remove attempts to remove the given role instance from
// the collection.
func (j *JSON) Remove(r *Instance) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if _, ok := j.M[r.RoleKey]; !ok {
		return fmt.Errorf("%s not found", r.String())
	}
	delete(j.M, r.RoleKey)
	return nil
}

// MarshalJSON marshals the JSON value into JSON.
// It adds extra maps around each type of data to
// help describe it.
func (j *JSON) MarshalJSON() ([]byte, error) {
	j.mu.Lock()
	defer j.mu.Unlock()

	m := make(map[string]interface{})

	initMap := func(m interface{}, key string) map[string]interface{} {
		t := m.(map[string]interface{})
		if _, ok := t[key]; !ok {
			t[key] = make(map[string]interface{})
		}
		ret := t[key].(map[string]interface{})
		return ret
	}

	for k, v := range j.M {
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
func (j *JSON) UnmarshalJSON(b []byte) error {
	j.mu.Lock()
	defer j.mu.Unlock()

	if j.M == nil {
		j.M = make(map[RoleKey]*Instance)
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
					n, err := v4.Int()
					if err != nil {
						return
					}
					r := Instance{
						RoleKey: RoleKey{
							Name:       string(k1),
							SystemType: string(k2),
							SystemID:   string(k3),
							Pool:       string(k4),
						},

						Quota: n,
					}
					j.M[r.RoleKey] = &r
				})
			})
		})
	})

	return nil
}

func mustParseInt(n int64, err error) int64 {
	if err != nil {
		panic(err)
	}
	return n
}
