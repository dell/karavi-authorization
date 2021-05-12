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
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

func TestListChangePowerFlex(t *testing.T) {
	// This file was generated using the following command:
	// kubectl get secrets,deployments,daemonsets -n vxflexos -o yaml
	listChangeMultiArray(t, "./testdata/kubectl_get_all_in_vxflexos.yaml", "vxflexos-config", 5)
}

func TestListChangeObservability(t *testing.T) {
	// This file was generated using the following command:
	// kubectl get secrets,deployments -n karavi -o yaml
	listChangeMultiArray(t, "./testdata/kubectl_get_all_in_karavi_observability.yaml", "vxflexos-config", 11)
}

func TestListChangePowerMaxNew(t *testing.T) {
	// This file was generated BEFORE injecting sidecar by using the following command:
	// kubectl get secrets,deployments,daemonsets -n powermax -o yaml

	//./testdata/kubectl_get_all_in_powermax.yaml
	listChangePowerMax(t, "./testdata/kubectl_get_all_in_powermax_new.yaml", 4)

}
func TestListChangePowerMaxUpdate(t *testing.T) {
	// This file was generated AFTER injecting sidecar by using the following command:
	// kubectl get secrets,deployments,daemonsets -n powermax -o yaml
	listChangePowerMax(t, "./testdata/kubectl_get_all_in_powermax_update.yaml", 7)

}
func TestGetStartingPortRanges(t *testing.T) {
	t.Run("no proxyPort flag", func(t *testing.T) {
		proxyPortFlags := []string{}
		got, err := getStartingPortRanges(proxyPortFlags)
		if err != nil {
			t.Fatal(err)
		}

		want := map[string]int{
			"powerflex": DefaultStartingPortRange,
			"powermax":  DefaultStartingPortRange + 200,
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("custom powerflex proxy port", func(t *testing.T) {
		proxyPortFlags := []string{"powerflex=10000"}
		got, err := getStartingPortRanges(proxyPortFlags)
		if err != nil {
			t.Fatal(err)
		}

		want := map[string]int{
			"powerflex": 10000,
			"powermax":  10200,
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("custom powermax proxy port", func(t *testing.T) {
		proxyPortFlags := []string{"powermax=10000"}
		got, err := getStartingPortRanges(proxyPortFlags)
		if err != nil {
			t.Fatal(err)
		}

		want := map[string]int{
			"powerflex": 9800,
			"powermax":  10000,
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})

	t.Run("custom powerflex and powermax proxy port", func(t *testing.T) {
		proxyPortFlags := []string{"powerflex=10000", "powermax=20000"}
		got, err := getStartingPortRanges(proxyPortFlags)
		if err != nil {
			t.Fatal(err)
		}

		want := map[string]int{
			"powerflex": 10000,
			"powermax":  20000,
		}

		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %+v, want %+v", got, want)
		}
	})
}

func listChangeMultiArray(t *testing.T, path, wantKey string, wantLen int) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var existing corev1.List
	err = yaml.Unmarshal(b, &existing)
	if err != nil {
		t.Fatal(err)
	}

	var sut ListChangeForMultiArray
	sut.ListChange = NewListChange(&existing)

	t.Run("injects the proxy pieces", func(t *testing.T) {
		portRanges, err := getStartingPortRanges(nil)
		if err != nil {
			t.Fatal(err)
		}

		got, err := injectUsingList(b,
			"http://image-addr",
			"http://proxy-addr",
			"./testdata/fake-certificate-file.pem", portRanges, false)
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
	})
}

func listChangePowerMax(t *testing.T, path string, wantLen int) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var existing corev1.List
	err = yaml.Unmarshal(b, &existing)
	if err != nil {
		t.Fatal(err)
	}

	var sut ListChangeForPowerMax
	sut.ListChange = NewListChange(&existing)
	wantKey := "powermax-creds"

	t.Run("injects the proxy pieces", func(t *testing.T) {
		portRanges, err := getStartingPortRanges(nil)
		if err != nil {
			t.Fatal(err)
		}
		got, err := injectUsingList(b,
			"http://image-addr",
			"http://proxy-addr",
			"./testdata/fake-certificate-file.pem", portRanges, false)
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
			Secret:     wantKey,
			Deployment: "powermax-controller",
		}
		sut.injectKaraviSecret(true)
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
	t.Run("inject a new deployment with localhost endpoints", func(t *testing.T) {
		modified, err := sut.Change(&existing, "http://image-addr", "http://proxy-addr", "", true)
		if err != nil {
			t.Fatal(err)
		}

		m, err := buildMapOfDeploymentsFromList(modified)
		if err != nil {
			t.Fatal(err)
		}

		deploy, ok := m[sut.InjectResources.Deployment]
		if !ok {
			t.Fatal("deployment not found")
		}

		// check that endpoint is modified
		for _, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == "driver" {
				commandEnvFlag := false
				for _, e := range c.Env {
					if e.Name == "X_CSI_POWERMAX_ENDPOINT" {
						u, err := url.Parse(e.Value)
						if err != nil {
							t.Fatal(err)
						}
						want := "localhost"
						if got := u.Hostname(); got != want {
							t.Errorf("got %q, want %q", got, want)
						}
						commandEnvFlag = true
					}

				}
				if !commandEnvFlag {
					t.Fatal("X_CSI_POWERMAX_ENDPOINT")
				}
				break
			}

		}

		for _, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == "driver" {
				commandEnvFlag := false
				for _, e := range c.Env {
					if e.Name == "CSM_CSI_POWERMAX_ENDPOINT" {
						u, err := url.Parse(e.Value)
						if err != nil {
							t.Fatal(err)
						}
						want := "localhost"
						if got := u.Hostname(); got == want {
							t.Errorf("got %q, want %q", got, want)
						}
						commandEnvFlag = true
					}

				}
				if !commandEnvFlag {
					t.Fatal("CSM_CSI_POWERMAX_ENDPOINT")
				}
				break
			}

		}

	})
}
