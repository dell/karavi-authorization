// Copyright © 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
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

package validate

import (
	"context"
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

// PowerScale validates powerscale storage parameters
func PowerScale(ctx context.Context, log *logrus.Entry, system types.System, systemID string) error {

	endpoint := GetPowerScaleEndpoint(system)
	epURL, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("endpoint is invalid: %+v", err)
	}

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
