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
)

const (
	arch                         = "amd64"
	k3SInstallScript             = "k3s-install.sh"
	k3SBinary                    = "k3s"
	k3SImagesTar                 = "k3s-airgap-images-" + arch + ".tar"
	credShieldImagesTar          = "credential-shield-images.tar"
	credShieldDeploymentManifest = "deployment.yaml"
	credShieldIngressManifest    = "ingress-traefik.yaml"
	dockerRegistryManifest       = "registry.yaml"
	bundleTarPath                = "dist/karavi-airgap-install.tar.gz"
	karaviCtl                    = "karavictl"
	registryImageTar             = "registry-image.tar"
	registryService              = "docker-registry-service"
	sidecarImageTar              = "sidecar-proxy-latest.tar"
	sidecarDockerImage           = "sidecar-proxy:latest"
)

func main() {
	dp := &DeployProcess{
		stdout:    os.Stdout,
		stderr:    os.Stderr,
		bundleTar: embedBundleTar, // see embed.go / embed_prod.go
	}
	if err := run(dp); err != nil {
		fmt.Fprintf(os.Stderr, "error: %+v", err)
		os.Exit(1)
	}
}

type DeployProcess struct {
	stdout io.Writer
	stderr io.Writer

	bundleTar fs.FS
	tmpDir    string

	CreateTempWorkspaceFunc func()
	UntarFilesFunc          func()

	Err error // sticky error
}

func NewDeploymentProcess() *DeployProcess {
	dp := &DeployProcess{
		bundleTar: embedBundleTar,
	}
	dp.CreateTempWorkspaceFunc = dp.createTempWorkspace
	dp.UntarFilesFunc = dp.UntarFiles
	return dp
}

func run(dp *DeployProcess) error {
	err := dp.Execute()
	if err != nil {
		return err
	}
	return nil
}

// Executes runs the installation steps in a specific order.
func (dp *DeployProcess) Execute() error {
	dp.CreateTempWorkspaceFunc()
	defer dp.Cleanup()
	dp.UntarFilesFunc()
	dp.CreateRequiredDirsForK3s()
	return dp.Err
}

func (dp *DeployProcess) createTempWorkspace() {
	dir, err := ioutil.TempDir("", "karavi-installer-*")
	if err != nil {
		dp.Err = err
		return
	}
	dp.tmpDir = dir
}

func (dp *DeployProcess) Cleanup() {
	if err := os.RemoveAll(dp.tmpDir); err != nil {
		fmt.Fprintf(dp.stderr, "error: cleaning up temporary dir: %s", dp.tmpDir)
	}
}

func (dp *DeployProcess) UntarFiles() {
	if dp.Err != nil {
		return
	}

	fmt.Fprintf(dp.stdout, "Extracting files...")

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
	fmt.Fprintln(dp.stdout, "Done")
}

func (dp *DeployProcess) CreateRequiredDirsForK3s() {
	if dp.Err != nil {
		return
	}

	dirsToCreate := []string{
		"/var/lib/rancher/k3s/agent/images",
		"/var/lib/rancher/k3s/server/manifests",
	}

	for _, dir := range dirsToCreate {
		createDir(dir)
	}
}

func runold() {
	err := unTarFiles()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// copy k3s binary to local/bin
	err = os.Rename(k3SBinary, "/usr/local/bin/k3s")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	err = os.Chmod("/usr/local/bin/k3s", 755)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// TODO CopyK3sBinaryToSystem
	// copy karavictl file to local/bin
	err = os.Rename(karaviCtl, "/usr/local/bin/karavictl")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	err = os.Chmod("/usr/local/bin/karavictl", 755)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// CopyImagesToRancherImages
	err = os.Rename(k3SImagesTar, "/var/lib/rancher/k3s/agent/images/"+k3SImagesTar)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	err = os.Rename(credShieldImagesTar, "/var/lib/rancher/k3s/agent/images/"+credShieldImagesTar)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	err = os.Rename(registryImageTar, "/var/lib/rancher/k3s/agent/images/"+registryImageTar)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// copy manifest files
	err = os.Rename(credShieldDeploymentManifest, "/var/lib/rancher/k3s/server/manifests/"+credShieldDeploymentManifest)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	err = os.Rename(credShieldIngressManifest, "/var/lib/rancher/k3s/server/manifests/"+credShieldIngressManifest)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// Run K3s install
	err = os.Chmod(k3SInstallScript, 755)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	//execute installation scripts
	fmt.Println("\nInstalling K3S cluster")
	cmd := exec.Command("./" + k3SInstallScript)
	cmd.Stdout = os.Stdout

	err = cmd.Start()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	cmd.Wait()

	//execute policy install scripts
	fmt.Println("\nCreating Policies")
	cmd = exec.Command("./policy-install.sh")
	cmd.Stdout = os.Stdout

	err = cmd.Start()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	cmd.Wait()
}

func unTarFiles() error {
	gzipFile, err := embedBundleTar.Open("dist/karavi-airgap-install.tar.gz")
	if err != nil {
		fmt.Println("Cant gunzip embedded file: ", err.Error())
		return err
	}
	gzr, err := gzip.NewReader(gzipFile)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {

		// if no more files are found return
		case err == io.EOF:
			return nil

		// return any other error
		case err != nil:
			return err

		// if the header is nil, just skip it (not sure how this happens)
		case header == nil:
			continue
		}

		// the target location where the dir/file should be created
		target := "."

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:

		// if it's a file create it
		case tar.TypeReg:
			fmt.Println(header.Name)
			target = filepath.Join(target, header.Name)

			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(755))
			if err != nil {
				fmt.Println("open: ", header.Name)
				return err
			}

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

			// manually close here after each file operation
			f.Close()
		}
	}
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
