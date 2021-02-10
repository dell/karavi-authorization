#!/bin/bash -x

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

# Create the bundle airgap tarfile.
cp $CRED_SHIELD_DEPLOYMENT_MANIFEST $CRED_SHIELD_INGRESS_MANIFEST $DIST/.
cp ../policies/*.rego ../policies/policy-install.sh $DIST/.
tar -czv -C $DIST -f karavi-airgap-install.tar.gz .

# Clean up the files that were just added to the bundle.
rm $K3S_INSTALL_SCRIPT \
	$K3S_BINARY \
	$K3S_IMAGES_TAR \
	$CRED_SHIELD_IMAGES_TAR \
  ${DIST}/$CRED_SHIELD_DEPLOYMENT_MANIFEST \
	${DIST}/$CRED_SHIELD_INGRESS_MANIFEST \
	${DIST}/*.rego \
	${DIST}/policy-install.sh
# Move the two main install files into place.
mv karavi-airgap-install.tar.gz $DIST/.
cp install.sh dist/install.sh
