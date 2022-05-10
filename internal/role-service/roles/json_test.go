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

package roles

import (
	"encoding/json"
	"strings"
	"testing"
)

const ExpectedInstanceCount = 3

func TestReadableJSON_MarshalJSON(t *testing.T) {
	sut := buildJSON(t)

	_, err := json.Marshal(&sut)
	if err != nil {
		t.Fatal(err)
	}
}

func TestJSON_Unmarshal(t *testing.T) {
	sut := buildJSON(t)

	got := len(sut.m)

	want := ExpectedInstanceCount
	if got != want {
		t.Errorf("got %d, want %d", got, want)
	}
}

func buildJSON(t *testing.T) *ReadableJSON {
	payload := `
{
  "OpenShiftMongo": {
    "system_types": {
      "powerflex": {
        "system_ids": {
          "542a2d5f5122210f": {
            "pool_quotas": {
              "bronze": "4 GB",
			  "silver": "8 GB"
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
              "bronze": "4 GB"
            }
          }
        }
      }
    }
  }
}
`
	var sut ReadableJSON
	err := json.NewDecoder(strings.NewReader(payload)).Decode(&sut)
	if err != nil {
		t.Fatal(err)
	}

	return &sut
}
