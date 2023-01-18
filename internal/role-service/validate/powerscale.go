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
	"errors"
	"fmt"
	storage "karavi-authorization/cmd/karavictl/cmd"
	"net/url"

	pscale "github.com/dell/goisilon"
	"github.com/sirupsen/logrus"
)

// GetPowerScaleEndpoint returns the endpoint URL for a PowerScale system
var GetPowerScaleEndpoint = func(storageSystemDetails storage.System) string {
	return storageSystemDetails.Endpoint
}

// PowerScale validates powerscale role parameters
func PowerScale(ctx context.Context, log *logrus.Entry, system storage.System, systemID string, pool string, quota int64) error {
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
	c, err := pscale.NewClientWithArgs(ctx, epURL.String(), system.Insecure, uint(1), system.User, "Administrators", system.Password, "", "777", false, uint8(0))
	if err != nil {
		return fmt.Errorf("powerscale authentication failed: %+v", err)
	}

	log.WithFields(logrus.Fields{
		"SystemId": systemID,
		"IsiPath":  pool,
	}).Debug("Validating isiPath existence on PowerScale")

	if _, err := c.GetVolumeWithIsiPath(ctx, pool, "", ""); err != nil {
		return err
	}

	return nil
}
