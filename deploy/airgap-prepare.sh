#!/bin/bash -x

# Copyright © 2021-2023 Dell Inc. or its subsidiaries. All Rights Reserved.
# 
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

ARCH=amd64
SIDECAR_DOCKER_TAG=${SIDECAR_TAG}
DIST=dist

# Create the dist directory, if not already present.
mkdir -p dist

K3S_INSTALL_SCRIPT=${DIST}/k3s-install.sh
K3S_BINARY=${DIST}/k3s
K3S_IMAGES_TAR=${DIST}/k3s-airgap-images-$ARCH.tar

CERT_MANAGER_IMAGES_TAR=${DIST}/cert-manager-images.tar
CRED_SHIELD_IMAGES_TAR=${DIST}/credential-shield-images.tar

# Update podman tag in deployment.yaml
cp deployment.yaml ${DIST}/deployment.yaml
sed -i 's/\${DOCKER_TAG}/'${DOCKER_TAG}'/g' ${DIST}/deployment.yaml

CRED_SHIELD_DEPLOYMENT_MANIFEST=${DIST}/deployment.yaml
CRED_SHIELD_INGRESS_MANIFEST=ingress-traefik.yaml
CRED_SHIELD_TLS_OPTION_MANIFEST=tls-option.yaml
CERT_MANAGER_MANIFEST=cert-manager.yaml
CERT_MANAGER_CONFIG_MANIFEST=self-cert.yaml
CERT_MANIFEST=signed-cert.yaml
TLS_STORE_MANIFEST=tls-store.yaml

KARAVICTL=karavictl
SIDECAR_PROXY=sidecar-proxy

INSTALL_SCRIPT=install.sh

# Download install script
if [[ ! -f $K3S_INSTALL_SCRIPT ]]
then
	curl -kL -o $K3S_INSTALL_SCRIPT https://get.k3s.io/
fi

# Download k3s
if [[ ! -f $K3S_BINARY ]]
then
	curl -kL -o $K3S_BINARY  https://github.com/k3s-io/k3s/releases/download/v1.25.5%2Bk3s2/k3s
fi

if [[ ! -f $K3S_IMAGES_TAR ]]
then
	# Download k3s images
	curl -kL -o $K3S_IMAGES_TAR https://github.com/k3s-io/k3s/releases/download/v1.25.5%2Bk3s1/k3s-airgap-images-$ARCH.tar
fi

if [[ ! -f $CERT_MANAGER_MANIFEST ]]
then
	# Download cert-manager manifest
	curl -kL -o  ${DIST}/$CERT_MANAGER_MANIFEST https://github.com/jetstack/cert-manager/releases/download/v1.10.1/cert-manager.yaml
fi

# Pull all 3rd party images to ensure they exist locally.
# You can also run "make dep" to pull these down without 
# having to run this script.
for image in $(grep "image: docker.io" ${DIST}/deployment.yaml | awk -F' ' '{ print $2 }' | xargs echo); do
  podman pull $image
done
# Save all referenced images into a tarball.
grep "image: " ${DIST}/deployment.yaml | awk -F' ' '{ print $2 }' | xargs podman save -o $CRED_SHIELD_IMAGES_TAR

#Pull all images required to install cert-manager
for image in $(grep "image: " ${DIST}/$CERT_MANAGER_MANIFEST | awk -F' ' '{ print $2 }' | xargs echo); do
  podman pull $image
done
# Save all referenced images into a tarball.
grep "image: " ${DIST}/$CERT_MANAGER_MANIFEST | awk -F' ' '{ print $2 }' | xargs podman save -o $CERT_MANAGER_IMAGES_TAR


# Create the bundle airgap tarfile.
cp $CRED_SHIELD_DEPLOYMENT_MANIFEST $CRED_SHIELD_INGRESS_MANIFEST $CERT_MANAGER_CONFIG_MANIFEST $CERT_MANIFEST $CRED_SHIELD_TLS_OPTION_MANIFEST $TLS_STORE_MANIFEST $DIST/.
cp ../bin/$KARAVICTL $DIST/.

podman save $SIDECAR_PROXY:$SIDECAR_DOCKER_TAG -o $DIST/$SIDECAR_PROXY-$SIDECAR_DOCKER_TAG.tar

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
	${DIST}/$CRED_SHIELD_TLS_OPTION_MANIFEST \
	${DIST}/$TLS_STORE_MANIFEST \
	${DIST}/$SIDECAR_PROXY-$SIDECAR_DOCKER_TAG.tar \
	${DIST}/$KARAVICTL \
	${DIST}/deployment.yaml

# Move the tarball into dist.
mv karavi-airgap-install.tar.gz $DIST/.
