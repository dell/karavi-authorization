package jwt

import (
	"time"

	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
)

type JWXTokenManager struct{}

func (m *JWXTokenManager) NewPair(cfg Config) (Pair, error) {
	t := jwt.New()
	t.Set(jwt.IssuerKey, "com.dell.karavi")
	t.Set(jwt.AudienceKey, "karavi")
	t.Set(jwt.SubjectKey, "karavi-tenant")
	t.Set(jwt.ExpirationKey, time.Now().Add(cfg.AccessExpiration).Unix())

	key, err := jwk.New([]byte(cfg.JWTSigningSecret))
	if err != nil {
		return Pair{}, err
	}

	// Sign for an access token
	accessToken, err := jwt.Sign(t, jwa.HS256, key)
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
