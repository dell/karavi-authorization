/*
 Copyright © 2022 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/
package types

import (
	"fmt"
	"strings"
)

const (
	powerflex  = "powerflex"
	powermax   = "powermax"
	powerscale = "powerscale"
)

// Storage represents a map of storage system types.
type Storage map[string]SystemType

// SystemType represents a map of systems.
type SystemType map[string]System

// System represents the properties of a system.
type System struct {
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Endpoint string `yaml:"endpoint"`
	Insecure bool   `yaml:"insecure"`
}

// SupportedStorageTypes holds the supported storage types
var SupportedStorageTypes = map[string]struct{}{
	powerflex:  {},
	powermax:   {},
	powerscale: {},
}

// SystemID wraps a system ID to be a quoted string because system IDs could be all numbers
// which will cause issues with yaml marshalers
type SystemID struct {
	Value string
}

func (id SystemID) String() string {
	return fmt.Sprintf("%q", strings.ReplaceAll(id.Value, `"`, ""))
}
