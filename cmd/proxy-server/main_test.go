// Copyright © 2021 Dell Inc., or its subsidiaries. All Rights Reserved.
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

package main

import (
	"fmt"
	"karavi-authorization/internal/proxy"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func TestProxy(t *testing.T) {
	t.Skip("TODO")
}

func TestUpdateConfiguration(t *testing.T) {
	v := viper.New()
	v.Set("certificate.crtfile", "testCrtFile")
	v.Set("certificate.keyfile", "testKeyFile")
	v.Set("certificate.rootcertificate", "testRootCertificate")
	v.Set("web.jwtsigningsecret", "testSecret")

	oldCfg := cfg
	cfg = Config{}

	oldJWTSigningSecret := JWTSigningSecret

	defer func() {
		cfg = oldCfg
		JWTSigningSecret = oldJWTSigningSecret
	}()

	updateConfiguration(v, logrus.NewEntry(logrus.StandardLogger()))

	if JWTSigningSecret != "testSecret" {
		t.Errorf("expeted web.jwtsigningsecret to be %v, got %v", "testSecret", JWTSigningSecret)
	}
}

func TestUpdateStorageSystems(t *testing.T) {
	// define the check function that will pass or fail tests
	type checkFn func(t *testing.T, err error,
		powerScaleSystems map[string]*proxy.PowerScaleSystem,
		powerFlexSystems map[string]*proxy.System,
		powerMaxSystems map[string]*proxy.PowerMaxSystem,
	)

	// define the tests
	tests := []struct {
		name               string
		storageSystemsFile string // file name in testdata folder
		checkFn            checkFn
	}{
		{
			"success",
			"storage-systems.yaml",
			func(t *testing.T, err error, powerScaleSystems map[string]*proxy.PowerScaleSystem, powerFlexSystems map[string]*proxy.System, powerMaxSystems map[string]*proxy.PowerMaxSystem) {
				if err != nil {
					t.Errorf("expected nil error, got %v", err)
				}

				if _, ok := powerScaleSystems["IsilonClusterName"]; !ok {
					t.Error("expected powerScale IsilonClusterName to be configured")
				}
				if _, ok := powerScaleSystems["isilonclustername"]; !ok {
					t.Error("expected powerScale isilonclustername to be configured")
				}

				if _, ok := powerFlexSystems["542a2d5f5122210f"]; !ok {
					t.Error("expected powerFlex 542a2d5f5122210f to be configured")
				}

				if _, ok := powerMaxSystems["1234567890"]; !ok {
					t.Error("expected powerMax 1234567890 to be configured")
				}
			},
		},
	}

	// run the tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			logger := logrus.NewEntry(logrus.New())

			powerScaleHandler := proxy.NewPowerScaleHandler(logger, nil, "")
			powerFlexHandler := proxy.NewPowerFlexHandler(logger, nil, "")
			powerMaxHandler := proxy.NewPowerMaxHandler(logger, nil, "")

			// When
			err := updateStorageSystems(logger, fmt.Sprintf("testdata/%s", tc.storageSystemsFile), powerFlexHandler, powerMaxHandler, powerScaleHandler)

			// Then
			tc.checkFn(t, err, powerScaleHandler.GetSystems(), powerFlexHandler.GetSystems(), powerMaxHandler.GetSystems())
		})
	}
}
