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

package proxy

// SystemConfig is a map of string keys to a Family of backend storage systems
type SystemConfig map[string]Family

// Family is map of string keys to a SystemEntry
type Family map[string]SystemEntry

// SystemEntry holds information for a backend storage system
type SystemEntry struct {
	Endpoint string `json:"endpoint"`
	User     string `json:"user"`
	Password string `json:"password"`
	Insecure bool   `json:"insecure"`
}
