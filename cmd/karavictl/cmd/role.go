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

	pscale "github.com/dell/goisilon"
	pmax "github.com/dell/gopowermax"
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

// NewRoleCmd creates a new role command
func NewRoleCmd() *cobra.Command {
	roleCmd := &cobra.Command{
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

	roleCmd.AddCommand(NewRoleCreateCmd())
	roleCmd.AddCommand(NewRoleDeleteCmd())
	roleCmd.AddCommand(NewRoleGetCmd())
	roleCmd.AddCommand(NewRoleListCmd())
	roleCmd.AddCommand(NewRoleUpdateCmd())
	return roleCmd
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

// GetPowerMaxEndpoint returns the endpoint URL for a PowerMax system
var GetPowerMaxEndpoint = func(storageSystemDetails System) string {
	return storageSystemDetails.Endpoint
}

// GetPowerScaleEndpoint returns the endpoint URL for a PowerMax system
var GetPowerScaleEndpoint = func(storageSystemDetails System) string {
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

func validatePowerMaxStorageResourcePool(storageSystemDetails System, storageSystemID string, poolQuota PoolQuota) error {
	endpoint := GetPowerMaxEndpoint(storageSystemDetails)
	epURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint is invalid: %+v", err)
	}

	epURL.Scheme = "https"
	//TODO(aaron): how should the version (90, 91) be determined?
	powerMaxClient, err := pmax.NewClientWithArgs(epURL.String(), "", "", true, false)
	if err != nil {
		return err
	}
	err = powerMaxClient.Authenticate(&pmax.ConfigConnect{
		Username: storageSystemDetails.User,
		Password: storageSystemDetails.Password,
	})
	if err != nil {
		return fmt.Errorf("powermax authentication failed: %+v", err)
	}
	_, err = powerMaxClient.GetStoragePool(storageSystemID, poolQuota.Pool)
	if err != nil {
		return err
	}

	if int(poolQuota.Quota) < 0 {
		return errors.New("the specified quota needs to be a positive number")
	}
	return nil
}

func validatePowerScaleIsiPath(storageSystemDetails System, storageSystemID string, poolQuota PoolQuota) error {
	endpoint := GetPowerScaleEndpoint(storageSystemDetails)
	epURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint is invalid: %+v", err)
	}

	epURL.Scheme = "https"
	//TODO(aaron): revert goisilon to public version
	c, err := pscale.NewClientWithArgs(context.Background(), epURL.String(), storageSystemDetails.Insecure, 1, storageSystemDetails.User, "Administrators", storageSystemDetails.Password, "", "777")
	if err != nil {
		return fmt.Errorf("powerscale authentication failed: %+v", err)
	}

	//pool := fmt.Sprintf("/%s", strings.Join(strings.Split(poolQuota.Pool, "-"), "/"))
	//pool := fmt.Sprintf("/%s", strings.Replace(poolQuota.Pool, "//", "/", -1))
	//pool := strings.Replace(poolQuota.Pool, "/", `\/`, -1)
	if _, err := c.GetVolumeWithIsiPath(context.Background(), poolQuota.Pool, "", ""); err != nil {
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
	if !validSystemType(role.SystemType) {
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
	case "powermax":
		err := validatePowerMaxStorageResourcePool(storageSystemDetails, role.SystemID, PoolQuota{
			Pool:  role.Pool,
			Quota: int64(role.Quota),
		})
		if err != nil {
			return err
		}
	case "powerscale":
		err := validatePowerScaleIsiPath(storageSystemDetails, role.SystemID, PoolQuota{
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

func validSystemType(sysType string) bool {
	validSystemTypes := []string{"powerflex", "powermax", "powerscale"}

	for _, s := range validSystemTypes {
		if sysType == s {
			return true
		}
	}
	return false
}
