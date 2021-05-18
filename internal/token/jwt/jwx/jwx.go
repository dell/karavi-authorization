package jwx

import (
	"encoding/json"
	karaviJwt "karavi-authorization/internal/token/jwt"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
)

type TokenManager struct {
	SigningAlgorithm jwa.SignatureAlgorithm
}

type Token struct {
	token            jwt.Token
	SigningAlgorithm jwa.SignatureAlgorithm
}

var (
	errExpiredMsg = "exp not satisfied"
)

func NewTokenManager(alg jwa.SignatureAlgorithm) *TokenManager {
	jwt.Settings(jwt.WithFlattenAudience(true))
	return &TokenManager{SigningAlgorithm: alg}
}

func (m *TokenManager) NewPair(cfg karaviJwt.Config) (karaviJwt.Pair, error) {
	t := jwt.New()
	t.Set(jwt.IssuerKey, "com.dell.karavi")
	t.Set(jwt.AudienceKey, "karavi")
	t.Set(jwt.SubjectKey, "karavi-tenant")
	t.Set(jwt.ExpirationKey, time.Now().Add(cfg.AccessExpiration).Unix())
	t.Set("roles", strings.Join(cfg.Roles, ","))
	t.Set("group", cfg.Tenant)

	key, err := jwk.New([]byte(cfg.JWTSigningSecret))
	if err != nil {
		return karaviJwt.Pair{}, err
	}

	// Sign for an access token
	accessToken, err := jwt.Sign(t, m.SigningAlgorithm, key)
	if err != nil {
		return karaviJwt.Pair{}, err
	}

	// Sign for a refresh token
	t.Set(jwt.ExpirationKey, time.Now().Add(cfg.RefreshExpiration).Unix())
	refreshToken, err := jwt.Sign(t, jwa.HS256, key)
	if err != nil {
		return karaviJwt.Pair{}, err
	}

	return karaviJwt.Pair{
		Access:  string(accessToken),
		Refresh: string(refreshToken),
	}, nil
}

func (m *TokenManager) NewWithClaims(claims karaviJwt.Claims) (karaviJwt.Token, error) {
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
	}, nil
}

// TODO(aaron): Investigate why passing in a Claims doesn't work
func (m *TokenManager) ParseWithClaims(token string, secret string, claims *karaviJwt.Claims) (karaviJwt.Token, error) {
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
			return nil, &karaviJwt.ErrExpired{Err: err}
		}
		return nil, err
	}

	return &Token{
		token:            t,
		SigningAlgorithm: m.SigningAlgorithm,
	}, nil
}

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

func (t *Token) Claims() (karaviJwt.Claims, error) {
	data, err := json.Marshal(t.token)
	if err != nil {
		return karaviJwt.Claims{}, err
	}

	var c karaviJwt.Claims
	err = json.Unmarshal(data, &c)
	if err != nil {
		return karaviJwt.Claims{}, err
	}

	return c, nil
}
