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
	"karavi-authorization/internal/web"
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
	v.Set("web.sidecarproxyaddr", "127.0.0.1:5000/sidecar-proxy:test")
	v.Set("web.jwtsigningsecret", "testSecret")

	oldCfg := cfg
	cfg = Config{}

	oldInsecure := web.Insecure
	oldRootCert := web.RootCertificate
	oldSidecarProxyAddr := web.SidecarProxyAddr
	oldJWTSigningSecret := JWTSigningSecret

	defer func() {
		cfg = oldCfg
		web.Insecure = oldInsecure
		web.RootCertificate = oldRootCert
		web.SidecarProxyAddr = oldSidecarProxyAddr
		JWTSigningSecret = oldJWTSigningSecret
	}()

	updateConfiguration(v, logrus.NewEntry(logrus.StandardLogger()))

	if web.Insecure != false {
		t.Errorf("expeted web.Insecure to be %v, got %v", false, web.Insecure)
	}

	if web.RootCertificate != "testRootCertificate" {
		t.Errorf("expeted web.RootCertificate to be %v, got %v", "testRootCertificate", web.RootCertificate)
	}

	if web.SidecarProxyAddr != "127.0.0.1:5000/sidecar-proxy:test" {
		t.Errorf("expeted web.sidecarproxyaddr to be %v, got %v", "127.0.0.1:5000/sidecar-proxy:test", web.SidecarProxyAddr)
	}

	if JWTSigningSecret != "testSecret" {
		t.Errorf("expeted web.jwtsigningsecret to be %v, got %v", "testSecret", JWTSigningSecret)
	}
}
