#!/bin/bash -x

ARCH=amd64
SIDECAR_DOCKER_TAG=1.4.0
DIST=dist

K3S_INSTALL_SCRIPT=${DIST}/k3s-install.sh
K3S_BINARY=${DIST}/k3s
K3S_IMAGES_TAR=${DIST}/k3s-airgap-images-$ARCH.tar

CERT_MANAGER_IMAGES_TAR=${DIST}/cert-manager-images.tar
CRED_SHIELD_IMAGES_TAR=${DIST}/credential-shield-images.tar
CRED_SHIELD_DEPLOYMENT_MANIFEST=deployment.yaml
CRED_SHIELD_INGRESS_MANIFEST=ingress-traefik.yaml
CERT_MANAGER_MANIFEST=cert-manager.yaml
CERT_MANAGER_CONFIG_MANIFEST=self-cert.yaml
CERT_MANIFEST=signed-cert.yaml

KARAVICTL=karavictl
SIDECAR_PROXY=sidecar-proxy

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

if [[ ! -f $CERT_MANAGER_MANIFEST ]]
then
	# Download cert-manager manifest
	curl -kL -o  ${DIST}/$CERT_MANAGER_MANIFEST https://github.com/jetstack/cert-manager/releases/download/v1.2.0/cert-manager.yaml
fi

# Pull all 3rd party images to ensure they exist locally.
# You can also run "make dep" to pull these down without 
# having to run this script.
for image in $(grep "image: docker.io" deployment.yaml | awk -F' ' '{ print $2 }' | xargs echo); do
  docker pull $image
done
# Save all referenced images into a tarball.
grep "image: " deployment.yaml | awk -F' ' '{ print $2 }' | xargs docker save -o $CRED_SHIELD_IMAGES_TAR

#Pull all images required to install cert-manager
for image in $(grep "image: " ${DIST}/$CERT_MANAGER_MANIFEST | awk -F' ' '{ print $2 }' | xargs echo); do
  docker pull $image
done
# Save all referenced images into a tarball.
grep "image: " ${DIST}/$CERT_MANAGER_MANIFEST | awk -F' ' '{ print $2 }' | xargs docker save -o $CERT_MANAGER_IMAGES_TAR


# Create the bundle airgap tarfile.
cp $CRED_SHIELD_DEPLOYMENT_MANIFEST $CRED_SHIELD_INGRESS_MANIFEST $CERT_MANAGER_CONFIG_MANIFEST $CERT_MANIFEST $DIST/.
cp ../policies/*.rego ../policies/policy-install.sh $DIST/.
cp ../bin/$KARAVICTL $DIST/.

docker save $SIDECAR_PROXY:$SIDECAR_DOCKER_TAG -o $DIST/$SIDECAR_PROXY-$SIDECAR_DOCKER_TAG.tar

tar -czv -C $DIST -f karavi-airgap-install.tar.gz .

# Clean up the files that were just added to the bundle.
rm $K3S_INSTALL_SCRIPT \
	$K3S_BINARY \
	$K3S_IMAGES_TAR \
	$CRED_SHIELD_IMAGES_TAR \
	$CERT_MANAGER_IMAGES_TAR \
	${DIST}/$CERT_MANAGER_MANIFEST \
	${DIST}/$CERT_MANAGER_CONFIG_MANIFEST \
	${DIST}/$CERT_MANIFEST \
	${DIST}/$CRED_SHIELD_DEPLOYMENT_MANIFEST \
	${DIST}/$CRED_SHIELD_INGRESS_MANIFEST \
	${DIST}/*.rego \
	${DIST}/policy-install.sh \
	${DIST}/$SIDECAR_PROXY-$SIDECAR_DOCKER_TAG.tar \
	${DIST}/$KARAVICTL

# Move the tarball into dist.
mv karavi-airgap-install.tar.gz $DIST/.
