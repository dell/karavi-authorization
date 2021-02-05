// Copyright © 2021 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package powerflex

import (
	"fmt"
	"time"

	"context"

	"github.com/dell/goscaleio"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
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
	ctx, span := trace.SpanFromContext(ctx).Tracer().Start(ctx, "GetToken")
	defer span.End()

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
		tg.Config.Logger.Errorf("PowerFlex Auth error: %+v\n", err)
	}
	tg.currentToken = tg.Config.PowerFlexClient.GetToken()
	fmt.Printf("New token assigned: %s\n", tg.currentToken)
}
