/*
Copyright © 2020 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Short: "Add Karavi-Security to a CSI driver",
	Long: `Add Karavi-Security to a CSI driver.

You can inject resources coming from stdin.

Usage:
karavictl inject [flags]

Examples:
# Inject into an existing vxflexos CSI driver 
kubectl get deploy/vxflexos-controller | karavictl inject | kubectl apply -f -`,
	Run: func(cmd *cobra.Command, args []string) {
		info, err := os.Stdin.Stat()
		if err != nil {
			panic(err)
		}

		if info.Mode()&os.ModeCharDevice != 0 {
			fmt.Println("The command is intended to work with pipes.")
			return
		}

		imageAddr, err := cmd.Flags().GetString("image-addr")
		if err != nil {
			log.Fatal(err)
		}

		proxyAddr, err := cmd.Flags().GetString("proxy-addr")
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
			case "Deployment":
				resource, err = injectDeployment(bytes, imageAddr, proxyAddr)
			case "DaemonSet":
				resource, err = injectDaemonSet(bytes, imageAddr, proxyAddr)
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
	injectCmd.Flags().String("proxy-addr", "", "Help message for proxy-addr")
	injectCmd.Flags().String("image-addr", "", "Help message for image-addr")
}

func injectDeployment(b []byte, imageAddr, proxyAddr string) (*appsv1.Deployment, error) {
	deploy := appsv1.Deployment{}
	err := yaml.Unmarshal(b, &deploy)
	if err != nil {
		log.Fatal(err)
	}

	proxyContainer, vol := buildProxyContainer(imageAddr, proxyAddr)
	deploy.Annotations["com.dell.karavi-authorization-proxy"] = "true"
	deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, *vol)

	containers := deploy.Spec.Template.Spec.Containers
	for i, v := range containers {
		if v.Name != CSIDriverContainerName {
			continue
		}
		for j, e := range containers[i].Env {
			if e.Name != CSIDriverEndpointEnvName {
				continue
			}
			containers[i].Env[j].Value = "https://localhost:8443"
		}
	}

	containers = append(containers, *proxyContainer)
	deploy.Spec.Template.Spec.Containers = containers

	return &deploy, nil
}

func injectDaemonSet(b []byte, imageAddr, proxyAddr string) (*appsv1.DaemonSet, error) {
	ds := appsv1.DaemonSet{}
	err := yaml.Unmarshal(b, &ds)
	if err != nil {
		log.Fatal(err)
	}

	proxyContainer, vol := buildProxyContainer(imageAddr, proxyAddr)
	ds.Annotations["com.dell.karavi-authorization-proxy"] = "true"
	ds.Spec.Template.Spec.Volumes = append(ds.Spec.Template.Spec.Volumes, *vol)

	containers := ds.Spec.Template.Spec.Containers
	for i, v := range containers {
		if v.Name != CSIDriverContainerName {
			continue
		}
		for j, e := range containers[i].Env {
			if e.Name != CSIDriverEndpointEnvName {
				continue
			}
			//containers[i].Env[j].Value = "https://karavi-proxy.vxflexos.svc.cluster.local:8443"
			containers[i].Env[j].Value = "https://localhost:8443"
		}
	}

	containers = append(containers, *proxyContainer)
	ds.Spec.Template.Spec.Containers = containers

	return &ds, nil
}

func buildProxyContainer(imageAddr, proxyAddr string) (*corev1.Container, *corev1.Volume) {
	proxyContainer := corev1.Container{
		Image:           imageAddr,
		Name:            "karavi-authorization-proxy",
		ImagePullPolicy: corev1.PullAlways,
		Env: []corev1.EnvVar{
			corev1.EnvVar{
				Name:  "PROXY_ADDR",
				Value: proxyAddr,
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
				MountPath: "/etc/app",
				Name:      "ksec-keys-dir",
			},
		},
	}

	vol := corev1.Volume{
		Name: "ksec-keys-dir",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "ksec-keys-config",
				},
			},
		},
	}

	return &proxyContainer, &vol
}
