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

package validate

import (
	"context"
	"errors"
	"fmt"
	storage "karavi-authorization/cmd/karavictl/cmd"
	"net/url"

	pmax "github.com/dell/gopowermax/v2"
	"github.com/sirupsen/logrus"
)

// GetPowerMaxEndpoint returns the endpoint URL for a PowerMax system
var GetPowerMaxEndpoint = func(storageSystemDetails storage.System) string {
	return storageSystemDetails.Endpoint
}

// PowerMax validates powermax role parameters
func PowerMax(ctx context.Context, log *logrus.Entry, system storage.System, systemID string, pool string, quota int64) error {
	if quota < 0 {
		return errors.New("the specified quota needs to be a positive number")
	}

	endpoint := GetPowerMaxEndpoint(system)

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
	}).Debug("Establishing connection to PowerMax")

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

	log.WithFields(logrus.Fields{
		"SystemId":    systemID,
		"StoragePool": pool,
	}).Debug("Validating storage pool existence on PowerMax")

	_, err = powerMaxClient.GetStoragePool(ctx, systemID, pool)
	if err != nil {
		return err
	}

	return nil
}
