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

package jwx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"karavi-authorization/internal/token"
	"karavi-authorization/pb"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
	"github.com/sirupsen/logrus"
)

// Manager implements the token.Manager API via github.com/lestrrat-go/jwx
type Manager struct {
	SigningAlgorithm jwa.SignatureAlgorithm
}

// Token implements the token.Token API via github.com/lestrrat-go/jwx
type Token struct {
	token            jwt.Token
	SigningAlgorithm jwa.SignatureAlgorithm
}

// SignatureAlgorithm is a wrapper for jwx signature algorithms
type SignatureAlgorithm jwa.SignatureAlgorithm

const (
	// HS256 is the HS256 signature algorithm from jwx
	HS256 = SignatureAlgorithm(jwa.HS256)
)

var (
	errExpiredMsg = "exp not satisfied"
	// JWTSigningSecret is the secret string used to sign JWT tokens
	JWTSigningSecret = "secret"
)

var (
	_ token.Manager = &Manager{}
	_ token.Token   = &Token{}
)

// NewTokenManager returns a Manager configured with the supplied signature algorithm
func NewTokenManager(alg SignatureAlgorithm) token.Manager {
	jwt.Settings(jwt.WithFlattenAudience(true))
	return &Manager{SigningAlgorithm: jwa.SignatureAlgorithm(alg)}
}

// NewPair returns a new access/refresh Pair
func (m *Manager) NewPair(cfg token.Config) (token.Pair, error) {
	t, err := tokenFromConfig(cfg)
	if err != nil {
		return token.Pair{}, err
	}

	key, err := jwk.New([]byte(cfg.JWTSigningSecret))
	if err != nil {
		return token.Pair{}, err
	}

	// Sign for an access token
	accessToken, err := jwt.Sign(t, m.SigningAlgorithm, key)
	if err != nil {
		return token.Pair{}, err
	}

	// Sign for a refresh token
	err = t.Set(jwt.ExpirationKey, time.Now().Add(cfg.RefreshExpiration).Unix())
	if err != nil {
		return token.Pair{}, err
	}

	refreshToken, err := jwt.Sign(t, jwa.HS256, key)
	if err != nil {
		return token.Pair{}, err
	}

	return token.Pair{
		Access:  string(accessToken),
		Refresh: string(refreshToken),
	}, nil
}

// NewWithClaims returns an unsigned Token configured with the supplied Claims
func (m *Manager) NewWithClaims(claims token.Claims) (token.Token, error) {
	t, err := tokenFromClaims(claims)
	if err != nil {
		return nil, err
	}

	return &Token{
		token:            t,
		SigningAlgorithm: m.SigningAlgorithm,
	}, nil
}

// ParseWithClaims verifies and validates a token and unmarshals it into the supplied Claims
func (m *Manager) ParseWithClaims(tokenStr string, secret string, claims *token.Claims) (token.Token, error) {
	// verify the token with the secret, but don't validate it yet so we can use the token
	verifiedToken, err := jwt.ParseString(tokenStr, jwt.WithVerify(m.SigningAlgorithm, []byte(secret)))
	if err != nil {
		return nil, fmt.Errorf("error verifying token: %v", err)
	}

	data, err := json.Marshal(verifiedToken)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal token: %v", err)
	}

	err = json.Unmarshal(data, &claims)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal token: %v", err)
	}

	// now validate the verified token
	t, err := jwt.ParseString(tokenStr, jwt.WithValidate(true))
	if err != nil {
		if strings.Contains(err.Error(), errExpiredMsg) {
			return nil, token.ErrExpired
		}
		return nil, fmt.Errorf("error validating token: %v", err)
	}

	return &Token{
		token:            t,
		SigningAlgorithm: m.SigningAlgorithm,
	}, nil
}

// SignedString returns a signed, serialized token with the supplied secret
func (t *Token) SignedString(secret string) (string, error) {
	key, err := jwk.New([]byte(secret))
	if err != nil {
		return "", err
	}

	token, err := jwt.Sign(t.token, t.SigningAlgorithm, key)
	if err != nil {
		return "", err
	}

	return string(token), nil
}

// Claims returns the Claims of a token
func (t *Token) Claims() (token.Claims, error) {
	data, err := json.Marshal(t.token)
	if err != nil {
		return token.Claims{}, err
	}

	var c token.Claims
	err = json.Unmarshal(data, &c)
	if err != nil {
		return token.Claims{}, err
	}

	return c, nil
}

func tokenFromConfig(cfg token.Config) (jwt.Token, error) {
	t := jwt.New()
	err := t.Set(jwt.IssuerKey, "com.dell.csm")
	if err != nil {
		return nil, err
	}

	err = t.Set(jwt.AudienceKey, "csm")
	if err != nil {
		return nil, err
	}

	if cfg.Subject == "admin" {
		err = t.Set(jwt.SubjectKey, "csm-admin")
		if err != nil {
			return nil, err
		}
		err = t.Set("group", cfg.AdminName)
		if err != nil {
			return nil, err
		}
	} else {
		err = t.Set(jwt.SubjectKey, "csm-tenant")
		if err != nil {
			return nil, err
		}
		err = t.Set("group", cfg.Tenant)
		if err != nil {
			return nil, err
		}
	}

	err = t.Set(jwt.ExpirationKey, time.Now().Add(cfg.AccessExpiration).Unix())
	if err != nil {
		return nil, err
	}

	err = t.Set("roles", strings.Join(cfg.Roles, ","))
	if err != nil {
		return nil, err
	}

	return t, nil
}

func tokenFromClaims(claims token.Claims) (jwt.Token, error) {
	t := jwt.New()
	err := t.Set(jwt.IssuerKey, claims.Issuer)
	if err != nil {
		return nil, err
	}

	err = t.Set(jwt.AudienceKey, claims.Audience)
	if err != nil {
		return nil, err
	}

	err = t.Set(jwt.SubjectKey, claims.Subject)
	if err != nil {
		return nil, err
	}

	err = t.Set(jwt.ExpirationKey, claims.ExpiresAt)
	if err != nil {
		return nil, err
	}

	err = t.Set("roles", claims.Roles)
	if err != nil {
		return nil, err
	}

	err = t.Set("group", claims.Group)
	if err != nil {
		return nil, err
	}

	return t, nil
}

// GenerateAdminToken generates a token for an admin. The returned token is
// in JSON format.
func GenerateAdminToken(_ context.Context, req *pb.GenerateAdminTokenRequest) (*pb.GenerateAdminTokenResponse, error) {
	tm := NewTokenManager(HS256)

	// Get the expiration values from config.
	if req.RefreshExpiration <= 0 {
		req.RefreshExpiration = int64(24 * time.Hour)
	}
	if req.AccessExpiration <= 0 {
		req.AccessExpiration = int64(30 * time.Minute)
	}

	// Generate the token.
	s, err := token.CreateAdminSecret(tm, token.Config{
		AdminName:         req.AdminName,
		Subject:           "admin",
		Roles:             nil,
		JWTSigningSecret:  req.JWTSigningSecret,
		RefreshExpiration: time.Duration(req.RefreshExpiration),
		AccessExpiration:  time.Duration(req.AccessExpiration),
	})
	if err != nil {
		return nil, err
	}

	// Return the token.
	return &pb.GenerateAdminTokenResponse{
		Token: s,
	}, nil
}

// RefreshAdminToken refreshes an admin access token given a valid refresh and access token.
func RefreshAdminToken(_ context.Context, req *pb.RefreshAdminTokenRequest) (*pb.RefreshAdminTokenResponse, error) {
	tm := NewTokenManager(HS256)
	refreshToken := req.RefreshToken
	accessToken := req.AccessToken

	var refreshClaims token.Claims
	_, err := tm.ParseWithClaims(refreshToken, req.JWTSigningSecret, &refreshClaims)
	if err != nil {
		return nil, fmt.Errorf("parsing admin refresh token: %w", err)
	}

	var accessClaims token.Claims
	_, err = tm.ParseWithClaims(accessToken, req.JWTSigningSecret, &accessClaims)
	if err == nil {
		return nil, errors.New("admin access token was valid")
	}

	switch err {
	case token.ErrExpired:
		logrus.WithField("audience", accessClaims.Audience).Debug("Refreshing admin token")
	default:
		return nil, fmt.Errorf("jwt validation: %w", err)
	}

	admin := strings.TrimSpace(accessClaims.Subject)
	if admin != "csm-admin" {
		return nil, fmt.Errorf("invalid admin: %q", admin)
	}

	// Use the refresh token with a smaller expiration timestamp to be
	// the new access token.
	refreshClaims.ExpiresAt = time.Now().Add(30 * time.Second).Unix()
	newAccess, err := tm.NewWithClaims(refreshClaims)
	if err != nil {
		return nil, err
	}

	newAccessStr, err := newAccess.SignedString(req.JWTSigningSecret)
	if err != nil {
		return nil, err
	}

	return &pb.RefreshAdminTokenResponse{
		AccessToken: newAccessStr,
	}, nil
}
