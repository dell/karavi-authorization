#!/bin/bash

ARCH=amd64

K3S_INSTALL_SCRIPT=k3s-install.sh
K3S_BINARY=k3s
K3S_IMAGES_TAR=k3s-airgap-images-$ARCH.tar

CRED_SHIELD_IMAGES_TAR=credential-shield-images.tar
CRED_SHIELD_DEPLOYMENT_MANIFEST=deployment.yaml
CRED_SHIELD_INGRESS_MANIFEST=ingress-traefik.yaml

INSTALL_SCRIPT=install.sh

tar xfv airgap-install.tar

# Copy over the binary and make executable
sudo cp ./$K3S_BINARY /usr/local/bin/k3s
sudo chmod 755 /usr/local/bin/k3s

# Create the directory for loading images.
sudo mkdir -p /var/lib/rancher/k3s/agent/images
# Copy over the images
sudo cp ./$K3S_IMAGES_TAR ./$CRED_SHIELD_IMAGES_TAR /var/lib/rancher/k3s/agent/images/.

# Create the directory for automated manifest deployments.
sudo mkdir -p /var/lib/rancher/k3s/server/manifests
# Copy over the manifests
sudo cp $CRED_SHIELD_DEPLOYMENT_MANIFEST $CRED_SHIELD_INGRESS_MANIFEST /var/lib/rancher/k3s/server/manifests/.

# Run the K3s install script.
INSTALL_K3S_SKIP_DOWNLOAD=true ./$K3S_INSTALL_SCRIPT
