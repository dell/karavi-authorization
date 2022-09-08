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

	pmax "github.com/dell/gopowermax/v2"
	"github.com/sirupsen/logrus"
)

// GetPowerMaxEndpoint returns the endpoint URL for a PowerMax system
var GetPowerMaxEndpoint = func(storageSystemDetails types.System) string {
	return storageSystemDetails.Endpoint
}

// PowerMax validates powermax storage parameters
func PowerMax(ctx context.Context, log *logrus.Entry, system types.System, systemID string) error {

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
