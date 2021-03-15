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
	"strings"
	"testing"
)

func TestJSON_Unmarshal(t *testing.T) {
	sut := buildJSON(t)

	got := sut.Instances()

	want := 2
	if got := len(got); got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func TestNewInstance(t *testing.T) {
	name := "test"
	args := []string{"powerflex", "542", "bronze", "100G"}

	got := roles.NewInstance(name, args...)

	want := roles.Instance{
		Name:       name,
		SystemType: args[0],
		SystemID:   args[1],
		Pool:       args[2],
		Quota:      args[3],
	}
	if got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestJSON_Instances(t *testing.T) {
	sut := roles.NewJSON()
	if err := sut.Add(roles.Instance{"role-1", "powerflex", "542", "bronze", "100G"}); err != nil {
		t.Fatal(err)
	}
	if err := sut.Add(roles.Instance{"role-2", "powerflex", "542", "bronze", "100G"}); err != nil {
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
	t.Run("new entry", func(t *testing.T) {
		sut := buildJSON(t)
		adding := roles.Instance{
			Name: "adding",
		}

		err := sut.Add(adding)
		if err != nil {
			t.Fatal(err)
		}

		got := len(sut.Instances())
		want := 3
		if got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	})
	t.Run("existing entry", func(t *testing.T) {
		sut := buildJSON(t)
		adding := roles.Instance{
			Name: "adding",
		}
		err := sut.Add(adding)
		if err != nil {
			t.Fatal(err)
		}

		err = sut.Add(adding)
		if err != nil {
			t.Fatal(err)
		}

		got := len(sut.Instances())
		want := 3
		if got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	})
}

func buildJSON(t *testing.T) *roles.JSON {
	payload := `
{ 
  "roles": {
      "OpenShiftMongo": {
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
