package main

import (
	"archive/tar"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	bundleTar                    = "karavi-airgap-install.tar.gz"
)

var (
	//go:embed "karavi-airgap-install.tar.gz"
	embedBundleTar embed.FS
)

func main() {
	err := unTarFiles()
	if err != nil {
		fmt.Println(err.Error())
	}

	// create required directories for k3s
	err = createDir("/var/lib/rancher/k3s/agent/images")
	if err != nil {
		fmt.Println(err.Error())
	}
	err = createDir("/var/lib/rancher/k3s/server/manifests")
	if err != nil {
		fmt.Println(err.Error())
	}

	// create docker registry volume directory
	err = createDir("/opt/registry")
	if err != nil {
		fmt.Println(err.Error())
	}

	// copy k3s binary file
	err = os.Rename(k3SBinary, "/usr/local/bin/k3s")
	if err != nil {
		fmt.Println(err.Error())
	}
	err = os.Chmod("/usr/local/bin/k3s", 755)
	if err != nil {
		fmt.Println(err.Error())
	}

	// copy images
	err = os.Rename(k3SImagesTar, "/var/lib/rancher/k3s/agent/images/"+k3SImagesTar)
	if err != nil {
		fmt.Println(err.Error())
	}
	err = os.Rename(credShieldImagesTar, "/var/lib/rancher/k3s/agent/images/"+credShieldImagesTar)
	if err != nil {
		fmt.Println(err.Error())
	}

	// copy manifest files
	err = os.Rename(credShieldDeploymentManifest, "/var/lib/rancher/k3s/server/manifests/"+credShieldDeploymentManifest)
	if err != nil {
		fmt.Println(err.Error())
	}
	err = os.Rename(credShieldIngressManifest, "/var/lib/rancher/k3s/server/manifests/"+credShieldIngressManifest)
	if err != nil {
		fmt.Println(err.Error())
	}

	err = copyFile(dockerRegistryManifest, "/var/lib/rancher/k3s/server/manifests/"+dockerRegistryManifest)
	if err != nil {
		fmt.Println(err.Error())
	}

	err = os.Chmod(k3SInstallScript, 755)
	if err != nil {
		fmt.Println(err.Error())
	}

	//execute installation scripts
	cmd := exec.Command("./" + k3SInstallScript)
	err = cmd.Run()
	if err != nil {
		fmt.Println(err.Error())
	}
	//execute policy install scripts
	cmd = exec.Command("./policy-install.sh")
	err = cmd.Run()
	if err != nil {
		fmt.Println(err.Error())
	}
}

func unTarFiles() error {
	gzipFile, err := embedBundleTar.Open("karavi-airgap-install.tar.gz")
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

func copyFile(srcFile, destFile string) error {
	sourceFile, err := os.Open(srcFile)
	if err != nil {
		fmt.Println(err)
	}
	defer sourceFile.Close()

	// Create new file
	newFile, err := os.Create(destFile)
	if err != nil {
		fmt.Println(err)
	}
	defer newFile.Close()

	bytesCopied, err := io.Copy(newFile, sourceFile)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println("Copied %d bytes.", bytesCopied)
	return nil
}

func createDir(newDir string) error {
	// if dir is not exist create it
	if _, err := os.Stat(newDir); err != nil {
		if err := os.MkdirAll(newDir, 0755); err != nil {
			return err
		}
	}

	return nil
}
