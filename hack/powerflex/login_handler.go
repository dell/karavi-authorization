package powerflex

import (
	"fmt"
	"sync"
	"time"

	"context"

	"github.com/dell/goscaleio"
)

type LoginHandler struct {
	Config       Config
	currentToken string
	tokenMutex   *sync.Mutex
	newToken     chan struct{}
	sem          chan struct{}
}

type Config struct {
	PowerFlexClient      *goscaleio.Client
	TokenRefreshInterval time.Duration
	ConfigConnect        *goscaleio.ConfigConnect
}

func NewLoginHandler(ctx context.Context, c Config) *LoginHandler {
	loginHandler := &LoginHandler{
		Config:     c,
		tokenMutex: &sync.Mutex{},
		sem:        make(chan struct{}),
		newToken:   make(chan struct{}),
	}

	go func(ctx context.Context, lh *LoginHandler) {
		ticker := time.NewTicker(lh.Config.TokenRefreshInterval)
		for {
			lh.tokenMutex.Lock()
			lh.currentToken = ""
			lh.updateTokenFromPowerFlex()
			lh.tokenMutex.Unlock()
			select {
			case <-ticker.C:
			case <-lh.sem:
			case <-ctx.Done():
				return
			}
		}
	}(ctx, loginHandler)

	return loginHandler
}

func (lh *LoginHandler) updateTokenFromPowerFlex() {
	_, err := lh.Config.PowerFlexClient.Authenticate(lh.Config.ConfigConnect)
	if err != nil {
		fmt.Printf("PowerFlex Auth error: %s\n", err)
	} else {
		lh.currentToken = lh.Config.PowerFlexClient.GetToken()
		select {
		case lh.newToken <- struct{}{}:
		default:
		}
	}
}

func (lh *LoginHandler) isValidToken(token string) bool {
	if token == "" {
		return false
	} else {
		//TODO make API call to PowerFlex to determine if token is valid
		return true
	}
}

func (lh *LoginHandler) GetToken(ctx context.Context) (string, error) {
	lh.tokenMutex.Lock()
	if lh.isValidToken(lh.currentToken) {
		defer lh.tokenMutex.Unlock()
		return lh.currentToken, nil
	} else {
		lh.tokenMutex.Unlock()
		lh.sem <- struct{}{}
		select {
		case <-lh.newToken:
			return lh.currentToken, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}
