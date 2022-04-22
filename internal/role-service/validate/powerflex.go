package validate

import (
	"context"
	"errors"
	"fmt"
	"karavi-authorization/internal/types"
	"net/url"

	"github.com/dell/goscaleio"
)

// GetPowerFlexEndpoint returns the endpoint URL for a PowerFlex system
var GetPowerFlexEndpoint = func(system types.System) string {
	return system.Endpoint
}

func ValidatePowerFlex(ctx context.Context, system types.System, systemId string, pool string, quota int64) error {
	endpoint := GetPowerFlexEndpoint(system)
	epURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint is invalid: %+v", err)
	}

	epURL.Scheme = "https"
	//log.Println(system.Insecure)
	powerFlexClient, err := goscaleio.NewClientWithArgs(epURL.String(), "", system.Insecure, false)
	if err != nil {
		return fmt.Errorf("powerflex client is not available: %+v", err)
	}

	_, err = powerFlexClient.Authenticate(&goscaleio.ConfigConnect{
		Username: system.User,
		Password: system.Password,
	})

	if err != nil {
		return fmt.Errorf("powerflex authentication failed: %+v", err)
	}

	storagePool, err := getPowerFlexStoragePool(powerFlexClient, systemId, pool)
	if err != nil {
		return err
	}

	// Ensuring that the storage pool exists
	_, err = storagePool.GetStatistics()
	if err != nil {
		return err
	}

	if quota < 0 {
		return errors.New("the specified quota needs to be a positive number")
	}
	return nil
}

func getPowerFlexStoragePool(powerFlexClient *goscaleio.Client, storageSystemID string, storagePoolName string) (*goscaleio.StoragePool, error) {
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
