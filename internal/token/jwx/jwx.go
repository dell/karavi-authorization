package jwx

import (
	"encoding/json"
	"karavi-authorization/internal/token"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
)

// TokenManager implements the TokenManager API via github.com/lestrrat-go/jwx
type TokenManager struct {
	SigningAlgorithm jwa.SignatureAlgorithm
}

// Token implemetns the Token API via github.com/lestrrat-go/jwx
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
)

var _ token.TokenManager = &TokenManager{}
var _ token.Token = &Token{}

// NewJwxTokenManager returns a TokenManager configured with the supplied signature algorithm
func NewTokenManager(alg SignatureAlgorithm) token.TokenManager {
	jwt.Settings(jwt.WithFlattenAudience(true))
	return &TokenManager{SigningAlgorithm: jwa.SignatureAlgorithm(alg)}
}

// NewPair returns a new access/refresh Pair
func (m *TokenManager) NewPair(cfg token.Config) (token.Pair, error) {
	t := jwt.New()
	t.Set(jwt.IssuerKey, "com.dell.karavi")
	t.Set(jwt.AudienceKey, "karavi")
	t.Set(jwt.SubjectKey, "karavi-tenant")
	t.Set(jwt.ExpirationKey, time.Now().Add(cfg.AccessExpiration).Unix())
	t.Set("roles", strings.Join(cfg.Roles, ","))
	t.Set("group", cfg.Tenant)

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
	t.Set(jwt.ExpirationKey, time.Now().Add(cfg.RefreshExpiration).Unix())
	refreshToken, err := jwt.Sign(t, jwa.HS256, key)
	if err != nil {
		return token.Pair{}, err
	}

	return token.Pair{
		Access:  string(accessToken),
		Refresh: string(refreshToken),
	}, nil
}

// NewWithClaims returns an unsigned Token configued with the supplied Claims
func (m *TokenManager) NewWithClaims(claims token.Claims) token.Token {
	t := jwt.New()
	t.Set(jwt.IssuerKey, claims.Issuer)
	t.Set(jwt.AudienceKey, claims.Audience)
	t.Set(jwt.SubjectKey, claims.Subject)
	t.Set(jwt.ExpirationKey, claims.ExpiresAt)
	t.Set("roles", claims.Roles)
	t.Set("group", claims.Group)

	return &Token{
		token:            t,
		SigningAlgorithm: m.SigningAlgorithm,
	}
}

// ParseWithClaims verifies and validates a token and unmarshals it into the supplied Claims
func (m *TokenManager) ParseWithClaims(tokenStr string, secret string, claims *token.Claims) (token.Token, error) {
	// verify the token with the secret, but don't validate it yet so we can use the token
	verifiedToken, err := jwt.ParseString(tokenStr, jwt.WithVerify(m.SigningAlgorithm, []byte(secret)))
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
	t, err := jwt.ParseString(tokenStr, jwt.WithValidate(true))
	if err != nil {
		if strings.Contains(err.Error(), errExpiredMsg) {
			return nil, &token.ErrExpired{Err: err}
		}
		return nil, err
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
