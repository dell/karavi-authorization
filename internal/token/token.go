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

package token

import (
	"errors"
	"time"
)

var (
	// ErrExpired is the error for an expired token
	ErrExpired = errors.New("token has expired")
)

// Claims represents the standard JWT claims in addition
// to Karavi-Authorization specific claims.
type Claims struct {
	Audience  string `json:"aud,omitempty"`
	ExpiresAt int64  `json:"exp,omitempty"`
	Issuer    string `json:"iss,omitempty"`
	Subject   string `json:"sub,omitempty"`
	Roles     string `json:"roles"`
	Group     string `json:"group"`
}

// Pair represents a pair of tokens, refresh and access.
type Pair struct {
	Refresh string
	Access  string
}

// Config contains configurable options when creating tokens.
type Config struct {
	Tenant            string
	AdminName         string
	Subject           string
	Roles             []string
	JWTSigningSecret  string
	RefreshExpiration time.Duration
	AccessExpiration  time.Duration
}

// AdminToken contains the access-refresh pair token string
type AdminToken struct {
	Access  string `yaml:"access"`
	Refresh string `yaml:"refresh"`
}

// Manager defines the interface for a JWT API
type Manager interface {
	// NewPair returns an access/refresh pair from a Config
	NewPair(Config) (Pair, error)
	// NewWithClaims returns a Token built from the claims
	NewWithClaims(claims Claims) (Token, error)
	// ParseWithClaims unmarshals a token string into claims and returns the Token
	ParseWithClaims(token string, secret string, claims *Claims) (Token, error)
}

// Token defines the interface for token operations
type Token interface {
	// Claims returns the Claims of the Token
	Claims() (Claims, error)
	// SignedString returns a token string signed with the secret
	SignedString(secret string) (string, error)
}
