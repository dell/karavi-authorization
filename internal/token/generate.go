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

package token

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
)

// Claims represents the standard JWT claims in addition
// to Karavi-Authorization specific claims.
type Claims struct {
	jwt.StandardClaims
	Role  string `json:"role"`
	Group string `json:"group"`
}

// Pair represents a pair of tokens, refresh and access.
type Pair struct {
	Refresh string
	Access  string
}

// Config contains configurable options when creating tokens.
type Config struct {
	Tenant            string
	Roles             []string
	JWTSigningSecret  string
	RefreshExpiration time.Duration
	AccessExpiration  time.Duration
}

// CreateAsK8sSecret returns a pair of created tokens in the form
// of a Kubernetes Secret.
func CreateAsK8sSecret(cfg Config) (string, error) {
	tp, err := Create(cfg)
	if err != nil {
		return "", err
	}

	accessTokenEnc := base64.StdEncoding.EncodeToString([]byte(tp.Access))
	refreshTokenEnc := base64.StdEncoding.EncodeToString([]byte(tp.Refresh))

	ret := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: proxy-authz-tokens
type: Opaque
data:
  access: %s
  refresh: %s
`, accessTokenEnc, refreshTokenEnc)

	return ret, nil
}

// Create creates a pair of tokens based on the provided Config.
func Create(cfg Config) (Pair, error) {
	// Create the claims
	claims := Claims{
		StandardClaims: jwt.StandardClaims{
			Issuer:    "com.dell.karavi",
			ExpiresAt: time.Now().Add(cfg.AccessExpiration).Unix(),
			Audience:  "karavi",
			Subject:   "karavi-tenant",
		},
		Role:  strings.Join(cfg.Roles, ","),
		Group: cfg.Tenant,
	}
	// Sign for an access token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString([]byte(cfg.JWTSigningSecret))
	if err != nil {
		return Pair{}, err
	}
	// Sign for a refresh token
	claims.ExpiresAt = time.Now().Add(cfg.RefreshExpiration).Unix()
	token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	refreshToken, err := token.SignedString([]byte(cfg.JWTSigningSecret))
	if err != nil {
		return Pair{}, err
	}

	return Pair{
		Access:  accessToken,
		Refresh: refreshToken,
	}, nil
}
