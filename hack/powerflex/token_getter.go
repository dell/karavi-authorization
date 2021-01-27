package powerflex

import (
	"sync"
	"time"

	"context"

	"github.com/dell/goscaleio"
	"github.com/sirupsen/logrus"
)

type PowerFlexTokenGetter struct {
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

func NewTokenGetter(ctx context.Context, c Config) *PowerFlexTokenGetter {
	loginHandler := &PowerFlexTokenGetter{
		Config:     c,
		tokenMutex: &sync.Mutex{},
		sem:        make(chan struct{}),
		newToken:   make(chan struct{}),
	}

	go func(ctx context.Context, tg *PowerFlexTokenGetter) {
		ticker := time.NewTicker(tg.Config.TokenRefreshInterval)
		defer ticker.Stop()
		for {
			tg.tokenMutex.Lock()
			tg.currentToken = ""
			tg.updateTokenFromPowerFlex()
			tg.tokenMutex.Unlock()
			select {
			case <-ticker.C:
			case <-tg.sem:
			case <-ctx.Done():
				return
			}
		}
	}(ctx, loginHandler)

	return loginHandler
}

func (tg *PowerFlexTokenGetter) updateTokenFromPowerFlex() {
	_, err := tg.Config.PowerFlexClient.Authenticate(tg.Config.ConfigConnect)
	if err != nil {
		tg.Config.Logger.Errorf("PowerFlex Auth error: %s\n", err)
	} else {
		tg.currentToken = tg.Config.PowerFlexClient.GetToken()
		select {
		case tg.newToken <- struct{}{}:
		default:
		}
	}
}

func (tg *PowerFlexTokenGetter) isValidToken(token string) bool {
	//TODO make API call to PowerFlex to determine if token is valid
	return token != ""
}

func (tg *PowerFlexTokenGetter) GetToken(ctx context.Context) (string, error) {
	tg.tokenMutex.Lock()
	if tg.isValidToken(tg.currentToken) {
		defer tg.tokenMutex.Unlock()
		return tg.currentToken, nil
	} else {
		tg.tokenMutex.Unlock()
		tg.sem <- struct{}{}
		select {
		case <-tg.newToken:
			return tg.currentToken, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
}
