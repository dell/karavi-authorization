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

// Command deploy is used to install the application using embedded
// resources.
package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
)

// Overrides for testing purposes.
var (
	gzipNewReader = gzip.NewReader
	createDir     = realCreateDir
	osRename      = os.Rename
	osChmod       = os.Chmod
	ioutilTempDir = ioutil.TempDir
	osRemoveAll   = os.RemoveAll
)

// Common Rancher constants, including the required dirs for installing
// k3s and preloading our application.
const (
	RancherImagesDir          = "/var/lib/rancher/k3s/agent/images"
	RancherManifestsDir       = "/var/lib/rancher/k3s/server/manifests"
	EnvK3sInstallSkipDownload = "INSTALL_K3S_SKIP_DOWNLOAD=true"
)

const (
	arch                         = "amd64"
	k3SInstallScript             = "k3s-install.sh"
	k3sBinary                    = "k3s"
	k3SImagesTar                 = "k3s-airgap-images-" + arch + ".tar"
	credShieldImagesTar          = "credential-shield-images.tar"
	credShieldDeploymentManifest = "deployment.yaml"
	credShieldIngressManifest    = "ingress-traefik.yaml"
	bundleTarPath                = "dist/karavi-airgap-install.tar.gz"
	karavictl                    = "karavictl"
	sidecarImageTar              = "sidecar-proxy-latest.tar"
	sidecarDockerImage           = "sidecar-proxy:latest"
)

func main() {
	dp := NewDeploymentProcess()
	// TODO(ian): Configure these with functional options instead.
	dp.stdout = os.Stdout
	dp.stderr = os.Stderr
	dp.bundleTar = embedBundleTar // see embed.go / embed_prod.go

	if err := run(dp); err != nil {
		fmt.Fprintf(os.Stderr, "error: %+v", err)
		os.Exit(1)
	}
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
	Err       error // sticky error.
	stdout    io.Writer
	stderr    io.Writer
	bundleTar fs.FS
	tmpDir    string
	Steps     []StepFunc
}

// NewDeploymentProcess creates a new DeployProcess, pre-configured
// with an list of StepFuncs.
func NewDeploymentProcess() *DeployProcess {
	dp := &DeployProcess{
		bundleTar: embedBundleTar,
	}
	dp.Steps = append(dp.Steps,
		dp.CreateTempWorkspace,
		dp.UntarFiles,
		dp.InstallKaravictl,
		dp.CreateRancherDirs,
		dp.InstallK3s,
		dp.CopyImagesToRancherDirs,
		dp.CopyManifestsToRancherDirs,
		dp.ExecuteK3sInstallScript,
		dp.InitKaraviPolicies,
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

// CreateTempWorkspace creates a temporary working directory
// to be used as part of deployment.
func (dp *DeployProcess) CreateTempWorkspace() {
	dir, err := ioutilTempDir("", "karavi-installer-*")
	if err != nil {
		dp.Err = fmt.Errorf("creating tmp directory: %w", err)
		return
	}
	dp.tmpDir = dir
}

func (dp *DeployProcess) CopySidecarProxyToCwd() {
	if dp.Err != nil {
		return
	}

	fmt.Fprintf(dp.stdout, "Copying the Karavi-Authorization sidecar proxy image locally...")
	defer fmt.Fprintln(dp.stdout, "Done!")

	tmpPath := filepath.Join(dp.tmpDir, sidecarImageTar)
	wd, err := os.Getwd()
	if err != nil {
		dp.Err = fmt.Errorf("getting working directory: %w", err)
		return
	}
	tgtPath := filepath.Join(wd, sidecarImageTar)
	if err := os.Rename(tmpPath, tgtPath); err != nil {
		dp.Err = fmt.Errorf("moving sidecar proxy from %s to %s: %w", tmpPath, tgtPath, err)
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
	defer gzr.Close()

	tr := tar.NewReader(gzr)

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
			target := filepath.Join(dp.tmpDir, header.Name)

			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(755))
			if err != nil {
				dp.Err = fmt.Errorf("creating file %q: %w", target, err)
				return
			}

			if _, err := io.Copy(f, tr); err != nil {
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
	if err := osRename(tmpPath, tgtPath); err != nil {
		dp.Err = fmt.Errorf("installing karavictl: %w", err)
		return
	}
	if err := osChmod(tgtPath, 755); err != nil {
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
		createDir(dir)
	}
}

// InstallK3s moves the embedded/extracted k3s binary to /usr/local/bin.
func (dp *DeployProcess) InstallK3s() {
	if dp.Err != nil {
		return
	}

	tmpPath := filepath.Join(dp.tmpDir, k3sBinary)
	tgtPath := filepath.Join("/usr/local/bin", k3sBinary)
	if err := osRename(tmpPath, tgtPath); err != nil {
		dp.Err = fmt.Errorf("moving k3s binary: %w", err)
		return
	}
	if err := osChmod(tgtPath, 755); err != nil {
		dp.Err = fmt.Errorf("chmod k3s: %w", err)
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

	images := []string{k3SImagesTar, credShieldImagesTar}

	for _, image := range images {
		tmpPath := filepath.Join(dp.tmpDir, image)
		tgtPath := filepath.Join(RancherImagesDir, image)
		if err := osRename(tmpPath, tgtPath); err != nil {
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

	mans := []string{credShieldDeploymentManifest, credShieldIngressManifest}

	for _, man := range mans {
		tmpPath := filepath.Join(dp.tmpDir, man)
		tgtPath := filepath.Join(RancherManifestsDir, man)
		if err := osRename(tmpPath, tgtPath); err != nil {
			dp.Err = fmt.Errorf("moving %s to %s: %w", tmpPath, tgtPath, err)
			return
		}
	}
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
	if err := os.Chmod(tmpPath, 755); err != nil {
		dp.Err = fmt.Errorf("chmod %s: %w", k3SInstallScript, err)
		return
	}

	logFile, err := ioutil.TempFile("", "k3s-install-for-karavi")
	if err != nil {
		dp.Err = fmt.Errorf("creating k3s install logfile: %w", err)
		return
	}

	cmd := exec.Command(filepath.Join(dp.tmpDir, k3SInstallScript))
	cmd.Env = append(os.Environ(), EnvK3sInstallSkipDownload)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	err = cmd.Run()
	if err != nil {
		dp.Err = fmt.Errorf("failed to install k3s (see %s): %w", logFile.Name(), err)
		return
	}
}

// InitKaraviPolicies initializes the application with a set of
// default policies.
func (dp *DeployProcess) InitKaraviPolicies() {
	if dp.Err != nil {
		return
	}

	logFile, err := ioutil.TempFile("", "policy-install-for-karavi")
	if err != nil {
		dp.Err = fmt.Errorf("creating k3s install logfile: %w", err)
		return
	}

	cmd := exec.Command(filepath.Join(dp.tmpDir, "policy-install.sh"))
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	err = cmd.Run()
	if err != nil {
		dp.Err = fmt.Errorf("failed to install policies (see %s): %w", logFile.Name(), err)
		return
	}
}

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
		if err := os.MkdirAll(newDir, 0755); err != nil {
			return err
		}
	}

	return nil
}
