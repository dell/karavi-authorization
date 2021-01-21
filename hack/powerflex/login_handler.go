package powerflex

import (
	"errors"
	"fmt"
	"time"

	"context"

	"github.com/dell/goscaleio"
)

type LoginHandler struct {
	Config       Config
	currentToken string
}

type Config struct {
	PowerFlexClient      *goscaleio.Client
	TokenRefreshInterval time.Duration
	ConfigConnect        *goscaleio.ConfigConnect
}

func NewLoginHandler(c Config) *LoginHandler {
	loginHandler := &LoginHandler{
		Config: c,
	}

	go func(lh *LoginHandler) {
		ticker := time.NewTicker(lh.Config.TokenRefreshInterval)
		for {
			<-ticker.C
			_, err := lh.Config.PowerFlexClient.Authenticate(lh.Config.ConfigConnect)
			if err != nil {
				fmt.Printf("PowerFlex Auth error: %s\n", err)
			} else {
				lh.currentToken = lh.Config.PowerFlexClient.GetToken()
				fmt.Println("NEW TOKEN: " + lh.currentToken)
			}
		}
	}(loginHandler)

	return loginHandler
}

func (lh *LoginHandler) validateCurrentToken() bool {
	return true
}

func (lh *LoginHandler) GetToken(ctx context.Context) (string, error) {
	if lh.currentToken == "" {
		return "", errors.New("LoginHandler does not have token available.")
	}
	return lh.currentToken, nil
}
