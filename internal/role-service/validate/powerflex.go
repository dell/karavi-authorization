// Copyright © 2021-2023 Dell Inc., or its subsidiaries. All Rights Reserved.
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

package validate

import (
	"context"
	"errors"
	"fmt"
	storage "karavi-authorization/cmd/karavictl/cmd"
	"net/url"

	"github.com/dell/goscaleio"
	"github.com/sirupsen/logrus"
)

// GetPowerFlexEndpoint returns the endpoint URL for a PowerFlex system
var GetPowerFlexEndpoint = func(system storage.System) string {
	return system.Endpoint
}

// PowerFlex validates powerflex role parameters
func PowerFlex(ctx context.Context, log *logrus.Entry, system storage.System, systemID string, pool string, quota int64) error {
	if quota < 0 {
		return errors.New("the specified quota needs to be a positive number")
	}

	endpoint := GetPowerFlexEndpoint(system)
	epURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint %s is invalid: %+v", epURL, err)
	}

	log.WithFields(logrus.Fields{
		"Endpoint": epURL,
		"Insecure": system.Insecure,
	}).Debug("Establishing connection to PowerFlex")

	epURL.Scheme = "https"
	powerFlexClient, err := goscaleio.NewClientWithArgs(epURL.String(), "", system.Insecure, false)
	if err != nil {
		return fmt.Errorf("failed to connect to powerflex %s: %+v", systemID, err)
	}

	_, err = powerFlexClient.Authenticate(&goscaleio.ConfigConnect{
		Username: system.User,
		Password: system.Password,
	})

	if err != nil {
		return fmt.Errorf("powerflex authentication failed: %+v", err)
	}

	log.WithFields(logrus.Fields{
		"SystemId":    systemID,
		"StoragePool": pool,
	}).Debug("Validating storage pool existence on PowerFlex")

	storagePool, err := getPowerFlexStoragePool(powerFlexClient, systemID, pool)
	if err != nil {
		return err
	}

	// Ensuring that the storage pool exists
	_, err = storagePool.GetStatistics()
	if err != nil {
		return err
	}

	return nil
}

func getPowerFlexStoragePool(powerFlexClient *goscaleio.Client, storageSystemID string, storagePoolName string) (*goscaleio.StoragePool, error) {
	systems, err := powerFlexClient.FindSystem(storageSystemID, "", "")
	if err != nil {
		return nil, fmt.Errorf("sytem ID %s was not found on powerflex: %+v", storageSystemID, err)
	}

	protectionDomains, err := systems.GetProtectionDomain("")
	if err != nil {
		return nil, fmt.Errorf("failed to get powerflex protection domains: %+v", err)
	}

	for _, protectionDomain := range protectionDomains {
		protectionDomainRef := goscaleio.NewProtectionDomainEx(powerFlexClient, protectionDomain)
		protectionDomainStoragePools, err := protectionDomainRef.GetStoragePool("")
		if err != nil {
			return nil, fmt.Errorf("failed to get storage pool from protection domain %s: %+v", protectionDomainRef.ProtectionDomain.Name, err)
		}
		for _, protectionDomainStoragePool := range protectionDomainStoragePools {
			if protectionDomainStoragePool.Name == storagePoolName {
				storagePool := goscaleio.NewStoragePoolEx(powerFlexClient, protectionDomainStoragePool)
				return storagePool, nil
			}
		}
	}

	return nil, fmt.Errorf("unable to find storage pool %s on powerflex %s", storagePoolName, storageSystemID)
}
