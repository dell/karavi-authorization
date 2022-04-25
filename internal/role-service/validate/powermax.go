package validate

import (
	"context"
	"errors"
	"fmt"
	"karavi-authorization/internal/types"
	"net/url"

	pmax "github.com/dell/gopowermax"
)

// GetPowerMaxEndpoint returns the endpoint URL for a PowerMax system
var GetPowerMaxEndpoint = func(storageSystemDetails types.System) string {
	return storageSystemDetails.Endpoint
}

func ValidatePowerMax(ctx context.Context, system types.System, systemId string, pool string, quota int64) error {
	if quota < 0 {
		return errors.New("the specified quota needs to be a positive number")
	}

	endpoint := GetPowerMaxEndpoint(system)
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
	err = powerMaxClient.Authenticate(ctx, &pmax.ConfigConnect{
		Username: system.User,
		Password: system.Password,
	})
	if err != nil {
		return fmt.Errorf("powermax authentication failed: %+v", err)
	}
	_, err = powerMaxClient.GetStoragePool(ctx, systemId, pool)
	if err != nil {
		return err
	}

	return nil
}
