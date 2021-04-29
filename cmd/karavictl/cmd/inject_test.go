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

package cmd

import (
	"io/ioutil"
	"net/url"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestListChangePowerFlex(t *testing.T) {
	// This file was generated using the following command:
	// kubectl get secrets,deployments,daemonsets -n vxflexos -o yaml
	path := "./testdata/kubectl_get_all_in_vxflexos.yaml"
	listChange(t, path, "vxflexos-config", 5)
}

func TestListChangePowerMax(t *testing.T) {
	// TODO
	// This file was generated using the following command:
	// kubectl get secrets,deployments,daemonsets -n powermax -o yaml
	path := "./testdata/kubectl_get_all_in_powermax.yaml"
	listChange(t, path, "powermax-creds", 4)
}

func TestListChangeObservability(t *testing.T) {
	// This file was generated using the following command:
	// kubectl get secrets,deployments -n karavi -o yaml
	path := "./testdata/kubectl_get_all_in_karavi_observability.yaml"
	// TODO: ask Trevor what this is about
	listChange(t, path, "vxflexos-config", 11)
}

func listChange(t *testing.T, path, wantKey string, wantLen int) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var existing corev1.List
	err = yaml.Unmarshal(b, &existing)
	if err != nil {
		t.Fatal(err)
	}

	sut := NewListChange(&existing)

	t.Run("injects the proxy pieces", func(t *testing.T) {
		got, err := injectUsingList(b,
			"http://image-addr",
			"http://proxy-addr",
			"./testdata/fake-certificate-file.pem", false)
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			t.Error("expected non-nil return value, but got nil")
		}
	})

	t.Run("build a map of secrets", func(t *testing.T) {
		got, err := buildMapOfSecretsFromList(sut.Existing)
		if err != nil {
			t.Fatal(err)
		}

		if l := len(got); l != wantLen {
			t.Errorf("buildMapOfSecretsFromList: got len %d, want %d", l, wantLen)
		}
		// The Secret should exist with a non-nil value.
		if v, ok := got[wantKey]; !ok || v == nil {
			t.Errorf("buildMapOfSecretsFromList: expected key %q to exist, but got [%v,%v]",
				wantKey, v, ok)
		}
	})
	t.Run("inject a new secret with localhost endpoints", func(t *testing.T) {
		sut.InjectResources = &Resources{
			Secret: wantKey,
		}
		sut.injectKaraviSecret()
		if sut.Err != nil {
			t.Fatal(sut.Err)
		}

		// Extract only the secrets from this list.
		secrets, err := buildMapOfSecretsFromList(sut.Modified)
		if err != nil {
			t.Fatal(err)
		}
		secret, ok := secrets["karavi-authorization-config"]
		if !ok {
			t.Fatal("expected new secret to exist, but it didn't")
		}
		secretData, err := getSecretData(secret)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range secretData {
			u, err := url.Parse(v.Endpoint)
			if err != nil {
				t.Fatal(err)
			}
			want := "localhost"
			if got := u.Hostname(); got != want {
				t.Errorf("got %q, want %q", got, want)
			}
		}
		// The new secret should be called karavi-auth-config
		// It should replace endpoint values with localhost
		// Each localhost should have a unique port number
		// The original secret should be left intact.
	})
}
