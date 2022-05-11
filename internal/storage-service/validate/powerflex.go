package validate

import (
	"context"
	"fmt"
	"karavi-authorization/internal/types"
	"net/url"

	"github.com/dell/goscaleio"
)

// GetPowerFlexEndpoint returns the endpoint URL for a PowerFlex system
var GetPowerFlexEndpoint = func(system types.System) string {
	return system.Endpoint
}

func ValidatePowerFlex(ctx context.Context, system types.System, systemID string) error {

	endpoint := GetPowerFlexEndpoint(system)
	epURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint %s is invalid: %+v", epURL, err)
	}

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

	return nil
}
