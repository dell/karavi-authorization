package powerflex

import (
	"time"

	"context"

	"github.com/dell/goscaleio"
)

type LoginHandler struct {
	PowerFlexClient *goscaleio.Client
}

type Config struct {
	PowerFlexClient      *goscaleio.Client
	TokenRefreshInterval time.Duration
}

func NewLoginHandler(c Config) *LoginHandler {
	return nil
}

func (lh *LoginHandler) GetToken(ctx context.Context) (string, error) {
	return "", nil
}
