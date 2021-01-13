package powerflex

import (
	"time"

	"context"
)

type LoginHandler struct{}

type Config struct {
	TokenRefreshInterval time.Duration
}

func NewLoginHandler(addr, user, password string, c Config) *LoginHandler {
	return nil
}

func (lh *LoginHandler) GetToken(ctx context.Context) (string, error) {
	return "", nil
}
