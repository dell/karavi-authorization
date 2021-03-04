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
	"fmt"
	"karavi-authorization/internal/token"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
)

const secret = "secret"

func TestCreateAsK8sSecret(t *testing.T) {
	t.Run("it creates a secret as a k8s secret", func(t *testing.T) {
		cfg := testBuildTokenConfig()

		got, err := token.CreateAsK8sSecret(cfg)
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

		_, err := token.CreateAsK8sSecret(cfg)

		want := token.ErrBlankSecretNotAllowed
		if got := err; got != want {
			t.Errorf("got err = %+v, want %+v", got, want)
		}
	})
}

func TestCreate(t *testing.T) {
	t.Run("it creates a token", func(t *testing.T) {
		cfg := testBuildTokenConfig()

		got, err := token.Create(cfg)
		if err != nil {
			t.Fatal(err)
		}

		if got := testDecodeJWT(t, got.Access); !got.Valid {
			t.Errorf("Access: got invalid token %+v, want valid token", got)
		}
		if got := testDecodeJWT(t, got.Refresh); !got.Valid {
			t.Errorf("Refresh: got invalid token %+v, want valid token", got)
		}
	})
	t.Run("it requires a non-blank secret", func(t *testing.T) {
		cfg := testBuildTokenConfig()
		cfg.JWTSigningSecret = "  "

		_, err := token.Create(cfg)

		want := token.ErrBlankSecretNotAllowed
		if got := err; got != want {
			t.Errorf("got err = %+v, want %+v", got, want)
		}
	})
}

func testBuildTokenConfig() token.Config {
	return token.Config{
		Tenant:            "tenant",
		Roles:             []string{"role"},
		JWTSigningSecret:  secret,
		RefreshExpiration: time.Hour,
		AccessExpiration:  time.Minute,
	}
}

func testDecodeJWT(t *testing.T, token string) *jwt.Token {
	t.Helper()
	parsedToken, err := jwt.Parse(token, func(tk *jwt.Token) (interface{}, error) {
		if _, ok := tk.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected JWT signing method: %v", tk.Header["alg"])
		}
		return []byte(secret), nil
	})
	if err != nil {
		t.Fatal(err)
	}

	return parsedToken
}
