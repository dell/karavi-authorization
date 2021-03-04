package token

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
)

type Claims struct {
	jwt.StandardClaims
	Role  string `json:"role"`
	Group string `json:"group"`
}

// TokenPair represents a pair of tokens, refresh and access.
type TokenPair struct {
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
func Create(cfg Config) (TokenPair, error) {
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
		return TokenPair{}, err
	}
	// Sign for a refresh token
	claims.ExpiresAt = time.Now().Add(cfg.RefreshExpiration).Unix()
	token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	refreshToken, err := token.SignedString([]byte(cfg.JWTSigningSecret))
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		Access:  accessToken,
		Refresh: refreshToken,
	}, nil
}
