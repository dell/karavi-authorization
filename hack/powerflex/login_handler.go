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
	sem          chan struct{}
	tokenMutex   *sync.RWMutex
}

type Config struct {
	PowerFlexClient      *goscaleio.Client
	TokenRefreshInterval time.Duration
	ConfigConnect        *goscaleio.ConfigConnect
}

func NewLoginHandler(ctx context.Context, c Config) *LoginHandler {
	loginHandler := &LoginHandler{
		Config:     c,
		sem:        make(chan struct{}),
		tokenMutex: &sync.RWMutex{},
	}

	go func(ctx context.Context, lh *LoginHandler) {
		numTimesUpdated := 0
		ticker := time.NewTicker(lh.Config.TokenRefreshInterval)
		for {
			lh.updateTokenFromPowerFlex()
			numTimesUpdated++
			fmt.Printf("Num times updated: %v\n", numTimesUpdated)
			select {
			case <-ticker.C:
				fmt.Println("TICK")
			case <-lh.sem:
				fmt.Println("SEM")
			case <-ctx.Done():
				fmt.Println("loginhandler context done!")
				return
			}
		}
	}(ctx, loginHandler)

	return loginHandler
}

func (lh *LoginHandler) updateTokenFromPowerFlex() {
	fmt.Println("Updating token...")
	lh.tokenMutex.Lock()
	_, err := lh.Config.PowerFlexClient.Authenticate(lh.Config.ConfigConnect)
	if err != nil {
		fmt.Printf("PowerFlex Auth error: %s\n", err)
	} else {
		lh.currentToken = lh.Config.PowerFlexClient.GetToken()
	}
	lh.tokenMutex.Unlock()
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
	for !lh.isValidToken(lh.currentToken) {
		lh.tokenMutex.Unlock()
		lh.sem <- struct{}{}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}
		lh.tokenMutex.Lock()
	}
	lh.tokenMutex.Unlock()
	return lh.currentToken, nil
}
