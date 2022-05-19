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

	"github.com/dell/goscaleio"
	"github.com/sirupsen/logrus"
)

// GetPowerFlexEndpoint returns the endpoint URL for a PowerFlex system
var GetPowerFlexEndpoint = func(system types.System) string {
	return system.Endpoint
}

// PowerFlex validates powerflex storage parameters
func PowerFlex(ctx context.Context, log *logrus.Entry, system types.System, systemID string) error {

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
