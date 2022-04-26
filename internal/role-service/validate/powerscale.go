package validate

import (
	"context"
	"errors"
	"fmt"
	"karavi-authorization/internal/types"
	"net/url"

	pscale "github.com/dell/goisilon"
	"github.com/sirupsen/logrus"
)

// GetPowerScaleEndpoint returns the endpoint URL for a PowerScale system
var GetPowerScaleEndpoint = func(storageSystemDetails types.System) string {
	return storageSystemDetails.Endpoint
}

func ValidatePowerScale(ctx context.Context, log *logrus.Entry, system types.System, systemId string, pool string, quota int64) error {
	if quota != 0 {
		return errors.New("quota must be 0 as it is not enforced by CSM-Authorization")
	}

	endpoint := GetPowerScaleEndpoint(system)

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
	}).Debug("Establishing connection to PowerScale")

	epURL.Scheme = "https"
	log.Println(system)
	c, err := pscale.NewClientWithArgs(ctx, epURL.String(), system.Insecure, 1, system.User, "Administrators", system.Password, "", "777", 0)
	if err != nil {
		return fmt.Errorf("powerscale authentication failed: %+v", err)
	}

	log.WithFields(logrus.Fields{
		"SystemId": systemId,
		"IsiPath":  pool,
	}).Debug("Validating isiPath existence on PowerScale")

	if _, err := c.GetVolumeWithIsiPath(ctx, pool, "", ""); err != nil {
		return err
	}

	return nil
}
