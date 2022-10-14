/*
 Copyright © 2021-2022 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/
package main

import (
	"karavi-authorization/internal/tenantsvc"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func TestUpdateConfiguration(t *testing.T) {
	v := viper.New()
	v.Set("web.jwtsigningsecret", "testSecret")

	oldCfg := cfg
	cfg = Config{}

	oldJWTSigningSecret := tenantsvc.JWTSigningSecret

	defer func() {
		cfg = oldCfg
		tenantsvc.JWTSigningSecret = oldJWTSigningSecret
	}()

	updateConfiguration(v, logrus.NewEntry(logrus.StandardLogger()))

	if tenantsvc.JWTSigningSecret != "testSecret" {
		t.Errorf("expeted web.jwtsigningsecret to be %v, got %v", "testSecret", tenantsvc.JWTSigningSecret)
	}
}
