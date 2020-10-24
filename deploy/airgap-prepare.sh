#!/bin/bash

ARCH=amd64

K3S_INSTALL_SCRIPT=k3s-install.sh
K3S_BINARY=k3s
K3S_IMAGES_TAR=k3s-airgap-images-$ARCH.tar

CRED_SHIELD_IMAGES_TAR=credential-shield-images.tar
CRED_SHIELD_DEPLOYMENT_MANIFEST=deployment.yaml
CRED_SHIELD_INGRESS_MANIFEST=ingress-traefik.yaml

INSTALL_SCRIPT=install.sh


# Download install script
curl -kL -o $K3S_INSTALL_SCRIPT https://get.k3s.io/

# Download k3s
curl -kL -o $K3S_BINARY  https://github.com/rancher/k3s/releases/download/v1.18.10%2Bk3s1/k3s

# Download k3s images
curl -kL -o $K3S_IMAGES_TAR https://github.com/rancher/k3s/releases/download/v1.18.10%2Bk3s1/k3s-airgap-images-$ARCH.tar

# Save all referenced images into a tarball
grep "image: " deployment.yaml | awk -F' ' '{ print $2 }' | xargs docker save -o $CRED_SHIELD_IMAGES_TAR

tar cfv airgap-install.tar \
	$K3S_INSTALL_SCRIPT \
	$K3S_BINARY \
	$K3S_IMAGES_TAR \
	$CRED_SHIELD_IMAGES_TAR \
	$CRED_SHIELD_DEPLOYMENT_MANIFEST \
	$CRED_SHIELD_INGRESS_MANIFEST \
  $INSTALL_SCRIPT

exit 0

#sudo cp ./k3s /usr/local/bin/.
#sudo chmod 755 /usr/local/bin/k3s

#sudo mkdir -p /var/lib/rancher/k3s/agent/images
#sudo cp ./k3s-airgap-images-amd64.tar /var/lib/rancher/k3s/agent/images/.

# Create the directory for loading images.
sudo mkdir -p /var/lib/rancher/k3s/agent/images/
# Copy over the images
sudo cp ./k3s-airgap-images-amd64.tar /var/lib/rancher/k3s/agent/images/.
sudo cp ./credential-shield-images.tar /var/lib/rancher/k3s/agent/images/.

# Create the directory for automated manifest deployments.
sudo mkdir -p /var/lib/rancher/k3s/server/manifests
# Copy over the manifests
sudo cp ./karavi-security/deploy/deployment.yaml /var/lib/rancher/k3s/server/manifests/.
sudo cp ./karavi-security/deploy/ingress-traefik.yaml /var/lib/rancher/k3s/server/manifests/.

sudo cp k3s /usr/local/bin/.
sudo chmod 755 /usr/local/bin/k3s

INSTALL_K3S_SKIP_DOWNLOAD=true ./install.sh

