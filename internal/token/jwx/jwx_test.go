// Copyright © 2021-2022 Dell Inc., or its subsidiaries. All Rights Reserved.
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

package jwx_test

import (
	"context"
	"karavi-authorization/internal/token"
	"karavi-authorization/internal/token/jwx"
	"karavi-authorization/pb"
	"reflect"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwt"
)

func TestNewPair(t *testing.T) {
	tm := jwx.NewTokenManager(jwx.HS256)

	secret := "secret"

	cfg := token.Config{
		Tenant:            "tenant",
		Roles:             []string{"role"},
		JWTSigningSecret:  secret,
		RefreshExpiration: time.Hour,
		AccessExpiration:  time.Minute,
	}

	p, err := tm.NewPair(cfg)
	if err != nil {
		t.Fatal(err)
	}

	_, err = jwt.ParseString(p.Access, jwt.WithVerify(jwa.HS256, []byte(secret)), jwt.WithValidate(true))
	if err != nil {
		t.Errorf("Access: got invalid token: %+v", err)
	}

	_, err = jwt.ParseString(p.Refresh, jwt.WithVerify(jwa.HS256, []byte(secret)), jwt.WithValidate(true))
	if err != nil {
		t.Errorf("Refresh: got invalid token: %+v", err)
	}
}

func TestNewWithClaims(t *testing.T) {
	tm := jwx.NewTokenManager(jwx.HS256)

	want := token.Claims{
		Audience:  "karavi",
		ExpiresAt: 1915585883,
		Issuer:    "com.dell.karavi",
		Subject:   "karavi-tenant",
		Roles:     "CA-medium",
		Group:     "PancakeGroup",
	}

	token, err := tm.NewWithClaims(want)
	if err != nil {
		t.Fatal(err)
	}

	got, err := token.Claims()
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestParseWithClaims(t *testing.T) {
	t.Run("it parses a valid token", func(t *testing.T) {
		tm := jwx.NewTokenManager(jwx.HS256)
		secret := "secret"

		want := token.Claims{
			Audience:  "karavi",
			ExpiresAt: 1915585883,
			Issuer:    "com.dell.karavi",
			Subject:   "karavi-tenant",
			Roles:     "CA-medium",
			Group:     "PancakeGroup",
		}

		jwtToken, err := tm.NewWithClaims(want)
		if err != nil {
			t.Fatal(err)
		}
		tokenStr, err := jwtToken.SignedString(secret)
		if err != nil {
			t.Fatal(err)
		}

		var got token.Claims
		_, err = tm.ParseWithClaims(tokenStr, secret, &got)
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("it returns an expired error", func(t *testing.T) {
		tm := jwx.NewTokenManager(jwx.HS256)
		secret := "secret"

		want := token.Claims{
			Audience:  "karavi",
			ExpiresAt: 1114484883,
			Issuer:    "com.dell.karavi",
			Subject:   "karavi-tenant",
			Roles:     "CA-medium",
			Group:     "PancakeGroup",
		}

		jwtToken, err := tm.NewWithClaims(want)
		if err != nil {
			t.Fatal(err)
		}
		tokenStr, err := jwtToken.SignedString(secret)
		if err != nil {
			t.Fatal(err)
		}

		_, err = tm.ParseWithClaims(tokenStr, secret, &token.Claims{})

		if err == nil {
			t.Errorf("expected non-nil err")
		}

		if err != token.ErrExpired {
			t.Errorf("got %v, want %v", err, token.ErrExpired)
		}
	})
}

func TestGenerateAdminToken(t *testing.T) {
	got, err := jwx.GenerateAdminToken(context.Background(), &pb.GenerateAdminTokenRequest{
		AdminName:        "admin",
		JWTSigningSecret: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Token) == 0 {
		t.Errorf("got %q, want non-empty", got.Token)
	}
}
