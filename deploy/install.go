package main

import (
	"archive/tar"
	"compress/gzip"
	"embed"
	"io"
	"io/fs"
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
	bundleTar                    = "karavi-airgap-install.tar.gz"
)

var (
	//go:embed "karavi-airgap-install.tar.gz"
	embedBundleTar embed.FS
)

func main() {
	// read files from embed.FS
	gzipFile, _ := embedBundleTar.Open("karavi-airgap-install.tar.gz")
	unTarFiles(gzipFile)

}

func unTarFiles(r fs.File) error {

	gzr, err := gzip.NewReader(r)
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
		var target string

		// check the file type
		switch header.Typeflag {

		// if its a dir and it doesn't exist create it
		case tar.TypeDir:
			if _, err := os.Stat(target); err != nil {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
			}

		// if it's a file create it
		case tar.TypeReg:
			if header.Name == k3SBinary {
				target = filepath.Join("/usr/local/bin/", header.Name)
			}

			if header.Name == k3SImagesTar || header.Name == credShieldImagesTar {
				// check dir already created
				if _, err := os.Stat("/var/lib/rancher/k3s/agent/images"); err != nil {
					// dir do not exist create it
					if err := os.MkdirAll("/var/lib/rancher/k3s/agent/images", 0755); err != nil {
						return err
					}
				}
				target = filepath.Join("/var/lib/rancher/k3s/agent/images", header.Name)
			}

			if header.Name == credShieldDeploymentManifest || header.Name == credShieldIngressManifest {
				// check dir already created
				if _, err := os.Stat("/var/lib/rancher/k3s/server/manifests"); err != nil {
					// dir do not exist create it
					if err := os.MkdirAll("/var/lib/rancher/k3s/server/manifests", 0755); err != nil {
						return err
					}
				}
				target = filepath.Join("/var/lib/rancher/k3s/server/manifests", header.Name)
			}

			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR, os.FileMode(755))
			if err != nil {
				return err
			}

			// copy over contents
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}

			// manually close here after each file operation; defering would cause each file close
			// to wait until all operations have completed.
			f.Close()
		}
	}
}
