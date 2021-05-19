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

// Manager implements the Manager API via github.com/lestrrat-go/jwx
type Manager struct {
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

var _ token.Manager = &Manager{}
var _ token.Token = &Token{}

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

// NewWithClaims returns an unsigned Token configued with the supplied Claims
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

func tokenFromConfig(cfg token.Config) (jwt.Token, error) {
	t := jwt.New()
	err := t.Set(jwt.IssuerKey, "com.dell.karavi")
	if err != nil {
		return nil, err
	}

	err = t.Set(jwt.AudienceKey, "karavi")
	if err != nil {
		return nil, err
	}

	err = t.Set(jwt.SubjectKey, "karavi-tenant")
	if err != nil {
		return nil, err
	}

	err = t.Set(jwt.ExpirationKey, time.Now().Add(cfg.AccessExpiration).Unix())
	if err != nil {
		return nil, err
	}

	err = t.Set("roles", strings.Join(cfg.Roles, ","))
	if err != nil {
		return nil, err
	}

	err = t.Set("group", cfg.Tenant)
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
