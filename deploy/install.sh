#!/bin/bash
cd "$(dirname "$0")"

ARCH=amd64

K3S_INSTALL_SCRIPT=k3s-install.sh
K3S_BINARY=k3s
K3S_IMAGES_TAR=k3s-airgap-images-$ARCH.tar

CRED_SHIELD_IMAGES_TAR=credential-shield-images.tar
CRED_SHIELD_DEPLOYMENT_MANIFEST=deployment.yaml
CRED_SHIELD_INGRESS_MANIFEST=ingress-traefik.yaml

BUNDLE_TAR=karavi-airgap-install.tar.gz
INSTALL_SCRIPT=install.sh

echo -n "Extracting files..."
tar xf $BUNDLE_TAR
echo "Done!"

# Copy over the binary and make executable
cp ./$K3S_BINARY /usr/local/bin/k3s
chmod 755 /usr/local/bin/k3s

# Create the directory for loading images.
mkdir -p /var/lib/rancher/k3s/agent/images
# Copy over the images
cp ./$K3S_IMAGES_TAR ./$CRED_SHIELD_IMAGES_TAR /var/lib/rancher/k3s/agent/images/.

# Create the directory for automated manifest deployments.
mkdir -p /var/lib/rancher/k3s/server/manifests
# Copy over the manifests
cp $CRED_SHIELD_DEPLOYMENT_MANIFEST $CRED_SHIELD_INGRESS_MANIFEST /var/lib/rancher/k3s/server/manifests/.

chmod 755 ./$K3S_INSTALL_SCRIPT
echo -n "Installing Karavi..."
# Run the K3s install script.
INSTALL_K3S_SKIP_DOWNLOAD=true ./$K3S_INSTALL_SCRIPT > /tmp/k3s-install.log 2>&1
echo "Done!"

echo -n Configuring policies...
./policy-install.sh >> /tmp/k3s-install.log 2>&1 
echo "Done!"

echo
echo Check cluster status with 'karavictl cluster-info'
