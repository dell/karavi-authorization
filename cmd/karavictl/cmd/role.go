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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

// PoolQuota contains the storage pool name and quota for the pool
type PoolQuota struct {
	Pool  string `json:"pool"`
	Quota int64  `json:"quota"`
}

// Role contains a storage system ID and slice of pool quotas for the role
type Role struct {
	StorageSystemID string      `json:"storage_system_id"`
	PoolQuotas      []PoolQuota `json:"pool_quotas"`
}

// roleCmd represents the role command
var roleCmd = &cobra.Command{
	Use:   "role",
	Short: "Manage roles",
	Long:  `Manage roles`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := cmd.Usage(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %+v\n", err)
		}
		os.Exit(1)
	},
}

func init() {
	rootCmd.AddCommand(roleCmd)
}

type Storage map[string]SystemType
type SystemType map[string]System

// GetAuthorizedStorageSystems returns list of storage systems added to authorization
func GetAuthorizedStorageSystems() (map[string]Storage, error) {
	k3sCmd := exec.Command("k3s", "kubectl", "get",
		"--namespace=karavi",
		"--output=json",
		"secret/karavi-storage-secret")

	b, err := k3sCmd.Output()
	if err != nil {
		return nil, err
	}

	base64Systems := struct {
		Data map[string]string
	}{}

	if err := json.Unmarshal(b, &base64Systems); err != nil {
		return nil, err
	}

	decodedSystems, err := base64.StdEncoding.DecodeString(base64Systems.Data["storage-systems.yaml"])
	if err != nil {
		return nil, err
	}

	var listData map[string]Storage
	if err := yaml.Unmarshal(decodedSystems, &listData); err != nil {
		return nil, err
	}

	return listData, nil
}
