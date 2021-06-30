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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

// ListChangeForMultiArray holds a k8s list and a modified version of said list
type ListChangeForPowerScale struct {
	*ListChange
	StartingPortRange int
}

func (lc *ListChangeForPowerScale) Change(existing *corev1.List, imageAddr, proxyHost, rootCertificate string, insecure bool) (*corev1.List, error) {
	lc.ListChange = NewListChange(existing)
	// Determine what we are injecting the sidecar into (e.g. powerflex csi driver, observability, etc)
	lc.setInjectedResources()
	// Inject the rootCA certificate as a Secret
	lc.injectRootCertificate(rootCertificate)
	// Inject our own secret based on the original config.
	lc.injectKaraviSecret(insecure)
	// Inject the sidecar proxy into the Deployment and update
	// the config volume to point to our own secret.
	lc.injectIntoDeployment(imageAddr, proxyHost, insecure)
	// Inject into the Daemonset.
	lc.injectIntoDaemonset(imageAddr, proxyHost, insecure)

	return lc.ListChange.Modified, lc.ListChange.Err
}

func (lc *ListChangeForPowerScale) injectKaraviSecret(insecure bool) {
	if lc.Err != nil {
		return
	}

	if lc.InjectResources.Secret == "" {
		return
	}

	// Extract all of the Secret resources.
	secrets, err := buildMapOfSecretsFromList(lc.Existing)
	if err != nil {
		lc.Err = fmt.Errorf("building secret map: %w", err)
		return
	}

	// Pick out the config.
	configSecret, ok := secrets[lc.InjectResources.Secret]
	if !ok {
		lc.Err = errors.New("config secret not found")
		return
	}

	// Get the config data.
	configSecData, err := lc.ExtractSecretData(configSecret)
	if err != nil {
		lc.Err = fmt.Errorf("getting secret data: %w", err)
		return
	}

	// Copy the config data and convert endpoints to localhost:<port>
	configSecData = convertEndpoints(configSecData, lc.StartingPortRange)
	configSecData = scrubLoginCredentials(configSecData)
	configSecDataJSON, err := json.Marshal(&configSecData)
	if err != nil {
		lc.Err = err
		return
	}

	// Create the Karavi config Secret, containing this new data.
	newSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "karavi-authorization-config",
			Namespace: configSecret.Namespace,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: configSecret.APIVersion,
			Kind:       "Secret",
		},
		Type: "Opaque",
		Data: map[string][]byte{
			"config": []byte(configSecDataJSON),
		},
	}

	// Append it to the list of items.
	enc, err := json.Marshal(&newSecret)
	if err != nil {
		lc.Err = err
		return
	}
	raw := runtime.RawExtension{
		Raw: enc,
	}
	lc.Modified.Items = append(lc.Modified.Items, raw)
}

func (lc *ListChangeForPowerScale) injectIntoDeployment(imageAddr, proxyHost string, insecure bool) {
	if lc.Err != nil {
		return
	}

	if lc.ListChange.InjectResources.Deployment == "" {
		return
	}

	m, err := buildMapOfDeploymentsFromList(lc.ListChange.Existing)
	if err != nil {
		lc.Err = err
		return
	}

	deploy, ok := m[lc.ListChange.InjectResources.Deployment]
	if !ok {
		lc.Err = errors.New("deployment not found")
		return
	}

	volumes := deploy.Spec.Template.Spec.Volumes
	for i, v := range volumes {
		if v.Name != lc.InjectResources.Secret {
			continue
		}
		volumes[i].Secret.SecretName = "karavi-authorization-config"
	}

	rootCertificateMounted := false
	for _, v := range volumes {
		if v.Name == "proxy-server-root-certificate" {
			rootCertificateMounted = true
			break
		}
	}

	if !rootCertificateMounted {
		rootCertificateVolume := corev1.Volume{}
		rootCertificateVolume.Name = "proxy-server-root-certificate"
		rootCertificateVolume.Secret = &corev1.SecretVolumeSource{
			SecretName: "proxy-server-root-certificate",
		}
		deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, rootCertificateVolume)
	}

	containers := deploy.Spec.Template.Spec.Containers

	// Remove any existing proxy containers...
	for i, c := range containers {
		if c.Name == "karavi-authorization-proxy" {
			containers = append(containers[:i], containers[i+1:]...)
		}
	}

	/*for i, c := range containers {
		if c.Name == "driver" {
			commandEnvFlag := false
			for j, e := range c.Env {
				if e.Name == "CSI_ENDPOINT" {
					containers[i].Env[j].Value = lc.Endpoint
					commandEnvFlag = true
				}
			}
			if !commandEnvFlag {
				lc.Err = errors.New("CSI_ENDPOINT not found")
				return
			}
			break
		}
	}*/

	// Add a new proxy container...
	proxyContainer := buildProxyContainer(deploy.Namespace, lc.InjectResources.Secret, imageAddr, proxyHost, insecure)
	containers = append(containers, *proxyContainer)
	deploy.Spec.Template.Spec.Containers = containers

	deploy.Annotations["com.dell.karavi-authorization-proxy"] = "true"

	// Add the extra-create-metadata flag to provisioner if it does not exist
	if deploy.Name == lc.InjectResources.Deployment {
		provisionerMetaDataFlag := false
		for i, c := range deploy.Spec.Template.Spec.Containers {
			if c.Name == "provisioner" {
				for _, a := range c.Args {
					if a == "--extra-create-metadata" {
						provisionerMetaDataFlag = true
						break
					}
				}
				if !provisionerMetaDataFlag {
					deploy.Spec.Template.Spec.Containers[i].Args = append(deploy.Spec.Template.Spec.Containers[i].Args, "--extra-create-metadata")
				}
			}
		}
	}

	// Append it to the list of items.
	enc, err := json.Marshal(&deploy)
	if err != nil {
		lc.Err = err
		return
	}
	raw := runtime.RawExtension{
		Raw: enc,
	}
	lc.Modified.Items = append(lc.Modified.Items, raw)
}

func (lc *ListChangeForPowerScale) injectIntoDaemonset(imageAddr, proxyHost string, insecure bool) {
	if lc.Err != nil {
		return
	}

	if lc.InjectResources.DaemonSet == "" {
		return
	}

	m, err := buildMapOfDaemonsetsFromList(lc.Existing)
	if err != nil {
		lc.Err = err
		return
	}

	ds, ok := m[lc.InjectResources.DaemonSet]
	if !ok {
		lc.Err = errors.New("daemonset not found")
		return
	}

	volumes := ds.Spec.Template.Spec.Volumes
	for i, v := range volumes {
		if v.Name != lc.InjectResources.Secret {
			continue
		}
		volumes[i].Secret.SecretName = "karavi-authorization-config"
	}

	rootCertificateMounted := false
	for _, v := range volumes {
		if v.Name == "proxy-server-root-certificate" {
			rootCertificateMounted = true
			break
		}
	}

	if !rootCertificateMounted {
		rootCertificateVolume := corev1.Volume{}
		rootCertificateVolume.Name = "proxy-server-root-certificate"
		rootCertificateVolume.Secret = &corev1.SecretVolumeSource{
			SecretName: "proxy-server-root-certificate",
		}
		ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, rootCertificateVolume)
	}

	containers := ds.Spec.Template.Spec.Containers

	// Remove any existing proxy containers...
	for i, c := range containers {
		if c.Name == "karavi-authorization-proxy" {
			containers = append(containers[:i], containers[i+1:]...)
		}
	}

	proxyContainer := buildProxyContainer(ds.Namespace, lc.InjectResources.Secret, imageAddr, proxyHost, insecure)
	containers = append(containers, *proxyContainer)
	ds.Spec.Template.Spec.Containers = containers

	ds.Annotations["com.dell.karavi-authorization-proxy"] = "true"

	// Append it to the list of items.
	enc, err := json.Marshal(&ds)
	if err != nil {
		lc.Err = err
		return
	}
	raw := runtime.RawExtension{
		Raw: enc,
	}
	lc.Modified.Items = append(lc.Modified.Items, raw)
}

type IsilonCreds struct {
	IsilonClusters []IsilonCluster `json:"isilonClusters"`
}

type IsilonCluster struct {
	ClusterName               string `json:"clusterName"`
	Username                  string `json:"username"`
	Password                  string `json:"password"`
	Endpoint                  string `json:"endpoint"`
	IsDefault                 bool   `json:"isDefault"`
	IsiPort                   string `json:"isiPort"`
	SkipCertificateValidation bool   `json:"skipCertificateValidation"`
	IsiPath                   string `json:"isiPath"`
}

func (lc *ListChangeForPowerScale) ExtractSecretData(s *corev1.Secret) ([]SecretData, error) {
	data, ok := s.Data["config"]
	if !ok {
		return nil, errors.New("missing config key")
	}

	var creds IsilonCreds
	err := json.NewDecoder(bytes.NewReader(data)).Decode(&creds)
	if err != nil {
		// Got an error with JSON decode, try to decode as YAML
		yamlErr := yaml.Unmarshal(data, &creds)
		if yamlErr != nil {
			return nil, fmt.Errorf("decoding secret data: yaml error: %v, json error: %v", yamlErr, err)
		}
	}

	ret := make([]SecretData, len(creds.IsilonClusters))
	for i, cluster := range creds.IsilonClusters {
		port := "8080"
		if cluster.IsiPort != "" {
			port = cluster.IsiPort
		}

		endpoint := fmt.Sprintf("https://%s:%s", cluster.Endpoint, port)
		ret[i].Username = cluster.Username
		ret[i].Password = cluster.Password
		ret[i].Endpoint = endpoint
		ret[i].Insecure = cluster.SkipCertificateValidation
		ret[i].SystemID = cluster.ClusterName
		ret[i].IsDefault = cluster.IsDefault
	}
	return ret, nil
}
