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
