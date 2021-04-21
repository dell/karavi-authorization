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
	"karavi-authorization/internal/roles"
	"net/url"
	"os"
	"strings"

	"github.com/dell/goscaleio"
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
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("error: %+v", err))
		}
		os.Exit(1)
	},
}

func init() {
	rootCmd.AddCommand(roleCmd)
}

// GetAuthorizedStorageSystems returns list of storage systems added to authorization
func GetAuthorizedStorageSystems() (map[string]Storage, error) {
	k3sCmd := execCommandContext(context.Background(), K3sPath, "kubectl", "get",
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
func GetRoles() (*roles.JSON, error) {
	var existing roles.JSON

	ctx := context.Background()
	k3sCmd := execCommandContext(ctx, K3sPath, "kubectl", "get",
		"--namespace=karavi",
		"--output=json",
		"configmap/common")

	b, err := k3sCmd.Output()
	if err != nil {
		return nil, err
	}

	dataField := struct {
		Data map[string]string `json:"data"`
	}{}

	if err := json.Unmarshal(b, &dataField); err != nil {
		return nil, fmt.Errorf("unmarshalling dataField: %w", err)
	}

	rolesRego := dataField.Data["common.rego"]
	if err != nil {
		return nil, err
	}

	rolesJSON := strings.Replace(rolesRego, "package karavi.common\ndefault roles = {}\nroles = ", "", 1)

	dec := json.NewDecoder(strings.NewReader(rolesJSON))
	if err := dec.Decode(&existing); err != nil {
		return nil, fmt.Errorf("decoding roles json: %w", err)
	}

	return &existing, nil
}

// GetPowerFlexEndpoint returns the endpoint URL for a PowerFlex system
var GetPowerFlexEndpoint = func(storageSystemDetails System) string {
	return storageSystemDetails.Endpoint
}

func validatePowerFlexPool(storageSystemDetails System, storageSystemID string, poolQuota PoolQuota) error {
	endpoint := GetPowerFlexEndpoint(storageSystemDetails)
	epURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint is invalid: %+v", err)
	}

	epURL.Scheme = "https"
	powerFlexClient, err := goscaleio.NewClientWithArgs(epURL.String(), "", storageSystemDetails.Insecure, false)
	if err != nil {
		return fmt.Errorf("powerflex client is not available: %+v", err)
	}

	_, err = powerFlexClient.Authenticate(&goscaleio.ConfigConnect{
		Username: storageSystemDetails.User,
		Password: storageSystemDetails.Password,
	})

	if err != nil {
		return fmt.Errorf("powerflex authentication failed: %+v", err)
	}

	storagePool, err := getStoragePool(powerFlexClient, storageSystemID, poolQuota.Pool)
	if err != nil {
		return err
	}

    // Ensuring that the storage pool exists
	_, err = storagePool.GetStatistics()
	if err != nil {
		return err
	}

	if int(poolQuota.Quota) < 0 {
		return errors.New("the specified quota needs to be a positive number")
	}
	return nil
}

func getStoragePool(powerFlexClient *goscaleio.Client, storageSystemID string, storagePoolName string) (*goscaleio.StoragePool, error) {
	systems, err := powerFlexClient.FindSystem(storageSystemID, "", "")
	if err != nil {
		return nil, fmt.Errorf("the sytem ID: %s was not found in actual powerflex: %+v", storageSystemID, err)
	}

	protectionDomains, err := systems.GetProtectionDomain("")
	if err != nil {
		return nil, fmt.Errorf("failed to get powerflex protection domains: %+v", err)
	}

	for _, protectionDomain := range protectionDomains {
		protectionDomainRef := goscaleio.NewProtectionDomainEx(powerFlexClient, protectionDomain)
		protectionDomainStoragePools, err := protectionDomainRef.GetStoragePool("")
		if err != nil {
			return nil, fmt.Errorf("failed to get pool from storage system: %+v", err)
		}
		for _, protectionDomainStoragePool := range protectionDomainStoragePools {
			if protectionDomainStoragePool.Name == storagePoolName {
				storagePool := goscaleio.NewStoragePoolEx(powerFlexClient, protectionDomainStoragePool)
				return storagePool, nil
			}
		}
	}

	return nil, fmt.Errorf("unable to find storage pool with name %s on storage system %s", storagePoolName, storageSystemID)
}

func getStorageSystemDetails(storageSystemID string) (System, string, error) {
	authorizedSystems, err := GetAuthorizedStorageSystems()
	if err != nil {
		return System{}, "", fmt.Errorf("failed to get authorized storage systems: %+v", err)
	}

	for systemType, storageSystems := range authorizedSystems["storage"] {
		if _, ok := storageSystems[storageSystemID]; ok {
			return storageSystems[storageSystemID], systemType, nil
		}
	}
	return System{}, "", fmt.Errorf("unable to find authorized storage system with ID: %s", storageSystemID)
}

func validateRole(role *roles.Instance) error {
	if role.SystemType != "powerflex" {
		return fmt.Errorf("%s is not supported", role.SystemType)
	}

	storageSystemDetails, storageSystemType, err := getStorageSystemDetails(role.SystemID)
	if err != nil {
		return err
	}

	switch storageSystemType {
	case "powerflex":
		err := validatePowerFlexPool(storageSystemDetails, role.SystemID, PoolQuota{
			Pool:  role.Pool,
			Quota: int64(role.Quota),
		})
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s is not supported", storageSystemType)
	}

	return nil
}
