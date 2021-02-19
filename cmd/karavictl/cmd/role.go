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
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"karavi-authorization/cmd/karavictl/cmd/types"
	"log"
	"net/url"
	"os"
	"os/exec"
	"strings"

	"github.com/dell/goscaleio"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

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

// GetRoles returns all of the roles with associated storage systems, storage pools, and quotas
func GetRoles() (map[string][]types.Role, error) {

	ctx := context.Background()
	k3sCmd := execCommandContext(ctx, "k3s", "kubectl", "get",
		"--namespace=karavi",
		"--output=json",
		"configmap/common")

	b, err := k3sCmd.Output()
	if err != nil {
		log.Printf("ERROR: %v", err)

		return nil, err
	}

	base64Systems := struct {
		Data map[string]string
	}{}

	if err := json.Unmarshal(b, &base64Systems); err != nil {
		return nil, err
	}

	rolesRego := base64Systems.Data["common.rego"]
	if err != nil {
		return nil, err
	}

	rolesJSON := strings.Replace(rolesRego, "package karavi.common\ndefault roles = {}\nroles = ", "", 1)

	log.Printf("rolesJSON = %s\n", rolesJSON)
	var listData map[string][]types.Role
	if err := yaml.Unmarshal([]byte(rolesJSON), &listData); err != nil {
		return nil, err
	}

	return listData, nil
}

func validatePFpools(s System, sysID string, pqs []types.PoolQuota) error {
	poolSearch := make(map[string]int64) // they key is pool name and value is quota
	for _, pq := range pqs {
		poolSearch[pq.Pool] = pq.Quota
	}
	numValidate := 0

	// Set Up PowerStore
	epURL, err := url.Parse(s.Endpoint)
	if err != nil {
		fmt.Errorf("endpoint is invalid: %+v", err)
	}
	epURL.Scheme = "https"
	pfc, err := goscaleio.NewClientWithArgs(epURL.String(), "", true, false)
	if err != nil {
		fmt.Errorf("powerflex client is not available: %+v", err)
	}

	_, err = pfc.Authenticate(&goscaleio.ConfigConnect{
		Username: s.User,
		Password: s.Pass,
	})
	if err != nil {
		fmt.Errorf("powerflex authentication failed: %+v", err)
	}

	// get storage systems via protected domains
	pfs, err := pfc.FindSystem(sysID, "", "")
	if err != nil {
		return fmt.Errorf("the sytem ID: %s was not found in actual powerflex: %+v", sysID, err)
	}
	pfpds, err := pfs.GetProtectionDomain("")
	if err != nil {
		return fmt.Errorf("failed to get powerflex protected domain: %+v", err)
	}

	for _, pfpd := range pfpds {
		pfpdEX := goscaleio.NewProtectionDomainEx(pfc, pfpd)
		pfpools, err := pfpdEX.GetStoragePool("")
		if err != nil {
			return fmt.Errorf("failed to get pool from storage system: %+v", err)
		}
		for _, pfpool := range pfpools {
			if qt, ok := poolSearch[pfpool.Name]; ok {
				pfpc := goscaleio.NewStoragePoolEx(pfc, pfpool)
				stat, _ := pfpc.GetStatistics()
				if int(qt) > stat.SpareCapacityInKb {
					return errors.New("the specified quota is larger than the storage capacity")
				}
				numValidate++
			}

		}
	}
	if numValidate != len(pqs) {
		return errors.New("one or more specified pools do exist on the given storage system")
	}
	return nil
}

func ValidateRole(role types.Role) error {
	listData, err := GetAuthorizedStorageSystems()
	if err != nil {
		return fmt.Errorf("failed to get authorized storage systems: %+v", err)
	}

	for arrayType, storageTypes := range listData["storage"] {
		if _, ok := storageTypes[role.StorageSystemID]; !ok {
			return errors.New("storage systems does not exit and/or is not authorized")
		}

		poolSearch := make(map[string]int64) // they key is pool name and value is quota
		for _, poolQuota := range role.PoolQuotas {
			poolSearch[poolQuota.Pool] = poolQuota.Quota
		}

		switch arrayType {
		case "powerflex":
			return validatePFpools(storageTypes[role.StorageSystemID], role.StorageSystemID, role.PoolQuotas)
		default:
			return fmt.Errorf("%s is not supported", arrayType)
		}
	}
	return errors.New("failed validation")

}
