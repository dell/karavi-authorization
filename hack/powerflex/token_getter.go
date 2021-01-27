package powerflex

import (
	"sync"
	"time"

	"context"

	"github.com/dell/goscaleio"
	"github.com/sirupsen/logrus"
)

type TokenGetter struct {
	Config       Config
	currentToken string
	tokenMutex   *sync.Mutex // guards the cached currentToken
	newToken     chan struct{}
	sem          chan struct{}
}

type Config struct {
	PowerFlexClient      *goscaleio.Client
	TokenRefreshInterval time.Duration
	ConfigConnect        *goscaleio.ConfigConnect
	Logger               *logrus.Entry
}

func NewTokenGetter(ctx context.Context, c Config) *TokenGetter {
	loginHandler := &TokenGetter{
		Config:     c,
		tokenMutex: &sync.Mutex{},
		sem:        make(chan struct{}),
		newToken:   make(chan struct{}),
	}

	go func(ctx context.Context, lh *TokenGetter) {
		ticker := time.NewTicker(lh.Config.TokenRefreshInterval)
		defer ticker.Stop()
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

func (lh *TokenGetter) updateTokenFromPowerFlex() {
	_, err := lh.Config.PowerFlexClient.Authenticate(lh.Config.ConfigConnect)
	if err != nil {
		lh.Config.Logger.Errorf("PowerFlex Auth error: %s\n", err)
	} else {
		lh.currentToken = lh.Config.PowerFlexClient.GetToken()
		select {
		case lh.newToken <- struct{}{}:
		default:
		}
	}
}

func (lh *TokenGetter) isValidToken(token string) bool {
	//TODO make API call to PowerFlex to determine if token is valid
	return token != ""
}

func (lh *TokenGetter) GetToken(ctx context.Context) (string, error) {
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
