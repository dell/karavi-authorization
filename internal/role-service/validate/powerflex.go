package validate

import (
	"context"
	"errors"
	"fmt"
	"karavi-authorization/internal/types"
	"net/url"

	"github.com/dell/goscaleio"
	"github.com/sirupsen/logrus"
)

// GetPowerFlexEndpoint returns the endpoint URL for a PowerFlex system
var GetPowerFlexEndpoint = func(system types.System) string {
	return system.Endpoint
}

func ValidatePowerFlex(ctx context.Context, log *logrus.Entry, system types.System, systemId string, pool string, quota int64) error {
	if quota < 0 {
		return errors.New("the specified quota needs to be a positive number")
	}

	endpoint := GetPowerFlexEndpoint(system)

	log.WithFields(logrus.Fields{
		"Endpoint": endpoint,
	}).Debugf("Parsing system endpoint")

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
		return fmt.Errorf("failed to connect to powerflex %s: %+v", systemId, err)
	}

	_, err = powerFlexClient.Authenticate(&goscaleio.ConfigConnect{
		Username: system.User,
		Password: system.Password,
	})

	if err != nil {
		return fmt.Errorf("powerflex authentication failed: %+v", err)
	}

	log.WithFields(logrus.Fields{
		"SystemId":    systemId,
		"StoragePool": pool,
	}).Debug("Validating storage pool existence on PowerFlex")

	storagePool, err := getPowerFlexStoragePool(powerFlexClient, systemId, pool)
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
