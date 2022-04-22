package validate

import (
	"context"
	"errors"
	"fmt"
	"karavi-authorization/internal/types"
	"net/url"

	pscale "github.com/dell/goisilon"
)

// GetPowerScaleEndpoint returns the endpoint URL for a PowerScale system
var GetPowerScaleEndpoint = func(storageSystemDetails types.System) string {
	return storageSystemDetails.Endpoint
}

func ValidatePowerScale(ctx context.Context, system types.System, systemId string, pool string, quota int64) error {
	endpoint := GetPowerScaleEndpoint(system)
	epURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint is invalid: %+v", err)
	}

	epURL.Scheme = "https"
	c, err := pscale.NewClientWithArgs(ctx, epURL.String(), system.Insecure, 1, system.User, "Administrators", system.Password, "", "777", 0)
	if err != nil {
		return fmt.Errorf("powerscale authentication failed: %+v", err)
	}

	if _, err := c.GetVolumeWithIsiPath(ctx, pool, "", ""); err != nil {
		return err
	}

	if quota != 0 {
		return errors.New("quota must be 0 as it is not enforced by CSM-Authorization")
	}

	return nil
}
