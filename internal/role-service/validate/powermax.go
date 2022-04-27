package validate

import (
	"context"
	"errors"
	"fmt"
	"karavi-authorization/internal/types"
	"net/url"

	pmax "github.com/dell/gopowermax"
	"github.com/sirupsen/logrus"
)

// GetPowerMaxEndpoint returns the endpoint URL for a PowerMax system
var GetPowerMaxEndpoint = func(storageSystemDetails types.System) string {
	return storageSystemDetails.Endpoint
}

// PowerMax validates powermax role parameters
func PowerMax(ctx context.Context, log *logrus.Entry, system types.System, systemID string, pool string, quota int64) error {
	if quota < 0 {
		return errors.New("the specified quota needs to be a positive number")
	}

	endpoint := GetPowerMaxEndpoint(system)

	log.WithFields(logrus.Fields{
		"Endpoint": endpoint,
	}).Debugf("Parsing system endpoint")

	epURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint is invalid: %+v", err)
	}

	log.WithFields(logrus.Fields{
		"Endpoint": epURL,
		"Insecure": system.Insecure,
	}).Debug("Establishing connection to PowerMax")

	epURL.Scheme = "https"
	powerMaxClient, err := pmax.NewClientWithArgs(epURL.String(), "", "", true, false)
	if err != nil {
		return err
	}
	err = powerMaxClient.Authenticate(ctx, &pmax.ConfigConnect{
		Username: system.User,
		Password: system.Password,
	})
	if err != nil {
		return fmt.Errorf("powermax authentication failed: %+v", err)
	}

	log.WithFields(logrus.Fields{
		"SystemId":    systemID,
		"StoragePool": pool,
	}).Debug("Validating storage pool existence on PowerMax")

	_, err = powerMaxClient.GetStoragePool(ctx, systemID, pool)
	if err != nil {
		return err
	}

	return nil
}
