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
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	yamlDecoder "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

// Common constants.
const (
	CSIDriverContainerName   = "driver"
	CSIDriverEndpointEnvName = "X_CSI_VXFLEXOS_ENDPOINT"
)

// injectCmd represents the inject command
var injectCmd = &cobra.Command{
	Use:   "inject",
	Short: "Inject the sidecar proxy into to a CSI driver pod",
	Long: `Injects the sidecar proxy into a CSI driver pod.

You can inject resources coming from stdin.

Usage:
karavictl inject [flags]

Examples:
# Inject into an existing vxflexos CSI driver 
kubectl get secrets,deployments,daemonsets -n vxflexos -o yaml \
  | karavictl inject --image-addr 10.0.0.1:5000/sidecar-proxy:latest --proxy-host 10.0.0.1 \
  | kubectl apply -f -`,
	Run: func(cmd *cobra.Command, args []string) {
		info, err := os.Stdin.Stat()
		if err != nil {
			panic(err)
		}

		if info.Mode()&os.ModeCharDevice != 0 {
			fmt.Fprintln(os.Stderr, "The command is intended to work with pipes.")
			return
		}

		imageAddr, err := cmd.Flags().GetString("image-addr")
		if err != nil {
			log.Fatal(err)
		}

		proxyHost, err := cmd.Flags().GetString("proxy-host")
		if err != nil {
			log.Fatal(err)
		}

		guestAccessToken, err := cmd.Flags().GetString("guest-access-token")
		if err != nil {
			log.Fatal(err)
		}

		guestRefreshToken, err := cmd.Flags().GetString("guest-refresh-token")
		if err != nil {
			log.Fatal(err)
		}

		buf := bufio.NewReaderSize(os.Stdin, 4096)
		reader := yamlDecoder.NewYAMLReader(buf)

		for {
			bytes, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				log.Fatal(err)
			}

			var meta metav1.TypeMeta
			err = yaml.Unmarshal(bytes, &meta)
			if err != nil {
				log.Fatal(err)
			}

			var resource interface{}
			switch meta.Kind {
			case "List":
				resource, err = injectUsingList(bytes, imageAddr, proxyHost, guestAccessToken, guestRefreshToken)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error: %+v\n", err)
					return
				}
			default:
				fmt.Fprintln(os.Stderr, "This command works with a List of Kubernetes resources from within a CSI driver namespace.")
				return
			}
			b, err := yaml.Marshal(&resource)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(string(b))
		}
	},
}

func init() {
	rootCmd.AddCommand(injectCmd)
	injectCmd.Flags().String("proxy-host", "", "Help message for proxy-host")
	injectCmd.Flags().String("image-addr", "", "Help message for image-addr")
	injectCmd.Flags().String("guest-access-token", "", "Access token")
	injectCmd.Flags().String("guest-refresh-token", "", "Refresh token")
}

func buildProxyContainer(imageAddr, proxyHost string) *corev1.Container {
	proxyContainer := corev1.Container{
		Image:           imageAddr,
		Name:            "karavi-authorization-proxy",
		ImagePullPolicy: corev1.PullAlways,
		Env: []corev1.EnvVar{
			corev1.EnvVar{
				Name:  "PROXY_HOST",
				Value: proxyHost,
			},
			corev1.EnvVar{
				Name:  "PLUGIN_IDENTIFIER",
				Value: "csi-vxflexos", // TODO(ian): Get this dynamically; can we rely on the namespace name?
			},
			corev1.EnvVar{
				Name: "ACCESS_TOKEN",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: "access",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "proxy-authz-tokens",
						},
					},
				}},
			corev1.EnvVar{
				Name: "REFRESH_TOKEN",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						Key: "refresh",
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "proxy-authz-tokens",
						},
					},
				}},
		},
		VolumeMounts: []corev1.VolumeMount{
			corev1.VolumeMount{
				MountPath: "/etc/karavi-authorization/config",
				Name:      "vxflexos-config",
			},
		},
	}

	return &proxyContainer
}

// ListChange holds a k8s list and a modified version of said list
type ListChange struct {
	Existing        *corev1.List
	Modified        *corev1.List
	InjectResources *Resources
	Namespace       string
	Err             error // sticky error
}

// Resources contains the workload resources that will be injected with the sidecar
type Resources struct {
	Deployment string
	DaemonSet  string
}

// NewListChange returns a new ListChange from a k8s list
func NewListChange(existing *corev1.List) *ListChange {
	return &ListChange{
		Existing: existing,
		Modified: &corev1.List{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "List",
			},
		},
	}
}

func injectUsingList(b []byte, imageAddr, proxyHost,
	guestAccessToken, guestRefreshToken string) (*corev1.List, error) {

	var l corev1.List
	err := yaml.Unmarshal(b, &l)
	if err != nil {
		return nil, err
	}

	// TODO(ian): Determine CSI driver type: vxflexos, powerscale, etc.?
	// The configs are assumed to contain the type, e.g. "vxflexos-config".

	change := NewListChange(&l)
	// Determine what we are injecting the sidecar into (e.g. powerflex csi driver, observability, etc)
	change.setInjectedResources()
	// Inject a pair of tokens encoded with the Guest tenant/role.
	change.injectGuestTokenSecret(guestAccessToken, guestRefreshToken)
	// Inject our own secret based on the original config.
	change.injectKaraviSecret()
	// Inject the sidecar proxy into the Deployment and update
	// the config volume to point to our own secret.
	change.injectIntoDeployment(imageAddr, proxyHost)
	// Inject into the Daemonset.
	change.injectIntoDaemonset(imageAddr, proxyHost)

	return change.Modified, change.Err
}

func (lc *ListChange) setInjectedResources() {
	deployments, err := buildMapOfDeploymentsFromList(lc.Existing)
	if err != nil {
		lc.Err = err
		return
	}

	switch {
	// injecting into vxflexos csi driver
	case deployments["vxflexos-controller"] != nil:
		lc.InjectResources = &Resources{
			Deployment: "vxflexos-controller",
			DaemonSet:  "vxflexos-node",
		}
		lc.Namespace = deployments["vxflexos-controller"].Namespace
	// injecting into observability
	case deployments["karavi-metrics-powerflex"] != nil:
		lc.InjectResources = &Resources{
			Deployment: "karavi-metrics-powerflex",
		}
		lc.Namespace = deployments["karavi-metrics-powerflex"].Namespace
	default:
		err := errors.New("unable to determine what resources should be injected")
		lc.Err = err
	}
}

func (lc *ListChange) injectGuestTokenSecret(accessToken, refreshToken string) {
	if lc.Err != nil {
		return
	}

	// Extract all of the Secret resources.
	secrets, err := buildMapOfSecretsFromList(lc.Existing)
	if err != nil {
		lc.Err = fmt.Errorf("building secret map: %w", err)
		return
	}

	// Determine if tokens already exist.
	if _, ok := secrets["proxy-authz-tokens"]; ok {
		// no further processing required.
		return
	}

	// Create the new Secret.
	newSecret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proxy-authz-tokens",
			Namespace: lc.Namespace,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		Type: "Opaque",
		Data: map[string][]byte{
			"access":  []byte(accessToken),
			"refresh": []byte(refreshToken),
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

func (lc *ListChange) injectKaraviSecret() {
	if lc.Err != nil {
		return
	}

	// Extract all of the Secret resources.
	secrets, err := buildMapOfSecretsFromList(lc.Existing)
	if err != nil {
		lc.Err = fmt.Errorf("building secret map: %w", err)
		return
	}

	// Pick out the config.
	configSecret, ok := secrets["vxflexos-config"]
	if !ok {
		lc.Err = errors.New("config secret not found")
		return
	}

	// Get the config data.
	configSecData, err := getSecretData(configSecret)
	if err != nil {
		lc.Err = fmt.Errorf("getting secret data: %w", err)
		return
	}

	// Copy the config data and convert endpoints to localhost:<port>
	configSecData = convertEndpoints(configSecData, 9000)
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

func buildMapOfDeploymentsFromList(list *corev1.List) (map[string]*appsv1.Deployment, error) {
	ret := make(map[string]*appsv1.Deployment)

	for _, v := range list.Items {
		var meta metav1.TypeMeta
		err := yaml.Unmarshal(v.Raw, &meta)
		if err != nil {
			return nil, err
		}

		switch meta.Kind {
		case "Deployment":
			var deploy appsv1.Deployment
			err := yaml.Unmarshal(v.Raw, &deploy)
			if err != nil {
				return nil, err
			}
			ret[deploy.Name] = &deploy
		}
	}

	return ret, nil
}

func buildMapOfDaemonsetsFromList(list *corev1.List) (map[string]*appsv1.DaemonSet, error) {
	ret := make(map[string]*appsv1.DaemonSet)

	for _, v := range list.Items {
		var meta metav1.TypeMeta
		err := yaml.Unmarshal(v.Raw, &meta)
		if err != nil {
			return nil, err
		}

		switch meta.Kind {
		case "DaemonSet":
			var ds appsv1.DaemonSet
			err := yaml.Unmarshal(v.Raw, &ds)
			if err != nil {
				return nil, err
			}
			ret[ds.Name] = &ds
		}
	}

	return ret, nil
}

func buildMapOfSecretsFromList(list *corev1.List) (map[string]*corev1.Secret, error) {
	ret := make(map[string]*corev1.Secret)

	for _, v := range list.Items {
		var meta metav1.TypeMeta
		err := yaml.Unmarshal(v.Raw, &meta)
		if err != nil {
			return nil, err
		}

		switch meta.Kind {
		case "Secret":
			var secret corev1.Secret
			err := yaml.Unmarshal(v.Raw, &secret)
			if err != nil {
				return nil, err
			}
			ret[secret.Name] = &secret
		}
	}

	return ret, nil
}

// SecretData holds k8s secret data for a backend storage system
type SecretData struct {
	Username         string `json:"username"`
	Password         string `json:"password"`
	IntendedEndpoint string `json:"intendedEndpoint"`
	Endpoint         string `json:"endpoint"`
	SystemID         string `json:"systemID"`
	Insecure         bool   `json:"insecure"`
	IsDefault        bool   `json:"isDefault"`
}

func getSecretData(s *corev1.Secret) ([]SecretData, error) {
	data, ok := s.Data["config"]
	if !ok {
		return nil, errors.New("missing config key")
	}

	var ret []SecretData
	err := json.NewDecoder(bytes.NewReader(data)).Decode(&ret)
	if err != nil {
		return nil, fmt.Errorf("decoding secret data json: %w", err)
	}

	return ret, nil
}

func convertEndpoints(s []SecretData, startingPortRange int) []SecretData {
	var ret []SecretData
	for _, v := range s {
		v.IntendedEndpoint = v.Endpoint
		v.Endpoint = fmt.Sprintf("https://localhost:%d", startingPortRange)
		startingPortRange++
		ret = append(ret, v)
	}
	return ret
}

func scrubLoginCredentials(s []SecretData) []SecretData {
	var ret []SecretData
	for _, v := range s {
		v.Username, v.Password = "-", "-"
		ret = append(ret, v)
	}
	return ret
}

func (lc *ListChange) injectIntoDeployment(imageAddr, proxyHost string) {
	if lc.Err != nil {
		return
	}

	if lc.InjectResources.Deployment == "" {
		return
	}

	m, err := buildMapOfDeploymentsFromList(lc.Existing)
	if err != nil {
		lc.Err = err
		return
	}

	deploy, ok := m[lc.InjectResources.Deployment]
	if !ok {
		lc.Err = errors.New("deployment not found")
		return
	}

	volumes := deploy.Spec.Template.Spec.Volumes
	for i, v := range volumes {
		if v.Name != "vxflexos-config" {
			continue
		}
		volumes[i].Secret.SecretName = "karavi-authorization-config"
	}

	containers := deploy.Spec.Template.Spec.Containers

	// Remove any existing proxy containers...
	for i, c := range containers {
		if c.Name == "karavi-authorization-proxy" {
			containers = append(containers[:i], containers[i+1:]...)
		}
	}

	// Add a new proxy container...
	proxyContainer := buildProxyContainer(imageAddr, proxyHost)
	containers = append(containers, *proxyContainer)
	deploy.Spec.Template.Spec.Containers = containers

	deploy.Annotations["com.dell.karavi-authorization-proxy"] = "true"

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

func (lc *ListChange) injectIntoDaemonset(imageAddr, proxyHost string) {
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
		if v.Name != "vxflexos-config" {
			continue
		}
		volumes[i].Secret.SecretName = "karavi-authorization-config"
	}

	containers := ds.Spec.Template.Spec.Containers

	// Remove any existing proxy containers...
	for i, c := range containers {
		if c.Name == "karavi-authorization-proxy" {
			containers = append(containers[:i], containers[i+1:]...)
		}
	}

	proxyContainer := buildProxyContainer(imageAddr, proxyHost)
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
