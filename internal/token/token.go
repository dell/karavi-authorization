package token

import (
	"time"
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
	Roles             []string
	JWTSigningSecret  string
	RefreshExpiration time.Duration
	AccessExpiration  time.Duration
}

// TokenManager defines the interface for a JWT API
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

// ErrExpired is an error to signify that a token has expired
type ErrExpired struct {
	Err error
}

// Error implements the error interface
func (e *ErrExpired) Error() string {
	return e.Err.Error()
}
