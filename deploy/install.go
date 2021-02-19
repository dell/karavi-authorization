package main

import (
	"archive/tar"
	"compress/gzip"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
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
	karaviCtl                    = "karavictl"
	registryImageTar             = "registry-image.tar"
	registryService              = "docker-registry-service"
	sidecarImageTar              = "sidecar-proxy-latest.tar"
	sidecarDockerImage           = "sidecar-proxy:latest"
)

var (
	//go:embed "dist/karavi-airgap-install.tar.gz"
	embedBundleTar embed.FS
)

func main() {
	err := unTarFiles()
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// create required directories for k3s
	err = createDir("/var/lib/rancher/k3s/agent/images")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	err = createDir("/var/lib/rancher/k3s/server/manifests")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// create docker registry volume directory
	err = createDir("/opt/registry")
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

	// copy images
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

	err = os.Rename(dockerRegistryManifest, "/var/lib/rancher/k3s/server/manifests/"+dockerRegistryManifest)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

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

	fmt.Println("\nwaiting for pods to come up...")
	time.Sleep(1 * time.Minute)

	// Wait for Pods in karavi namespace to be Ready
	_, err = exec.Command("/bin/sh", "-c", "kubectl wait --for=condition=ready --timeout=5m -n karavi --all pods").CombinedOutput()

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	registryIP, err := exec.Command("/bin/sh", "-c", fmt.Sprintf("kubectl get svc %s -n karavi --template '{{.spec.clusterIP}}'", registryService)).CombinedOutput()

	if err != nil {
		fmt.Println(err.Error())
		return
	}
	if len(registryIP) == 0 {
		fmt.Println("Could not find the docker registry IP")
		return
	}

	// create docker daemon.json
	file, err := os.OpenFile("/etc/docker/daemon.json", os.O_CREATE|os.O_WRONLY, os.ModeAppend)
	defer file.Close()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	encoder := json.NewEncoder(file)
	data := map[string]interface{}{
		"insecure-registries": []string{fmt.Sprintf("%s:5000", registryIP)},
	}
	err = encoder.Encode(data)
	if err != nil {
		fmt.Println(err.Error())
	}

	// restart docker
	cmd = exec.Command("/bin/sh", "-c", "sudo systemctl restart docker")
	err = cmd.Start()
	if err != nil {
		fmt.Println(err.Error())
	}
	cmd.Wait()

	// load sidecar-proxy image
	output, err := exec.Command("/bin/sh", "-c", fmt.Sprintf("docker load --input %s", sidecarImageTar)).Output()
	if err != nil {
		fmt.Println(err.Error())
	}
	fmt.Println(string(output))

	// tag & push sidecar proxy
	output, err = exec.Command("/bin/sh", "-c", fmt.Sprintf("docker tag %s %s:5000/%s", sidecarDockerImage, registryIP, sidecarDockerImage)).Output()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println(string(output))

	output, err = exec.Command("/bin/sh", "-c", fmt.Sprintf("docker push %s:5000/%s", registryIP, sidecarDockerImage)).Output()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println(string(output))
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

func createDir(newDir string) error {
	// if dir is not exist create it
	if _, err := os.Stat(newDir); err != nil {
		if err := os.MkdirAll(newDir, 0755); err != nil {
			return err
		}
	}

	return nil
}
