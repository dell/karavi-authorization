package token

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
)

// JwxTokenManager implements the TokenManager API via github.com/lestrrat-go/jwx
type JwxTokenManager struct {
	SigningAlgorithm jwa.SignatureAlgorithm
}

// JwxToken implemetns the Token API via github.com/lestrrat-go/jwx
type JwxToken struct {
	token            jwt.Token
	SigningAlgorithm jwa.SignatureAlgorithm
}

var (
	errExpiredMsg = "exp not satisfied"
)

var _ TokenManager = &JwxTokenManager{}
var _ Token = &JwxToken{}

// NewJwxTokenManager returns a JwxTokenManager configured with the supplied signature algorithm
func NewJwxTokenManager(alg jwa.SignatureAlgorithm) *JwxTokenManager {
	jwt.Settings(jwt.WithFlattenAudience(true))
	return &JwxTokenManager{SigningAlgorithm: alg}
}

// NewPair returns a new access/refresh Pair
func (m *JwxTokenManager) NewPair(cfg Config) (Pair, error) {
	t := jwt.New()
	t.Set(jwt.IssuerKey, "com.dell.karavi")
	t.Set(jwt.AudienceKey, "karavi")
	t.Set(jwt.SubjectKey, "karavi-tenant")
	t.Set(jwt.ExpirationKey, time.Now().Add(cfg.AccessExpiration).Unix())
	t.Set("roles", strings.Join(cfg.Roles, ","))
	t.Set("group", cfg.Tenant)

	key, err := jwk.New([]byte(cfg.JWTSigningSecret))
	if err != nil {
		return Pair{}, err
	}

	// Sign for an access token
	accessToken, err := jwt.Sign(t, m.SigningAlgorithm, key)
	if err != nil {
		return Pair{}, err
	}

	// Sign for a refresh token
	t.Set(jwt.ExpirationKey, time.Now().Add(cfg.RefreshExpiration).Unix())
	refreshToken, err := jwt.Sign(t, jwa.HS256, key)
	if err != nil {
		return Pair{}, err
	}

	return Pair{
		Access:  string(accessToken),
		Refresh: string(refreshToken),
	}, nil
}

// NewWithClaims returns an unsigned Token configued with the supplied Claims
func (m *JwxTokenManager) NewWithClaims(claims Claims) (Token, error) {
	t := jwt.New()
	t.Set(jwt.IssuerKey, claims.Issuer)
	t.Set(jwt.AudienceKey, claims.Audience)
	t.Set(jwt.SubjectKey, claims.Subject)
	t.Set(jwt.ExpirationKey, claims.ExpiresAt)
	t.Set("roles", claims.Roles)
	t.Set("group", claims.Group)

	return &JwxToken{
		token:            t,
		SigningAlgorithm: m.SigningAlgorithm,
	}, nil
}

// ParseWithClaims verifies and validates a token and unmarshals it into the supplied Claims
func (m *JwxTokenManager) ParseWithClaims(token string, secret string, claims *Claims) (Token, error) {
	// verify the token with the secret, but don't validate it yet so we can use the token
	verifiedToken, err := jwt.ParseString(token, jwt.WithVerify(m.SigningAlgorithm, []byte(secret)))
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(verifiedToken)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &claims)
	if err != nil {
		return nil, err
	}

	// now validate the verified token
	t, err := jwt.ParseString(token, jwt.WithValidate(true))
	if err != nil {
		if strings.Contains(err.Error(), errExpiredMsg) {
			return nil, &ErrExpired{Err: err}
		}
		return nil, err
	}

	return &JwxToken{
		token:            t,
		SigningAlgorithm: m.SigningAlgorithm,
	}, nil
}

// SignedString returns a signed, serialized token with the supplied secret
func (t *JwxToken) SignedString(secret string) (string, error) {
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
func (t *JwxToken) Claims() (Claims, error) {
	data, err := json.Marshal(t.token)
	if err != nil {
		return Claims{}, err
	}

	var c Claims
	err = json.Unmarshal(data, &c)
	if err != nil {
		return Claims{}, err
	}

	return c, nil
}
