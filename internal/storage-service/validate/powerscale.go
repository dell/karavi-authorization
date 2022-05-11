package validate

import (
	"context"
	"fmt"
	"karavi-authorization/internal/types"
	"log"
	"net/url"

	pscale "github.com/dell/goisilon"
)

// GetPowerScaleEndpoint returns the endpoint URL for a PowerScale system
var GetPowerScaleEndpoint = func(storageSystemDetails types.System) string {
	return storageSystemDetails.Endpoint
}

func ValidatePowerScale(ctx context.Context, system types.System, systemID string) error {

	endpoint := GetPowerScaleEndpoint(system)
	epURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint is invalid: %+v", err)
	}
	log.Print(system.Insecure)

	epURL.Scheme = "https"
	psClient, err := pscale.NewClientWithArgs(context.Background(), epURL.String(), system.Insecure, 1, system.User, "Administrators", system.Password, "", "777", 0)
	if err != nil {
		return fmt.Errorf("failed to connect to powerscale %s: %+v", systemID, err)
	}

	clusterConfig, err := psClient.GetClusterConfig(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get cluster config: %+v", err)
	}

	if clusterConfig.Name != systemID {
		return fmt.Errorf("cluster name %s not found", systemID)
	}

	return nil
}
