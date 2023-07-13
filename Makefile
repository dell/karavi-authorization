# Copyright © 2021-2023 Dell Inc., or its subsidiaries. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
export PODMAN_TAG ?= 1.7.0
export SIDECAR_TAG ?= 1.7.0
# Get version and release from PODMAN_TAG
dot-delimiter = $(word $2,$(subst ., ,$1))
export VERSION = $(call dot-delimiter, ${PODMAN_TAG}, 1).$(call dot-delimiter, ${PODMAN_TAG}, 2)
export RELEASE = $(call dot-delimiter, ${PODMAN_TAG}, 3)

ifeq (${RELEASE},)
	VERSION=1.7
	RELEASE=0
endif

export VERSION_TAG ?= ${VERSION}-${RELEASE}
K3S_SELINUX_VERSION ?= 0.4-1

.PHONY: build
build:
	-mkdir -p ./bin
	cp Dockerfile ./bin/Dockerfile
	CGO_ENABLED=0 go build -o ./bin ./cmd/proxy-server/
	CGO_ENABLED=0 go build -o ./bin ./cmd/karavictl/
	CGO_ENABLED=0 go build -o ./bin ./cmd/sidecar-proxy/
	CGO_ENABLED=0 go build -o ./bin ./cmd/tenant-service/
	CGO_ENABLED=0 go build -o ./bin ./cmd/role-service/
	CGO_ENABLED=0 go build -o ./bin ./cmd/storage-service/

.PHONY: build-installer
build-installer: 
	# Requires dist artifacts
	go build -tags=prod -o ./bin ./deploy/

.PHONY: rpm
rpm:
	podman run --rm \
		-e VERSION \
		-e RELEASE \
		-v $$PWD/deploy/rpm/pkg:/srv/pkg \
		-v $$PWD/bin/deploy:/home/builder/rpm/deploy \
		-v $$PWD/deploy/rpm:/home/builder/rpm \
		rpmbuild/centos7

.PHONY: redeploy
redeploy: build podman
	# proxy-server
	podman save --output ./bin/proxy-server-$(PODMAN_TAG).tar proxy-server:$(PODMAN_TAG) 
	sudo /usr/local/bin/k3s ctr images import ./bin/proxy-server-$(PODMAN_TAG).tar
	sudo /usr/local/bin/k3s kubectl set image -n karavi deploy/proxy-server proxy-server=proxy-server:$(PODMAN_TAG)
	sudo /usr/local/bin/k3s kubectl rollout restart -n karavi deploy/proxy-server
	# tenant-service
	podman save --output ./bin/tenant-service-$(PODMAN_TAG).tar tenant-service:$(PODMAN_TAG) 
	sudo /usr/local/bin/k3s ctr images import ./bin/tenant-service-$(PODMAN_TAG).tar
	sudo /usr/local/bin/k3s kubectl set image -n karavi deploy/tenant-service tenant-service=tenant-service:$(PODMAN_TAG)
	sudo /usr/local/bin/k3s kubectl rollout restart -n karavi deploy/tenant-service
	# storage-service
	podman save --output ./bin/storage-service-$(PODMAN_TAG).tar storage-service:$(PODMAN_TAG) 
	sudo /usr/local/bin/k3s ctr images import ./bin/storage-service-$(PODMAN_TAG).tar
	sudo /usr/local/bin/k3s kubectl set image -n karavi deploy/storage-service storage-service=storage-service:$(PODMAN_TAG)
	sudo /usr/local/bin/k3s kubectl rollout restart -n karavi deploy/storage-service
	# role-service
	podman save --output ./bin/role-service-$(PODMAN_TAG).tar role-service:$(PODMAN_TAG) 
	sudo /usr/local/bin/k3s ctr images import ./bin/role-service-$(PODMAN_TAG).tar
	sudo /usr/local/bin/k3s kubectl set image -n karavi deploy/role-service role-service=role-service:$(PODMAN_TAG)
	sudo /usr/local/bin/k3s kubectl rollout restart -n karavi deploy/role-service

.PHONY: podman
podman: build
	podman build -t proxy-server:$(PODMAN_TAG) --build-arg APP=proxy-server ./bin/.
	podman build -t sidecar-proxy:$(SIDECAR_TAG) --build-arg APP=sidecar-proxy ./bin/.
	podman build -t tenant-service:$(PODMAN_TAG) --build-arg APP=tenant-service ./bin/.
	podman build -t role-service:$(PODMAN_TAG) --build-arg APP=role-service ./bin/.
	podman build -t storage-service:$(PODMAN_TAG) --build-arg APP=storage-service ./bin/.

.PHONY: protoc
protoc:
	protoc -I. \
		--go_out=paths=source_relative:. ./pb/*.proto --go-grpc_out=paths=source_relative:. \
		./pb/*.proto

.PHONY: dist
dist: podman dep
	cd ./deploy/ && ./airgap-prepare.sh
	curl -kL -o ./deploy/dist/microos-k3s-selinux.rpm https://rpm.rancher.io/k3s/latest/common/microos/noarch/k3s-selinux-${K3S_SELINUX_VERSION}.sle.noarch.rpm
	curl -kL -o ./deploy/dist/centos7-k3s-selinux.rpm https://rpm.rancher.io/k3s/latest/common/centos/7/noarch/k3s-selinux-${K3S_SELINUX_VERSION}.el7.noarch.rpm
	curl -kL -o ./deploy/dist/centos8-k3s-selinux.rpm https://rpm.rancher.io/k3s/latest/common/centos/8/noarch/k3s-selinux-${K3S_SELINUX_VERSION}.el8.noarch.rpm

.PHONY: dep
dep:
	# Pulls third party podman.io images that we depend on.
	for image in `grep "image: podman.io" deploy/deployment.yaml | awk -F' ' '{ print $$2 }' | xargs echo`; do \
		podman pull $$image; \
	done

.PHONY: distclean
distclean:
	-rm -r ./deploy/dist

.PHONY: test
test: testopa
	go test -count=1 -cover -race -timeout 30s -short ./...

.PHONY: testopa
testopa:
	podman run --rm -it -v ${PWD}/policies:/policies/ openpolicyagent/opa test -v /policies/

.PHONY: package
package:
	mkdir -p karavi_authorization_${PODMAN_TAG}
	cp ./deploy/rpm/x86_64/karavi-authorization-${VERSION_TAG}.x86_64.rpm karavi_authorization_${PODMAN_TAG}/
	cp ./deploy/dist/microos-k3s-selinux.rpm karavi_authorization_${PODMAN_TAG}/
	cp ./deploy/dist/centos7-k3s-selinux.rpm karavi_authorization_${PODMAN_TAG}/
	cp ./deploy/dist/centos8-k3s-selinux.rpm karavi_authorization_${PODMAN_TAG}/
	cp ./scripts/install_karavi_auth.sh karavi_authorization_${PODMAN_TAG}/
	cp ./scripts/traefik_nodeport.sh karavi_authorization_${PODMAN_TAG}/
	cp -r ./policies karavi_authorization_${PODMAN_TAG}/
	mkdir -p package
	tar -czvf package/karavi_authorization_${PODMAN_TAG}.tar.gz karavi_authorization_${PODMAN_TAG}
	rm -rf karavi_authorization_${PODMAN_TAG}
