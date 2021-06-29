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
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	yamlDecoder "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

// NewInjectCmd creates a new inject command
func NewInjectCmd() *cobra.Command {
	injectCmd := &cobra.Command{
		Use:   "inject",
		Short: "Inject the sidecar proxy into to a CSI driver pod",
		Long: `Injects the sidecar proxy into a CSI driver pod.
	
	You can inject resources coming from stdin.
	
	Usage:
	karavictl inject [flags]
	
	Examples:
	# Inject into an existing vxflexos CSI driver 
	kubectl get secrets,deployments,daemonsets -n vxflexos -o yaml \
	  | karavictl inject --image-addr 1.1.1.1:5000/sidecar-proxy:latest --proxy-host 1.1.1.1\
	  | kubectl apply -f -
	  `,
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

			insecure, err := cmd.Flags().GetBool("insecure")
			if err != nil {
				log.Fatal(err)
			}

			rootCertificate, err := cmd.Flags().GetString("root-certificate")
			if err != nil {
				log.Fatal(err)
			}

			proxyPortFlags, err := cmd.Flags().GetStringSlice("proxy-port")
			if err != nil {
				log.Fatal(err)
			}

			portRanges, err := getStartingPortRanges(proxyPortFlags)
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
					resource, err = injectUsingList(bytes, imageAddr, proxyHost, rootCertificate, portRanges, insecure)
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

	injectCmd.Flags().String("proxy-host", "", "Help message for proxy-host")
	injectCmd.Flags().String("image-addr", "", "Help message for image-addr")
	injectCmd.Flags().Bool("insecure", false, "Allow insecure connections from sidecar-proxy to proxy-server (default: false)")
	injectCmd.Flags().String("root-certificate", "", "The root certificate file used by the proxy server")
	injectCmd.Flags().StringSlice("proxy-port", []string{}, "proxy start port in the form <storageSystemType>=<startingPort>")
	return injectCmd
}

func buildProxyContainer(pluginID, configName, imageAddr, proxyHost string, insecure bool) *corev1.Container {
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
				Name:  "INSECURE",
				Value: fmt.Sprintf("%v", insecure),
			},
			corev1.EnvVar{
				Name:  "PLUGIN_IDENTIFIER",
				Value: pluginID,
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
				Name:      configName,
			},
			corev1.VolumeMount{
				MountPath: "/etc/karavi-authorization/root-certificates",
				Name:      "proxy-server-root-certificate",
			},
		},
	}

	return &proxyContainer
}

const (
	// DefaultStartingPortRange is the starting port number
	DefaultStartingPortRange = 9000
)

// ListChangeForMultiArray holds a k8s list and a modified version of said list
type ListChangeForMultiArray struct {
	*ListChange
	StartingPortRange int
}

// ListChangeForPowerMax holds a k8s list and a modified version of said list for powermax
type ListChangeForPowerMax struct {
	*ListChange
	Endpoint          string // only useful for powermax
	StartingPortRange int
}

//ListChange holds a k8s list and a modified version of said list
type ListChange struct {
	Existing        *corev1.List
	Modified        *corev1.List
	InjectResources *Resources
	Namespace       string
	Err             error // sticky error
}

// Resources contains the workload resources that will be injected with the sidecar
type Resources struct {
	Deployment   string
	DaemonSet    string
	Secret       string
	ReverseProxy string
	ConfigMap    string
}

// ListChanger is an interface for changes needed in a list
type ListChanger interface {
	// Change modifies the resources for ListChangeForMultiArray or ListChangeForPowerMax
	Change(existing *corev1.List, imageAddr, proxyHost, rootCertificate string, insecure bool) (*corev1.List, error)
}

// NewListChange returns a new ListChangeForMultiArray from a k8s list
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

func injectUsingList(b []byte, imageAddr, proxyHost, rootCertificate string, startingPortRange map[string]int, insecure bool) (*corev1.List, error) {

	var l corev1.List
	err := yaml.Unmarshal(b, &l)
	if err != nil {
		return nil, err
	}

	// TODO(ian): Determine CSI driver type: vxflexos, powerscale, etc.?
	// The configs are assumed to contain the type, e.g. "vxflexos-config".
	var change ListChanger
	if strings.Contains(string(b), "powermax") {
		change = &ListChangeForPowerMax{StartingPortRange: startingPortRange["powermax"]}
	} else if strings.Contains(string(b), "isilon") || strings.Contains(string(b), "powerscale") {
		change = &ListChangeForPowerScale{StartingPortRange: startingPortRange["powerscale"]}
	} else {
		change = &ListChangeForMultiArray{StartingPortRange: startingPortRange["powerflex"]}
	}

	return change.Change(&l, imageAddr, proxyHost, rootCertificate, insecure)

}

// Change modifies the resources for ListChangeForPowerMax
func (lc *ListChangeForPowerMax) Change(existing *corev1.List, imageAddr, proxyHost, rootCertificate string, insecure bool) (*corev1.List, error) {
	lc.ListChange = NewListChange(existing)
	lc.setInjectedResources()
	lc.injectRootCertificate(rootCertificate)
	lc.injectKaraviSecret(insecure)
	lc.injectIntoDeployment(imageAddr, proxyHost, insecure)
	lc.injectIntoDaemonset(imageAddr, proxyHost, insecure)
	lc.injectIntoReverseProxy(imageAddr, proxyHost, insecure)
	return lc.ListChange.Modified, lc.ListChange.Err
}

// Change modifies the resources for ListChangeForMultiArray
func (lc *ListChangeForMultiArray) Change(existing *corev1.List, imageAddr, proxyHost, rootCertificate string, insecure bool) (*corev1.List, error) {
	lc.ListChange = NewListChange(existing)
	// Determine what we are injecting the sidecar into (e.g. powerflex csi driver, observability, etc)
	lc.setInjectedResources()
	// Inject the rootCA certificate as a Secret
	lc.injectRootCertificate(rootCertificate)
	// Inject our own secret based on the original config.
	lc.injectKaraviSecret()
	// Inject the sidecar proxy into the Deployment and update
	// the config volume to point to our own secret.
	lc.injectIntoDeployment(imageAddr, proxyHost, insecure)
	// Inject into the Daemonset.
	lc.injectIntoDaemonset(imageAddr, proxyHost, insecure)

	return lc.ListChange.Modified, lc.ListChange.Err
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
			Secret:     "vxflexos-config",
		}
		lc.Namespace = deployments["vxflexos-controller"].Namespace
	// injecting into powermax csi driver
	case deployments["powermax-controller"] != nil:
		lc.InjectResources = &Resources{
			Deployment:   "powermax-controller",
			DaemonSet:    "powermax-node",
			Secret:       "powermax-creds",
			ReverseProxy: "powermax-reverseproxy",
			ConfigMap:    "powermax-reverseproxy-config",
		}
		lc.Namespace = deployments["powermax-controller"].Namespace
	// injecting into powerscale csi driver
	case deployments["isilon-controller"] != nil:
		lc.InjectResources = &Resources{
			Deployment: "isilon-controller",
			DaemonSet:  "isilon-node",
			Secret:     "isilon-creds",
		}
		lc.Namespace = deployments["isilon-controller"].Namespace
	// injecting into observability
	case deployments["karavi-metrics-powerflex"] != nil:
		lc.InjectResources = &Resources{
			Deployment: "karavi-metrics-powerflex",
			Secret:     "vxflexos-config",
		}
		lc.Namespace = deployments["karavi-metrics-powerflex"].Namespace
	default:
		err := errors.New("unable to determine what resources should be injected")
		lc.Err = err
	}
}

func (lc *ListChange) injectRootCertificate(rootCertificate string) {
	if lc.Err != nil {
		return
	}

	rootCertificateContent := []byte("")

	if rootCertificate != "" {
		var err error
		rootCertificateContent, err = ioutil.ReadFile(filepath.Clean(rootCertificate))
		if err != nil {
			lc.Err = fmt.Errorf("reading root certificate: %w", err)
			return
		}
	}

	// create a new Secret or overwrite the existing Secret so that we can support updating the root certificate
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proxy-server-root-certificate",
			Namespace: lc.Namespace,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		Type: "Opaque",
		Data: map[string][]byte{
			"rootCertificate.pem": rootCertificateContent,
		},
	}

	// Append it to the list of items.
	enc, err := json.Marshal(&secret)
	if err != nil {
		lc.Err = err
		return
	}
	raw := runtime.RawExtension{
		Raw: enc,
	}
	lc.Modified.Items = append(lc.Modified.Items, raw)
}

// GetCommandEnv get environment variable for powerflex deployment
func (lc *ListChangeForPowerMax) GetCommandEnv(deploy *appsv1.Deployment, s *corev1.Secret, insecure bool) ([]SecretData, error) {

	endpoint := ""
	systemIDs := ""

	foundEndpoint := false
	for _, c := range deploy.Spec.Template.Spec.Containers {
		if c.Name == "driver" {
			for _, e := range c.Env {
				if e.Name == "CSM_CSI_POWERMAX_ENDPOINT" {
					endpoint = e.Value
					foundEndpoint = true
					break
				}
			}
			break
		}
	}

	for _, c := range deploy.Spec.Template.Spec.Containers {
		if c.Name == "driver" {
			for _, e := range c.Env {
				if e.Name == "X_CSI_POWERMAX_ENDPOINT" {
					if !foundEndpoint {
						endpoint = e.Value
						foundEndpoint = true
					}
				}
				if e.Name == "X_CSI_POWERMAX_ARRAYS" {
					systemIDs = e.Value
				}
			}
			break
		}
	}

	if endpoint == "" || systemIDs == "" {
		return nil, errors.New("could not find endpoint or system ID")
	}

	return []SecretData{{Endpoint: endpoint,
		Username: string(s.Data["username"][:]),
		Password: string(s.Data["password"][:]),
		SystemID: systemIDs,
		Insecure: insecure},
	}, nil
}

func (lc *ListChangeForMultiArray) injectKaraviSecret() {
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
	configSecData, err := getSecretData(configSecret)
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

func (lc *ListChangeForPowerMax) injectKaraviSecret(insecure bool) {
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

	configSecData, err := lc.GetCommandEnv(deploy, configSecret, insecure)
	if err != nil {
		lc.Err = fmt.Errorf("getting command env: %w", err)
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

	lc.Endpoint = configSecData[0].Endpoint

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

func (lc *ListChangeForPowerMax) injectIntoReverseProxy(imageAddr, proxyHost string, insecure bool) {
	if lc.Err != nil {
		return
	}

	if lc.ListChange.InjectResources.ReverseProxy == "" {
		return
	}

	m, err := buildMapOfDeploymentsFromList(lc.ListChange.Existing)
	if err != nil {
		lc.Err = err
		return
	}

	deploy, ok := m[lc.InjectResources.ReverseProxy]
	if !ok {
		return
	}

	// Set configMAP
	cm, err := buildMapOfConfigMapsFromList(lc.ListChange.Existing)
	if err != nil {
		lc.Err = err
		return
	}

	configMap, ok := cm[lc.InjectResources.ConfigMap]
	if !ok {
		lc.Err = errors.New("configMap not found")
		return
	}

	configmapData, ok := configMap.Data["config.yaml"]
	if !ok {
		lc.Err = errors.New("config.yaml not found in configMap")
		return
	}

	re := regexp.MustCompile(`https://(.+)`)
	configMap.Data["config.yaml"] = strings.Replace(configmapData, string(re.Find([]byte(configmapData))), lc.Endpoint, 1)

	enc, err := json.Marshal(&configMap)
	if err != nil {
		lc.Err = err
		return
	}
	raw := runtime.RawExtension{
		Raw: enc,
	}
	lc.Modified.Items = append(lc.Modified.Items, raw)

	secretName := "karavi-authorization-config"
	authVolume := corev1.Volume{}
	authVolume.Name = "karavi-authorization-config"
	authVolume.Secret = &corev1.SecretVolumeSource{
		SecretName: secretName,
	}
	deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, authVolume)

	rootCertificateMounted := false
	volumes := deploy.Spec.Template.Spec.Volumes
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
	pluginID := deploy.Namespace

	// Remove any existing proxy containers and check for observability containers...
	for i, c := range containers {
		if c.Name == "karavi-authorization-proxy" {
			containers = append(containers[:i], containers[i+1:]...)
		} else if c.Name == "karavi-metrics-powerflex" {
			pluginID = "powerflex"
		} else if c.Name == "karavi-metrics-powerstore" {
			pluginID = "powerstore"
		}
	}

	// Add a new proxy container...
	proxyContainer := buildProxyContainer(pluginID, secretName, imageAddr, proxyHost, insecure)
	containers = append(containers, *proxyContainer)
	deploy.Spec.Template.Spec.Containers = containers

	deploy.Annotations["com.dell.karavi-authorization-proxy"] = "true"

	// Append it to the list of items.
	enc, err = json.Marshal(&deploy)
	if err != nil {
		lc.Err = err
		return
	}
	raw = runtime.RawExtension{
		Raw: enc,
	}
	lc.Modified.Items = append(lc.Modified.Items, raw)
}

func (lc *ListChangeForPowerMax) injectIntoDeployment(imageAddr, proxyHost string, insecure bool) {
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

	secretName := "karavi-authorization-config"
	authVolume := corev1.Volume{}
	authVolume.Name = "karavi-authorization-config"
	authVolume.Secret = &corev1.SecretVolumeSource{
		SecretName: secretName,
	}
	deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, authVolume)

	rootCertificateMounted := false
	volumes := deploy.Spec.Template.Spec.Volumes
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
	pluginID := deploy.Namespace

	// Remove any existing proxy containers and check for observability containers...
	for i, c := range containers {
		if c.Name == "karavi-authorization-proxy" {
			containers = append(containers[:i], containers[i+1:]...)
		} else if c.Name == "karavi-metrics-powerflex" {
			pluginID = "powerflex"
		} else if c.Name == "karavi-metrics-powerstore" {
			pluginID = "powerstore"
		}

	}

	var endpoint string
	for i, c := range containers {
		if c.Name == "driver" {
			commandEnvFlag := false
			for j, e := range c.Env {
				if e.Name == "X_CSI_POWERMAX_ENDPOINT" {
					endpoint = containers[i].Env[j].Value
					containers[i].Env[j].Value = lc.Endpoint
					commandEnvFlag = true

				}
			}
			if !commandEnvFlag {
				lc.Err = errors.New("X_CSI_POWERMAX_ENDPOINT not found")
				return
			}
			break
		}
	}

	for i, c := range containers {
		if c.Name == "driver" {
			foundEndpoint := false
			for _, e := range c.Env {
				if e.Name == "CSM_CSI_POWERMAX_ENDPOINT" {
					foundEndpoint = true
					break
				}
			}
			if !foundEndpoint {
				containers[i].Env = append(containers[i].Env, corev1.EnvVar{
					Name:  "CSM_CSI_POWERMAX_ENDPOINT",
					Value: endpoint,
				})
			}
			break
		}
	}

	// Add a new proxy container...
	proxyContainer := buildProxyContainer(pluginID, secretName, imageAddr, proxyHost, insecure)
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

func (lc *ListChangeForMultiArray) injectIntoDeployment(imageAddr, proxyHost string, insecure bool) {
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
	pluginID := deploy.Namespace
	isDriver := true

	// Remove any existing proxy containers and check for observability containers...
	for i, c := range containers {
		if c.Name == "karavi-authorization-proxy" {
			containers = append(containers[:i], containers[i+1:]...)
		} else if c.Name == "karavi-metrics-powerflex" {
			pluginID = "powerflex"
			isDriver = false
		} else if c.Name == "karavi-metrics-powerstore" {
			pluginID = "powerstore"
			isDriver = false
		}
	}

	// Add a new proxy container...
	proxyContainer := buildProxyContainer(pluginID, lc.InjectResources.Secret, imageAddr, proxyHost, insecure)
	if isDriver {
		proxyContainer.VolumeMounts = append(proxyContainer.VolumeMounts, corev1.VolumeMount{
			MountPath: "/etc/karavi-authorization",
			Name:      "vxflexos-config-params", // {{ .Release.Name }}-config-params
		})
		proxyContainer.Args = append(proxyContainer.Args, "--driver-config-params=/etc/karavi-authorization/driver-config-params.yaml")
	}
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

func (lc *ListChangeForPowerMax) injectIntoDaemonset(imageAddr, proxyHost string, insecure bool) {
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

	secretName := "karavi-authorization-config"
	authVolume := corev1.Volume{}
	authVolume.Name = "karavi-authorization-config"
	authVolume.Secret = &corev1.SecretVolumeSource{
		SecretName: secretName,
	}
	ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, authVolume)

	volumes := ds.Spec.Template.Spec.Volumes
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

	for i, c := range containers {
		if c.Name == "driver" {
			commandEnvFlag := false
			for j, e := range c.Env {
				if e.Name == "X_CSI_POWERMAX_ENDPOINT" {
					containers[i].Env[j].Value = lc.Endpoint
					commandEnvFlag = true
				}
			}
			if !commandEnvFlag {
				lc.Err = errors.New("X_CSI_POWERMAX_ENDPOINT not found")
				return
			}
			break
		}
	}

	proxyContainer := buildProxyContainer(ds.Namespace, secretName, imageAddr, proxyHost, insecure)
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

func (lc *ListChangeForMultiArray) injectIntoDaemonset(imageAddr, proxyHost string, insecure bool) {
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
	proxyContainer.VolumeMounts = append(proxyContainer.VolumeMounts, corev1.VolumeMount{
		MountPath: "/etc/karavi-authorization",
		Name:      "vxflexos-config-params", // {{ .Release.Name }}-config-params
	})
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

func buildMapOfConfigMapsFromList(list *corev1.List) (map[string]*corev1.ConfigMap, error) {
	ret := make(map[string]*corev1.ConfigMap)
	for _, v := range list.Items {
		var meta metav1.TypeMeta
		err := yaml.Unmarshal(v.Raw, &meta)
		if err != nil {
			return nil, err
		}
		switch meta.Kind {
		case "ConfigMap":
			var configMap corev1.ConfigMap
			err := yaml.Unmarshal(v.Raw, &configMap)
			if err != nil {
				return nil, err
			}
			ret[configMap.Name] = &configMap
		}
	}

	return ret, nil
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
	log.Println(string(data))
	err := json.NewDecoder(bytes.NewReader(data)).Decode(&ret)
	if err != nil {
		// Got an error with JSON decode, try to decode as YAML
		yamlErr := yaml.Unmarshal(data, &ret)
		if yamlErr != nil {
			return nil, fmt.Errorf("decoding secret data: yaml error: %v, json error: %v", yamlErr, err)
		}
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

func getStartingPortRanges(proxyPortFlags []string) (map[string]int, error) {
	if len(proxyPortFlags) == 0 {
		return map[string]int{
			"powerflex": DefaultStartingPortRange,
			"powermax":  DefaultStartingPortRange + 200,
		}, nil
	}

	portRanges := make(map[string]int)
	for _, v := range proxyPortFlags {
		t := strings.Split(v, "=")
		if len(t) < 2 {
			return nil, fmt.Errorf("invalid proxy flag: %s: no port provided", proxyPortFlags)
		}
		port, err := strconv.Atoi(t[1])
		if err != nil {
			return nil, fmt.Errorf("invalid port: %s", t[1])
		}
		portRanges[t[0]] = port
	}

	fillUnspecifiedPortRanges(portRanges)

	return portRanges, nil
}

func fillUnspecifiedPortRanges(portRanges map[string]int) {
	storageIndicies := map[string]int{
		"powerflex": 0,
		"powermax":  1,
	}
	storageTypes := []string{"powerflex", "powermax"}

	var referenceStorageSystem string
	var referencePort int
	for k, v := range portRanges {
		referenceStorageSystem = k
		referencePort = v
		break
	}

	storageIndex := storageIndicies[referenceStorageSystem]

	for i := storageIndex + 1; i < len(storageTypes); i++ {
		storage := storageTypes[i]
		if _, ok := portRanges[storage]; ok {
			continue
		}
		portRanges[storage] = referencePort + (i * 200)
	}

	for i := storageIndex - 1; i >= 0; i-- {
		storage := storageTypes[i]
		if _, ok := portRanges[storage]; ok {
			continue
		}
		portRanges[storage] = referencePort - ((i + 1) * 200)
	}
}
