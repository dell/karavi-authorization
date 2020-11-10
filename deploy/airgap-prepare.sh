#!/bin/bash

ARCH=amd64

DIST=dist

K3S_INSTALL_SCRIPT=${DIST}/k3s-install.sh
K3S_BINARY=${DIST}/k3s
K3S_IMAGES_TAR=${DIST}/k3s-airgap-images-$ARCH.tar

CRED_SHIELD_IMAGES_TAR=${DIST}/credential-shield-images.tar
CRED_SHIELD_DEPLOYMENT_MANIFEST=deployment.yaml
CRED_SHIELD_INGRESS_MANIFEST=ingress-traefik.yaml

INSTALL_SCRIPT=install.sh

# Create the dist directory, if not already present.
mkdir -p dist

# Download install script
if [[ ! -f $K3S_INSTALL_SCRIPT ]]
then
	curl -kL -o $K3S_INSTALL_SCRIPT https://get.k3s.io/
fi

# Download k3s
if [[ ! -f $K3S_BINARY ]]
then
	curl -kL -o $K3S_BINARY  https://github.com/rancher/k3s/releases/download/v1.18.10%2Bk3s1/k3s
fi

if [[ ! -f $K3S_IMAGES_TAR ]]
then
	# Download k3s images
	curl -kL -o $K3S_IMAGES_TAR https://github.com/rancher/k3s/releases/download/v1.18.10%2Bk3s1/k3s-airgap-images-$ARCH.tar
fi

# Save all referenced images into a tarball
grep "image: " deployment.yaml | awk -F' ' '{ print $2 }' | xargs docker save -o $CRED_SHIELD_IMAGES_TAR

tar cfv ${DIST}/airgap-install.tar \
	$K3S_INSTALL_SCRIPT \
	$K3S_BINARY \
	$K3S_IMAGES_TAR \
	$CRED_SHIELD_IMAGES_TAR \
	$CRED_SHIELD_DEPLOYMENT_MANIFEST \
	$CRED_SHIELD_INGRESS_MANIFEST

cp install.sh dist/install.sh
