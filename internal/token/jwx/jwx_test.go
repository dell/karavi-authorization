package jwx_test

import (
	"karavi-authorization/internal/token"
	"karavi-authorization/internal/token/jwx"
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

		if v, ok := err.(*token.ErrExpired); !ok {
			t.Errorf("got err type %T, want %T", v, token.ErrExpired{})
		}
	})
}
