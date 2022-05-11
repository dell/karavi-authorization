package validate

import (
	"context"
	"fmt"
	"karavi-authorization/internal/types"
	"net/url"

	pmax "github.com/dell/gopowermax"
)

// GetPowerMaxEndpoint returns the endpoint URL for a PowerMax system
var GetPowerMaxEndpoint = func(storageSystemDetails types.System) string {
	return storageSystemDetails.Endpoint
}

func ValidatePowerMax(ctx context.Context, system types.System, systemId string) error {

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

	return nil
}
