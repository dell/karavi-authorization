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
	"encoding/json"
	"errors"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "proxy-authz-tokens",
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"access":  []byte(tp.Access),
			"refresh": []byte(tp.Refresh),
		},
	}

	jsonBytes, err := json.Marshal(&secret)
	if err != nil {
		return "", err
	}

	yamlBytes, err := yaml.JSONToYAML(jsonBytes)
	if err != nil {
		return "", err
	}

	return string(yamlBytes), nil
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
