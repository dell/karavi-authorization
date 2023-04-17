// Copyright © 2021-2023 Dell Inc., or its subsidiaries. All Rights Reserved.
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

package token

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"
)

// Errors.
var (
	ErrBlankSecretNotAllowed = errors.New("blank JWT signing secret not allowed")
)

// CreateAsK8sSecret returns a pair of created tokens in the form
// of a Kubernetes Secret.
func CreateAsK8sSecret(tm Manager, cfg Config) (string, error) {
	tp, err := Create(tm, cfg)
	if err != nil {
		return "", err
	}

	accessTokenEnc := base64.StdEncoding.EncodeToString([]byte(tp.Access))
	refreshTokenEnc := base64.StdEncoding.EncodeToString([]byte(tp.Refresh))

	ret := fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: proxy-authz-tokens
type: Opaque
data:
  access: %s
  refresh: %s
`, accessTokenEnc, refreshTokenEnc)

	return ret, nil
}

// Create creates a pair of tokens based on the provided Config.
func Create(tm Manager, cfg Config) (Pair, error) {
	if len(strings.TrimSpace(cfg.JWTSigningSecret)) == 0 {
		return Pair{}, ErrBlankSecretNotAllowed
	}

	return tm.NewPair(cfg)
}

// CreateAdminSecret returns a pair of created tokens for
// CSM Authorization admin.
func CreateAdminSecret(tm Manager, cfg Config) ([]byte, error) {
	tp, err := Create(tm, cfg)
	if err != nil {
		return nil, err
	}

	admtoken := AdminToken{
		Access:  tp.Access,
		Refresh: tp.Refresh,
	}
	ret, err := yaml.Marshal(&admtoken)
	if err != nil {
		return nil, err
	}
	return ret, nil
}
