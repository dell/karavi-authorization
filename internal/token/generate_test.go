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

package token_test

import (
	"bytes"
	"karavi-authorization/internal/token"
	jwtInternal "karavi-authorization/internal/token/jwt"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/jwa"
	jwt "github.com/lestrrat-go/jwx/jwt"
)

const secret = "secret"

func TestCreateAsK8sSecret(t *testing.T) {
	t.Run("it creates a secret as a k8s secret", func(t *testing.T) {
		cfg := testBuildTokenConfig()

		got, err := token.CreateAsK8sSecret(&jwtInternal.JWXTokenManager{}, cfg)
		if err != nil {
			t.Fatal(err)
		}

		if !bytes.Contains([]byte(got), []byte("apiVersion")) {
			t.Errorf("got %q, want something k8s-secret like", got)
		}
	})
	t.Run("it requires a non-blank secret", func(t *testing.T) {
		cfg := testBuildTokenConfig()
		cfg.JWTSigningSecret = "  "

		_, err := token.CreateAsK8sSecret(&jwtInternal.JWXTokenManager{}, cfg)

		want := token.ErrBlankSecretNotAllowed
		if got := err; got != want {
			t.Errorf("got err = %+v, want %+v", got, want)
		}
	})
}

func TestCreate(t *testing.T) {
	cfg := testBuildTokenConfig()

	tests := []struct {
		name         string
		tm           token.TokenManager
		validTokenFn func(*testing.T, string) error
	}{
		{
			"jwx",
			&jwtInternal.JWXTokenManager{},
			testDecodeJWX,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := token.Create(test.tm, cfg)
			if err != nil {
				t.Fatal(err)
			}

			if err := test.validTokenFn(t, got.Access); err != nil {
				t.Errorf("Access: got invalid token: %+v", err)
			}

			if err := test.validTokenFn(t, got.Refresh); err != nil {
				t.Errorf("Access: got invalid token: %+v", err)
			}
		})
	}
}

func TestCreateError(t *testing.T) {
	cfg := testBuildTokenConfig()
	cfg.JWTSigningSecret = "  "

	tests := []struct {
		name         string
		tm           token.TokenManager
		validTokenFn func(*testing.T, string) error
	}{
		{
			"jwx",
			&jwtInternal.JWXTokenManager{},
			testDecodeJWX,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := token.Create(test.tm, cfg)

			want := token.ErrBlankSecretNotAllowed
			if got := err; got != want {
				t.Errorf("got err = %+v, want %+v", got, want)
			}
		})
	}
}

func testBuildTokenConfig() jwtInternal.Config {
	return jwtInternal.Config{
		Tenant:            "tenant",
		Roles:             []string{"role"},
		JWTSigningSecret:  secret,
		RefreshExpiration: time.Hour,
		AccessExpiration:  time.Minute,
	}
}

func testDecodeJWX(t *testing.T, token string) error {
	t.Helper()

	_, err := jwt.ParseString(token, jwt.WithVerify(jwa.HS256, []byte(secret)), jwt.WithValidate(true))
	if err != nil {
		return err
	}

	return nil
}
