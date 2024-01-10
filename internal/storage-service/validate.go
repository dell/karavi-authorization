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

package storage

import (
	"context"
	"fmt"
	storage "karavi-authorization/cmd/karavictl/cmd"
	"net/url"

	pscale "github.com/dell/goisilon"
	pmax "github.com/dell/gopowermax/v2"
	"github.com/dell/goscaleio"
	"github.com/sirupsen/logrus"
)

// SystemValidator validates a storage instance
type SystemValidator struct {
	kube Kube
	log  *logrus.Entry
}

// GetPowerFlexEndpoint returns the endpoint URL for a PowerFlex system
var GetPowerFlexEndpoint = func(system storage.System) string {
	return system.Endpoint
}

// GetPowerMaxEndpoint returns the endpoint URL for a PowerMax system
var GetPowerMaxEndpoint = func(storageSystemDetails storage.System) string {
	return storageSystemDetails.Endpoint
}

// GetPowerScaleEndpoint returns the endpoint URL for a PowerScale system
var GetPowerScaleEndpoint = func(storageSystemDetails storage.System) string {
	return storageSystemDetails.Endpoint
}

// NewSystemValidator returns a SystemValidator
func NewSystemValidator(kube Kube, log *logrus.Entry) *SystemValidator {
	return &SystemValidator{
		kube: kube,
		log:  log,
	}
}

// Validate validates a storage instance
func (v *SystemValidator) Validate(ctx context.Context, systemID string, systemType string, system storage.System) error {
	v.log.Info("Validating storage")
	if !validSystemType(systemType) {
		return fmt.Errorf("system type %s is not supported", systemType)
	}

	switch systemType {
	case "powerflex":
		return validatePowerflex(ctx, v.log, system, systemID)
	case "powermax":
		return validatePowermax(ctx, v.log, system, systemID)
	case "powerscale":
		return validatePowerscale(ctx, v.log, system, systemID)
	default:
		return fmt.Errorf("system type %s is not supported", systemType)
	}
}

func validatePowerflex(_ context.Context, _ *logrus.Entry, system storage.System, systemID string) error {
	endpoint := GetPowerFlexEndpoint(system)
	epURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint %s is invalid: %+v", epURL, err)
	}

	epURL.Scheme = "https"
	powerFlexClient, err := goscaleio.NewClientWithArgs(epURL.String(), "", 0, system.Insecure, false)
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

func validatePowermax(ctx context.Context, _ *logrus.Entry, system storage.System, _ string) error {
	endpoint := GetPowerMaxEndpoint(system)
	epURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint is invalid: %+v", err)
	}

	epURL.Scheme = "https"
	powerMaxClient, err := pmax.NewClientWithArgs(epURL.String(), "CSM-Authz", true, false)
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

func validatePowerscale(_ context.Context, _ *logrus.Entry, system storage.System, systemID string) error {
	endpoint := GetPowerScaleEndpoint(system)
	epURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint is invalid: %+v", err)
	}

	epURL.Scheme = "https"
	psClient, err := pscale.NewClientWithArgs(context.Background(), epURL.String(), system.Insecure, uint(1), system.User, "Administrators", system.Password, "", "777", false, uint8(0))
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

func validSystemType(sysType string) bool {
	for k := range storage.SupportedStorageTypes {
		if sysType == k {
			return true
		}
	}
	return false
}
