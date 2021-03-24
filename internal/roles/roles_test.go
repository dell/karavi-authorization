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

package roles_test

import (
	"encoding/json"
	"karavi-authorization/internal/roles"
	"strconv"
	"strings"
	"testing"
)

const ExpectedInstanceCount = 3

func TestRoleKey_String(t *testing.T) {
	var tests = []struct {
		want  string
		given roles.RoleKey
	}{
		{
			"RoleName=powerflex=123=mypool",
			roles.RoleKey{
				Name:       "RoleName",
				SystemType: "powerflex",
				SystemID:   "123",
				Pool:       "mypool",
			},
		},
		{
			"RoleName=powerflex=123",
			roles.RoleKey{
				Name:       "RoleName",
				SystemType: "powerflex",
				SystemID:   "123",
			},
		},
		{
			"RoleName=powerflex",
			roles.RoleKey{
				Name:       "RoleName",
				SystemType: "powerflex",
			},
		},
		{
			"RoleName",
			roles.RoleKey{
				Name: "RoleName",
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.want, func(t *testing.T) {
			got := tt.given.String()
			if got != tt.want {
				t.Errorf("(%s): got %s, want %s", tt.want, got, tt.want)
			}

		})
	}
}

func TestJSON_Get(t *testing.T) {
	sut := buildJSON(t)
	t.Run("valid", func(t *testing.T) {
		got := sut.Get(validRoleKey(t))

		if got == nil {
			t.Errorf("expected non-nil, but was nil")
		}
	})
	t.Run("missing", func(t *testing.T) {
		got := sut.Get(roles.RoleKey{Name: "nothere"})

		if got != nil {
			t.Errorf("got %v, want %v", got, nil)
		}
	})
}

func TestJSON_MarshalJSON(t *testing.T) {
	sut := buildJSON(t)

	_, err := json.Marshal(&sut)
	if err != nil {
		t.Fatal(err)
	}
}

func TestJSON_Unmarshal(t *testing.T) {
	sut := buildJSON(t)

	got := sut.Instances()

	want := ExpectedInstanceCount
	if got := len(got); got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestNewInstance(t *testing.T) {
	name := "test"
	args := []string{"powerflex", "542", "bronze", "100"}

	got := roles.NewInstance(name, args...)

	n, err := strconv.ParseInt(args[3], 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	want := &roles.Instance{
		RoleKey: roles.RoleKey{
			Name:       name,
			SystemType: args[0],
			SystemID:   args[1],
			Pool:       args[2],
		},
		Quota: int(n),
	}
	if *got != *want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestJSON_Instances(t *testing.T) {
	sut := roles.NewJSON()
	if err := sut.Add(roles.NewInstance("role-1")); err != nil {
		t.Fatal(err)
	}
	if err := sut.Add(roles.NewInstance("role-2")); err != nil {
		t.Fatal(err)
	}

	list := sut.Instances()

	want := 2
	if got := len(list); got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestJSON_Select(t *testing.T) {
	sut := buildJSON(t)

	var bronze []roles.Instance
	sut.Select(func(r roles.Instance) {
		if r.Pool == "bronze" {
			bronze = append(bronze, r)
		}
	})

	want := 2
	if got := len(bronze); got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestJSON_Add(t *testing.T) {
	t.Run("nil map", func(t *testing.T) {
		var sut roles.JSON // internal map not initialized
		adding := &roles.Instance{}
		adding.Name = "adding"

		err := sut.Add(adding)
		if err != nil {
			t.Fatal(err)
		}

		got := len(sut.Instances())
		want := 1
		if got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	})
	t.Run("new entry", func(t *testing.T) {
		sut := buildJSON(t)
		adding := &roles.Instance{}
		adding.Name = "adding"

		err := sut.Add(adding)
		if err != nil {
			t.Fatal(err)
		}

		got := len(sut.Instances())
		want := ExpectedInstanceCount + 1
		if got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	})
	t.Run("existing entry", func(t *testing.T) {
		sut := buildJSON(t)
		adding := &roles.Instance{}
		adding.Name = "adding"
		err := sut.Add(adding)
		if err != nil {
			t.Fatal(err)
		}

		err = sut.Add(adding)

		if err == nil {
			t.Error("expected error but was nil")
		}
	})
}

func TestJSON_Remove(t *testing.T) {
	t.Run("validation", func(t *testing.T) {
		var tests = []struct {
			name      string
			givenArgs []string
			expectErr bool
		}{
			{"", []string{"badrole"}, true},
			{"", []string{"OpenShiftMongo", "badsystemtype"}, true},
			{"", []string{"OpenShiftMongo", "powerflex", "badsystemid"}, true},
			{"", []string{"OpenShiftMongo", "powerflex", "542a2d5f5122210f", "badpool"}, true},
			{"", []string{"OpenShiftMongo", "powerflex", "542a2d5f5122210f", "bronze"}, false},
		}
		for _, tt := range tests {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				sut := buildJSON(t)

				err := sut.Remove(roles.NewInstance(tt.givenArgs[0], tt.givenArgs[1:]...))

				if tt.expectErr && err == nil {
					t.Errorf("err: got %+v, want %+v", nil, true)
				}
			})
		}
	})
	t.Run("it removes the instance", func(t *testing.T) {
		sut := buildJSON(t)
		c := len(sut.Instances())

		err := sut.Remove(roles.NewInstance("OpenShiftMongo",
			"powerflex",
			"542a2d5f5122210f",
			"bronze"))
		if err != nil {
			t.Fatal(err)
		}

		want := c - 1
		if got := len(sut.Instances()); got != want {
			t.Errorf("len(instances): got %d, want %d", got, want)
		}
	})
}

func buildJSON(t *testing.T) *roles.JSON {
	payload := `
{ 
  "OpenShiftMongo": {
    "system_types": {
      "powerflex": {
        "system_ids": {
          "542a2d5f5122210f": {
            "pool_quotas": {
              "bronze": 44000000,
			  "silver": 88000000
            }
          }
        }
      }
    }
  },
  "OpenShiftMongo-large": {
    "system_types": {
      "powerflex": {
        "system_ids": {
          "542a2d5f5122210f": {
            "pool_quotas": {
              "bronze": 44000000
            }
          }
        }
      }
    }
  }
}
`
	var sut roles.JSON
	err := json.NewDecoder(strings.NewReader(payload)).Decode(&sut)
	if err != nil {
		t.Fatal(err)
	}

	return &sut
}

func validRoleKey(t *testing.T) roles.RoleKey {
	sut := buildJSON(t)
	return sut.Instances()[0].RoleKey
}
