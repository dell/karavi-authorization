// Copyright © 2021-2022 Dell Inc., or its subsidiaries. All Rights Reserved.
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

// Command deploy is used to install the application using embedded
// resources.
package main

import (
	"archive/tar"
	"compress/gzip"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sYaml "sigs.k8s.io/yaml"
)

// Overrides for testing purposes.
var (
	gzipNewReader        = gzip.NewReader
	createDir            = realCreateDir
	osCreate             = os.Create
	osOpenFile           = os.OpenFile
	osChmod              = os.Chmod
	osChown              = os.Chown
	osGetwd              = os.Getwd
	ioutilTempDir        = ioutil.TempDir
	ioutilReadFile       = ioutil.ReadFile
	ioutilWriteFile      = ioutil.WriteFile
	osRemoveAll          = os.RemoveAll
	osRemove             = os.Remove
	ioutilTempFile       = ioutil.TempFile
	execCommand          = exec.Command
	osGeteuid            = os.Geteuid
	osLookupEnv          = os.LookupEnv
	filepathWalkDir      = filepath.WalkDir
	yamlMarshalSettings  = realYamlMarshalSettings
	yamlMarshalSecret    = realYamlMarshalSecret
	yamlMarshalConfigMap = realYamlMarshalConfigMap
	configDir            = "$HOME/.karavi/"
	sidecarImageTar      = "sidecar-proxy-"
)

const (
	// RootUID is the UID of the system root user.
	RootUID = 0
	// EnvSudoUID is the common envvar for the user invoking sudo.
	EnvSudoUID = "SUDO_UID"
	// EnvSudoGID is the common envvar for the group invoking sudo.
	EnvSudoGID = "SUDO_GID"
)

// Common errors.
var (
	ErrNeedRoot = errors.New("need to be run as root")
)

// Common Rancher constants, including the required dirs for installing
// k3s and preloading our application.
const (
	RancherImagesDir          = "/var/lib/rancher/k3s/agent/images"
	RancherManifestsDir       = "/var/lib/rancher/k3s/server/manifests"
	RancherK3sKubeConfigPath  = "/etc/rancher/k3s/k3s.yaml"
	EnvK3sInstallSkipDownload = "INSTALL_K3S_SKIP_DOWNLOAD=true"
	EnvK3sForceRestart        = "INSTALL_K3S_FORCE_RESTART=true"
	EnvK3sSkipSelinuxRpm      = "INSTALL_K3S_SKIP_SELINUX_RPM=true"
)

const (
	installedK3s           = "/usr/local/bin/k3s"
	arch                   = "amd64"
	k3SInstallScript       = "k3s-install.sh"
	k3sBinary              = "k3s"
	k3SImagesTar           = "k3s-airgap-images-" + arch + ".tar"
	authImagesTar          = "credential-shield-images.tar"
	authDeploymentManifest = "deployment.yaml"
	authIngressManifest    = "ingress-traefik.yaml"
	authTlsOptionManifest  = "tls-option.yaml"
	certManagerManifest    = "cert-manager.yaml"
	certManagerImagesTar   = "cert-manager-images.tar"
	selfSignedCertManifest = "self-cert.yaml"
	certConfigManifest     = "signed-cert.yaml"
	bundleTarPath          = "dist/karavi-airgap-install.tar.gz"
	karavictl              = "karavictl"

	defaultProxyHostName = "temporary.Host.Name"
	defaultGrpcHostName  = "grpc.tenants.cluster"
	getVersion           = "DOCKER_TAG \\?= ([0-9]+(\\.[0-9]+)+)"
)

func main() {
	// see embed.go / embed_prod.go
	dp := NewDeploymentProcess(os.Stdout, os.Stderr, embedBundleTar)
	dp.cfg = config()

	if err := run(dp); err != nil {
		fmt.Fprintf(os.Stderr, "error: %+v\n", err)
		os.Exit(1)
	}
}

func config() *viper.Viper {
	cfgViper := viper.New()
	cfgViper.SetConfigName("config")
	cfgViper.SetConfigType("json")
	cfgViper.AddConfigPath(".")
	cfgViper.AddConfigPath(configDir)

	err := cfgViper.ReadInConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("Using config file:", cfgViper.ConfigFileUsed())

	return cfgViper
}

func run(dp *DeployProcess) error {
	err := dp.Execute()
	if err != nil {
		return err
	}
	return nil
}

// StepFunc represents a step in the deployment process.
type StepFunc func()

// DeployProcess acts as the process for deploying the application.
// On calling the Execute function, the configured slice of StepFuncs
// will be called in order.
// Following the sticky error pattern, each step should first check
// the Err field to determine if it should continue or return immediately.
type DeployProcess struct {
	Err             error // sticky error.
	cfg             *viper.Viper
	stdout          io.Writer
	stderr          io.Writer
	bundleTar       fs.FS
	tmpDir          string
	processOwnerUID int
	processOwnerGID int
	Steps           []StepFunc
	manifests       []string
}

// NewDeploymentProcess creates a new DeployProcess, pre-configured
// with an list of StepFuncs.
func NewDeploymentProcess(stdout, stderr io.Writer, bundle fs.FS) *DeployProcess {
	dp := &DeployProcess{
		bundleTar: bundle,
		stdout:    stdout,
		stderr:    stderr,
		manifests: []string{authDeploymentManifest, authIngressManifest, authTlsOptionManifest, certManagerManifest},
	}
	dp.Steps = append(dp.Steps,
		dp.CheckRootPermissions,
		dp.CreateTempWorkspace,
		dp.UntarFiles,
		dp.AddCertificate,
		dp.AddHostName,
		dp.InstallKaravictl,
		dp.CreateRancherDirs,
		dp.InstallK3s,
		dp.CopyImagesToRancherDirs,
		dp.CopyManifestsToRancherDirs,
		dp.WriteConfigSecretManifest,
		dp.WriteStorageSecretManifest,
		dp.WriteConfigMapManifest,
		dp.ExecuteK3sInstallScript,
		dp.ChownK3sKubeConfig,
		dp.RemoveSecretManifest,
		dp.CopySidecarProxyToCwd,
		dp.Cleanup,
		dp.PrintFinishedMessage,
	)
	return dp
}

// Execute calls each step in order and returns any
// error encountered.
func (dp *DeployProcess) Execute() error {
	for _, step := range dp.Steps {
		step()
	}
	return dp.Err
}

// CheckRootPermissions checks that the effective user ID who is running the command
// is 0 (root). By default, the k3s KUBECONFIG file will have root permissions. If
// the user is running as sudo, attempt to determine the underlying user and use
// those permissions instead.
func (dp *DeployProcess) CheckRootPermissions() {
	if osGeteuid() != RootUID {
		dp.Err = ErrNeedRoot
		return
	}

	sudoUID, uidOK := osLookupEnv(EnvSudoUID)
	sudoGID, gidOK := osLookupEnv(EnvSudoGID)

	if !uidOK || !gidOK {
		return
	}
	// Both values exist at this point
	uid, err := strconv.Atoi(sudoUID)
	if err != nil {
		// ignore the error
		return
	}
	gid, err := strconv.Atoi(sudoGID)
	if err != nil {
		// ignore the error
		return
	}
	dp.processOwnerUID = uid
	dp.processOwnerGID = gid
}

// CreateTempWorkspace creates a temporary working directory
// to be used as part of deployment.
func (dp *DeployProcess) CreateTempWorkspace() {
	if dp.Err != nil {
		return
	}

	dir, err := ioutilTempDir("", "karavi-installer-*")
	if err != nil {
		dp.Err = fmt.Errorf("creating tmp directory: %w", err)
		return
	}
	dp.tmpDir = dir
}

// ChownK3sKubeConfig changes the ownership of the kubeconfig file
// to the user executing the installer.  If sudo is used, the user
// and group details are determined via the SUDO[UID,GID] environment
// variables.
func (dp *DeployProcess) ChownK3sKubeConfig() {
	if dp.Err != nil {
		return
	}

	if dp.processOwnerUID == RootUID && dp.processOwnerGID == RootUID {
		// nothing to do
		return
	}

	if err := osChown(RancherK3sKubeConfigPath,
		dp.processOwnerUID,
		dp.processOwnerGID); err != nil {
		dp.Err = fmt.Errorf("chown'ing %s to %d:%d: %w",
			RancherK3sKubeConfigPath,
			dp.processOwnerUID,
			dp.processOwnerGID,
			err)
	}
}

// CopySidecarProxyToCwd copies the sidecar proxy image to the
// current working directory
func (dp *DeployProcess) CopySidecarProxyToCwd() {
	if dp.Err != nil {
		return
	}

	fmt.Fprintf(dp.stdout, "Copying the Karavi-Authorization sidecar proxy image locally...")
	defer fmt.Fprintln(dp.stdout, "Done!")

	var sidecarFilePath string

	err := filepathWalkDir(dp.tmpDir, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(dp.stderr, "error: finding sidecar proxy file: %+v ", err)
			return nil
		}

		if !info.IsDir() && filepath.Ext(path) == ".tar" && strings.Contains(path, sidecarImageTar) {
			sidecarFilePath = path
			sidecarImageTar += strings.SplitN(sidecarFilePath, sidecarImageTar, 2)[1]
		}

		return nil
	})

	if err != nil {
		dp.Err = fmt.Errorf("finding sidecar file: %w", err)
		return
	}

	wd, err := osGetwd()
	if err != nil {
		dp.Err = fmt.Errorf("getting working directory: %w", err)
		return
	}
	tgtPath := filepath.Join(wd, sidecarImageTar)
	cmd := execCommand("cp", "-p", "--recursive", sidecarFilePath, tgtPath)
	if err := cmd.Run(); err != nil {
		dp.Err = fmt.Errorf("moving sidecar proxy from %s to %s: %w", sidecarFilePath, tgtPath, err)
		return
	}
}

// Cleanup performs cleanup operations like removing the
// temporary working directory.
func (dp *DeployProcess) Cleanup() {
	if dp.Err != nil {
		return
	}

	if err := osRemoveAll(dp.tmpDir); err != nil {
		fmt.Fprintf(dp.stderr, "error: cleaning up temporary dir: %s", dp.tmpDir)
	}
}

// RemoveSecretManifest removes the karavi-storage-secret.yaml to prevent
// overriding storage system data on k3s restart.
func (dp *DeployProcess) RemoveSecretManifest() {
	if dp.Err != nil {
		return
	}

	fname := filepath.Join(RancherManifestsDir, "karavi-storage-secret.yaml")

	if err := osRemove(fname); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(dp.stderr, "error: cleaning up secret file: %+v\n", err)
		}
	}
}

// UntarFiles extracts the files from the embedded bundle tar file.
func (dp *DeployProcess) UntarFiles() {
	if dp.Err != nil {
		return
	}

	fmt.Fprintf(dp.stdout, "Extracting files...")
	defer fmt.Fprintln(dp.stdout, "Done!")

	gzipFile, err := dp.bundleTar.Open(bundleTarPath)
	if err != nil {
		dp.Err = fmt.Errorf("opening gzip file: %w", err)
		return
	}
	gzr, err := gzipNewReader(gzipFile)
	if err != nil {
		dp.Err = fmt.Errorf("creating gzip reader: %w", err)
		return
	}
	defer func() {
		err := gzr.Close()
		if err != nil {
			dp.Err = fmt.Errorf("closing gzip reader: %w", err)
		}
	}()

	tr := tar.NewReader(gzr)
	// Limit the tar reader to 2 GB incase of decompression bomb
	lr := io.LimitReader(tr, 2000000000)

loop:
	for {
		header, err := tr.Next()

		switch {
		// if no more files are found return
		case err == io.EOF:
			break loop
		// return any other error
		case err != nil:
			dp.Err = err
			return
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// NOTE(ian): What if the tar file contains a directory.
		case tar.TypeReg:
			target, err := sanitizeExtractPath(header.Name, dp.tmpDir)
			if err != nil {
				dp.Err = fmt.Errorf("sanitizing extraction path %s, %w", target, err)
				return
			}

			f, err := os.OpenFile(filepath.Clean(target), os.O_CREATE|os.O_RDWR, os.FileMode(0755))
			if err != nil {
				dp.Err = fmt.Errorf("creating file %q: %w", target, err)
				return
			}

			if _, err := io.Copy(f, lr); err != nil {
				dp.Err = fmt.Errorf("copy contents of %q: %w", target, err)
				return
			}

			if err := f.Close(); err != nil {
				// ignore
			}
		}
	}
}

// InstallKaravictl moves the embedded/extracted karavictl binary
// to /usr/local/bin.
func (dp *DeployProcess) InstallKaravictl() {
	if dp.Err != nil {
		return
	}

	fmt.Fprintf(dp.stdout, "Installing karavictl into /usr/local/bin...")
	defer fmt.Fprintln(dp.stdout, "Done!")

	tmpPath := filepath.Join(dp.tmpDir, karavictl)
	tgtPath := filepath.Join("/usr/local/bin", karavictl)
	cmd := execCommand("cp", "-p", "--recursive", tmpPath, tgtPath)
	if err := cmd.Run(); err != nil {
		dp.Err = fmt.Errorf("installing karavictl: %w", err)
		return
	}
	if err := osChmod(tgtPath, 0755); err != nil {
		dp.Err = fmt.Errorf("chmod karavictl: %w", err)
		return
	}
}

// CreateRancherDirs creates the pre-requisite directories
// for K3s to pick up our application resources that we
// intend for auto-deployment.
func (dp *DeployProcess) CreateRancherDirs() {
	if dp.Err != nil {
		return
	}

	dirsToCreate := []string{
		RancherImagesDir,
		RancherManifestsDir,
	}

	for _, dir := range dirsToCreate {
		err := createDir(dir)
		if err != nil {
			dp.Err = fmt.Errorf("creating directory %s: %w", dir, err)
			return
		}
	}
}

// InstallK3s copies the embedded/extracted k3s binary to /usr/local/bin.
func (dp *DeployProcess) InstallK3s() {
	if dp.Err != nil {
		return
	}

	tmpPath := filepath.Join(dp.tmpDir, k3sBinary)
	tgtPath := filepath.Join("/usr/local/bin", k3sBinary)

	tgtK3s, err := osCreate(tgtPath)
	if err != nil {
		dp.Err = fmt.Errorf("creating /usr/local/bin/k3s: %w", err)
		return
	}
	defer func() {
		err := tgtK3s.Close()
		if err != nil {
			dp.Err = fmt.Errorf("closing /usr/local/bin/k3s: %w", err)
		}
	}()

	tmpK3s, err := osOpenFile(tmpPath, os.O_RDONLY, 0)
	if err != nil {
		dp.Err = fmt.Errorf("opening %s: %w", tmpPath, err)
		return
	}
	defer func() {
		err := tmpK3s.Close()
		if err != nil {
			dp.Err = fmt.Errorf("closing temp k3s: %w", err)
		}
	}()

	_, err = io.Copy(tgtK3s, tmpK3s)
	if err != nil {
		dp.Err = fmt.Errorf("copying %s to %s: %w", tmpPath, tgtPath, err)
		return
	}

	if err := osChmod(tmpPath, 0755); err != nil {
		dp.Err = fmt.Errorf("chmod %s: %w", tmpPath, err)
		return
	}

	if err := osChmod(tgtPath, 0755); err != nil {
		dp.Err = fmt.Errorf("chmod %s: %w", tgtPath, err)
		return
	}
}

// CopyImagesToRancherDirs copies the application images
// to the appropriate K3s dir for auto-populating into
// its internal container registry.
func (dp *DeployProcess) CopyImagesToRancherDirs() {
	if dp.Err != nil {
		return
	}

	images := []string{k3SImagesTar, authImagesTar, certManagerImagesTar}

	for _, image := range images {
		tmpPath := filepath.Join(dp.tmpDir, image)
		tgtPath := filepath.Join(RancherImagesDir, image)
		cmd := execCommand("cp", "-p", "--recursive", tmpPath, tgtPath)
		if err := cmd.Run(); err != nil {
			dp.Err = fmt.Errorf("moving %s to %s: %w", tmpPath, tgtPath, err)
			return
		}
	}
}

// CopyManifestsToRancherDirs copies the application manifests
// to the appropriate K3s dir for auto-applying into the running
// K3s.
func (dp *DeployProcess) CopyManifestsToRancherDirs() {
	if dp.Err != nil {
		return
	}

	for _, man := range dp.manifests {
		tmpPath := filepath.Join(dp.tmpDir, man)
		tgtPath := filepath.Join(RancherManifestsDir, man)
		cmd := execCommand("cp", "-p", "--recursive", tmpPath, tgtPath)
		if err := cmd.Run(); err != nil {
			dp.Err = fmt.Errorf("moving %s to %s: %w", tmpPath, tgtPath, err)
			return
		}
	}
}

// WriteConfigSecretManifest generates and writes the Kubernetes
// Secret manifest for Karavi-Authorization, based on the provided
// configuration options, if any.
func (dp *DeployProcess) WriteConfigSecretManifest() {
	if dp.Err != nil {
		return
	}

	settings := dp.cfg.AllSettings()
	settingsBytes, err := yamlMarshalSettings(&settings)
	if err != nil {
		dp.Err = fmt.Errorf("marshalling %+v: %w", settings, err)
		return
	}

	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "karavi-config-secret",
			Namespace: "karavi",
		},
		Type:       corev1.SecretTypeOpaque,
		StringData: make(map[string]string),
	}
	secret.StringData["config.yaml"] = string(settingsBytes)
	secretBytes, err := yamlMarshalSecret(&secret)
	if err != nil {
		dp.Err = fmt.Errorf("marshalling %+v: %w", secret, err)
		return
	}

	fname := filepath.Join(RancherManifestsDir, "karavi-config-secret.yaml")
	f, err := osOpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0341)
	if err != nil {
		dp.Err = fmt.Errorf("creating %s: %w", fname, err)
		return
	}
	defer func() {
		err := f.Close()
		if err != nil {
			dp.Err = fmt.Errorf("closing RancherManifestsDir: %w", err)
		}
	}()

	_, err = f.Write(secretBytes)
	if err != nil {
		dp.Err = fmt.Errorf("writing secret: %w", err)
		return
	}
}

// WriteStorageSecretManifest generates and writes the Kubernetes
// Storage Secret manifest for Karavi-Authorization, if it does not exist from previous install
func (dp *DeployProcess) WriteStorageSecretManifest() {
	if dp.Err != nil {
		return
	}

	//check if a secret already exists from previous install
	cmd := execCommand("/usr/local/bin/k3s", "kubectl", "get", "secret", "karavi-storage-secret", "-n", "karavi", "-o", "json")
	err := cmd.Run()
	if err == nil {
		//skip creating the secret
		return
	}

	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "karavi-storage-secret",
			Namespace: "karavi",
		},
		Data: make(map[string][]byte),
	}
	b64, err := base64.StdEncoding.DecodeString("c3RvcmFnZToK")
	if err != nil {
		dp.Err = fmt.Errorf("decoding base64 string: %w", err)
		return
	}
	secret.Data["storage-systems.yaml"] = b64
	secretBytes, err := yamlMarshalSecret(&secret)
	if err != nil {
		dp.Err = fmt.Errorf("marshalling %+v: %w", secret, err)
		return
	}

	fname := filepath.Join(RancherManifestsDir, "karavi-storage-secret.yaml")
	f, err := osOpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0341)
	if err != nil {
		dp.Err = fmt.Errorf("creating %s: %w", fname, err)
		return
	}
	defer func() {
		err := f.Close()
		if err != nil {
			dp.Err = fmt.Errorf("closing RancherManifestsDir: %w", err)
		}
	}()

	_, err = f.Write(secretBytes)
	if err != nil {
		dp.Err = fmt.Errorf("writing secret: %w", err)
		return
	}

}

// WriteConfigMapManifest generates and writes the Kubernetes
// Secret manifest for Karavi-Authorization, based on the provided
// configuration options, if any.
func (dp *DeployProcess) WriteConfigMapManifest() {
	if dp.Err != nil {
		return
	}

	settings := dp.cfg.AllSettings()

	logLevel := "debug"
	if proxySettings, ok := settings["proxy"]; ok {
		if proxySettingsMap, ok := proxySettings.(map[string]interface{}); ok {
			if ll, ok := proxySettingsMap["loglevel"]; ok {
				if v, ok := ll.(string); ok {
					logLevel = v
				}
			}
		}
	}

	data := map[string]interface{}{
		"LOG_LEVEL": logLevel,
	}

	configBytes, err := yamlMarshalSettings(&data)
	if err != nil {
		dp.Err = fmt.Errorf("marshalling %+v: %w", data, err)
		return
	}

	cm := corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "csm-config-params",
			Namespace: "karavi",
		},
		Data: make(map[string]string),
	}

	cm.Data["csm-config-params.yaml"] = string(configBytes)
	cmBytes, err := yamlMarshalConfigMap(&cm)
	if err != nil {
		dp.Err = fmt.Errorf("marshalling %+v: %w", cm, err)
		return
	}

	fname := filepath.Join(RancherManifestsDir, "csm-config-params.yaml")
	f, err := osOpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0341)
	if err != nil {
		dp.Err = fmt.Errorf("creating %s: %w", fname, err)
		return
	}
	defer func() {
		err := f.Close()
		if err != nil {
			dp.Err = fmt.Errorf("closing RancherManifestsDir: %w", err)
		}
	}()

	_, err = f.Write(cmBytes)
	if err != nil {
		dp.Err = fmt.Errorf("writing secret: %w", err)
		return
	}
}

func realYamlMarshalSettings(v *map[string]interface{}) ([]byte, error) {
	return k8sYaml.Marshal(v)
}

func realYamlMarshalSecret(v *corev1.Secret) ([]byte, error) {
	return k8sYaml.Marshal(v)
}

func realYamlMarshalConfigMap(v *corev1.ConfigMap) ([]byte, error) {
	return k8sYaml.Marshal(v)
}

// ExecuteK3sInstallScript executes the K3s install script.
// A log file of the stdout/stderr output is saved into a
// temporary file to help troubleshoot if an error occurs.
func (dp *DeployProcess) ExecuteK3sInstallScript() {
	if dp.Err != nil {
		return
	}

	fmt.Fprintf(dp.stdout, "Installing Karavi-Authorization...")
	defer fmt.Fprintln(dp.stdout, "Done!")

	tmpPath := filepath.Join(dp.tmpDir, k3SInstallScript)
	if err := osChmod(tmpPath, 0755); err != nil {
		dp.Err = fmt.Errorf("chmod %s: %w", k3SInstallScript, err)
		return
	}

	logFile, err := ioutilTempFile("", "k3s-install-for-karavi")
	if err != nil {
		dp.Err = fmt.Errorf("creating k3s install logfile: %w", err)
		return
	}

	cmd := execCommand(filepath.Join(dp.tmpDir, k3SInstallScript))
	cmd.Env = append(os.Environ(), EnvK3sInstallSkipDownload, EnvK3sForceRestart, EnvK3sSkipSelinuxRpm)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	err = cmd.Run()
	if err != nil {
		dp.Err = fmt.Errorf("failed to install k3s (see %s): %w", logFile.Name(), err)
		return
	}
}

// PrintFinishedMessage prints the completion messages on the console
func (dp *DeployProcess) PrintFinishedMessage() {
	if dp.Err != nil {
		return
	}

	fmt.Fprintln(dp.stdout)
	fmt.Fprintln(dp.stdout, "Check cluster status with karavictl cluster-info --watch")
	fmt.Fprintf(dp.stdout, "The sidecar container image has been saved at %q.\n", sidecarImageTar)
	fmt.Fprintln(dp.stdout, "Please push this image to a container registry accessible to tenant Kubernetes clusters.")
}

func realCreateDir(newDir string) error {
	// TODO(alik): Do we need to check these errors?
	// if dir is not exist create it
	if _, err := os.Stat(filepath.Clean(newDir)); err != nil {
		if err := os.MkdirAll(newDir, 0750); err != nil {
			return err
		}
	}

	return nil
}

// AddCertificate adds the certificate manifest
func (dp *DeployProcess) AddCertificate() {
	if dp.Err != nil {
		return
	}

	if !dp.cfg.IsSet("certificate") {
		//no certificate found, create self-signed certificate
		dp.manifests = append(dp.manifests, selfSignedCertManifest)
		return
	}
	certData := dp.cfg.GetStringMapString("certificate")
	var crtFile, keyFile string
	encodedCerts := make(map[string]string)
	if len(certData) < 3 {
		dp.Err = fmt.Errorf("missing certificate files")
		return
	}
	for k, v := range certData {
		switch {
		case k == "crtfile":
			crtFile = v
		case k == "keyfile":
			keyFile = v
		case k == "rootcertificate":
			continue
		default:
			dp.Err = fmt.Errorf("unknown certificate file format %s", k)
			return
		}
		content, err := ioutilReadFile(v)
		if err != nil {
			dp.Err = fmt.Errorf("failed to read file %s: %w", v, err)
			return
		}
		// Encode as base64.
		encodedCerts[k] = base64.StdEncoding.EncodeToString(content)
	}
	fmt.Fprintf(dp.stdout, "Provided Crtfile %s, KeyFile %s\n", crtFile, keyFile)

	//replace cert info in manifest file
	certFile := filepath.Join(dp.tmpDir, certConfigManifest)

	read, err := ioutilReadFile(certFile)
	if err != nil {
		dp.Err = fmt.Errorf("failed to read cert manifest file: %w", err)
		return
	}

	newContents := strings.Replace(string(read), "crtFile", encodedCerts["crtfile"], -1)
	newContents = strings.Replace(newContents, "keyFile", encodedCerts["keyfile"], -1)

	err = ioutilWriteFile(certFile, []byte(newContents), 0)
	if err != nil {
		dp.Err = fmt.Errorf("failed to write to cert manifest file: %w", err)
		return
	}
	dp.manifests = append(dp.manifests, certConfigManifest)

}

// AddHostName replaces the ingress hostname in the manifest
func (dp *DeployProcess) AddHostName() {
	if dp.Err != nil {
		return
	}

	if !dp.cfg.IsSet("hostname") {
		dp.Err = fmt.Errorf("missing hostname configuration")
		return
	}

	hostName := dp.cfg.GetString("hostname")

	//update hostnames in ingress manifest
	ingressFile := filepath.Join(dp.tmpDir, authIngressManifest)

	read, err := ioutilReadFile(ingressFile)
	if err != nil {
		dp.Err = fmt.Errorf("failed to read ingress manifest file: %w", err)
		return
	}

	newContents := strings.Replace(string(read), defaultProxyHostName, hostName, -1)
	newContents = strings.Replace(newContents, defaultGrpcHostName, "grpc."+hostName, -1)

	err = ioutilWriteFile(ingressFile, []byte(newContents), 0)
	if err != nil {
		dp.Err = fmt.Errorf("failed to write to ingress manifest file: %w", err)
		return
	}
}

func sanitizeExtractPath(filePath string, destination string) (string, error) {
	destpath := filepath.Join(destination, filePath)
	if !strings.HasPrefix(destpath, filepath.Clean(destination)+string(os.PathSeparator)) {
		return "", fmt.Errorf("illegal file path: %s", filePath)
	}
	return destpath, nil
}
