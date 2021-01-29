package powerflex

import (
	"fmt"
	"time"

	"context"

	"github.com/dell/goscaleio"
	"github.com/sirupsen/logrus"
)

type PowerFlexTokenGetter struct {
	Config       Config
	currentToken string
	sem          chan struct{}
}

type Config struct {
	PowerFlexClient      *goscaleio.Client
	TokenRefreshInterval time.Duration
	ConfigConnect        *goscaleio.ConfigConnect
	Logger               *logrus.Entry
}

func NewTokenGetter(c Config) *PowerFlexTokenGetter {
	return &PowerFlexTokenGetter{
		Config: c,
		sem:    make(chan struct{}, 1),
	}
}

func (tg *PowerFlexTokenGetter) Start(ctx context.Context) error {
	// Update the token one time on startup, then update on timer interval after that
	tg.currentToken = ""
	tg.updateTokenFromPowerFlex()

	timer := time.NewTimer(tg.Config.TokenRefreshInterval)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			tg.updateTokenFromPowerFlex()
			timer.Reset(tg.Config.TokenRefreshInterval)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (tg *PowerFlexTokenGetter) GetToken(ctx context.Context) (string, error) {
	select {
	case tg.sem <- struct{}{}:
	case <-ctx.Done():
		return "", ctx.Err()
	}
	defer func() { <-tg.sem }()
	return tg.currentToken, nil
}

func (tg *PowerFlexTokenGetter) updateTokenFromPowerFlex() {
	tg.sem <- struct{}{}
	fmt.Println("LOCKING")
	defer func() {
		<-tg.sem
		fmt.Println("UNLOCKING")
	}()

	if _, err := tg.Config.PowerFlexClient.Authenticate(tg.Config.ConfigConnect); err == nil {
		tg.Config.Logger.Errorf("PowerFlex Auth error: %s\n", err)
	}
	tg.currentToken = tg.Config.PowerFlexClient.GetToken()
	fmt.Printf("New token assigned: %s\n", tg.currentToken)
}
