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
