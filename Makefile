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
export BUILDER_TAG ?= 1.9.1
export SIDECAR_TAG ?= 1.9.1

# figure out if podman or docker should be used (use podman if found)
ifneq (, $(shell which podman 2>/dev/null))
export BUILDER=podman
else
export BUILDER=docker
endif

# Get version and release from BUILDER_TAG
dot-delimiter = $(word $2,$(subst ., ,$1))
export VERSION = $(call dot-delimiter, ${BUILDER_TAG}, 1).$(call dot-delimiter, ${BUILDER_TAG}, 2)
export RELEASE = $(call dot-delimiter, ${BUILDER_TAG}, 3)

ifeq (${RELEASE},)
	VERSION=1.9
	RELEASE=1
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
	$(BUILDER) run --rm \
		-e VERSION \
		-e RELEASE \
		-v $$PWD/deploy/rpm/pkg:/srv/pkg \
		-v $$PWD/bin/deploy:/home/builder/rpm/deploy \
		-v $$PWD/deploy/rpm:/home/builder/rpm \
		rpmbuild/centos7

.PHONY: redeploy
redeploy: build builder
	# proxy-server
	$(BUILDER) save --output ./bin/proxy-server-$(BUILDER_TAG).tar localhost/proxy-server:$(BUILDER_TAG) 
	sudo /usr/local/bin/k3s ctr images import ./bin/proxy-server-$(BUILDER_TAG).tar
	sudo /usr/local/bin/k3s kubectl set image -n karavi deploy/proxy-server proxy-server=localhost/proxy-server:$(BUILDER_TAG)
	sudo /usr/local/bin/k3s kubectl rollout restart -n karavi deploy/proxy-server
	# tenant-service
	$(BUILDER) save --output ./bin/tenant-service-$(BUILDER_TAG).tar localhost/tenant-service:$(BUILDER_TAG) 
	sudo /usr/local/bin/k3s ctr images import ./bin/tenant-service-$(BUILDER_TAG).tar
	sudo /usr/local/bin/k3s kubectl set image -n karavi deploy/tenant-service tenant-service=localhost/tenant-service:$(BUILDER_TAG)
	sudo /usr/local/bin/k3s kubectl rollout restart -n karavi deploy/tenant-service
	# storage-service
	$(BUILDER) save --output ./bin/storage-service-$(BUILDER_TAG).tar localhost/storage-service:$(BUILDER_TAG) 
	sudo /usr/local/bin/k3s ctr images import ./bin/storage-service-$(BUILDER_TAG).tar
	sudo /usr/local/bin/k3s kubectl set image -n karavi deploy/storage-service storage-service=localhost/storage-service:$(BUILDER_TAG)
	sudo /usr/local/bin/k3s kubectl rollout restart -n karavi deploy/storage-service
	# role-service
	$(BUILDER) save --output ./bin/role-service-$(BUILDER_TAG).tar localhost/role-service:$(BUILDER_TAG) 
	sudo /usr/local/bin/k3s ctr images import ./bin/role-service-$(BUILDER_TAG).tar
	sudo /usr/local/bin/k3s kubectl set image -n karavi deploy/role-service role-service=localhost/role-service:$(BUILDER_TAG)
	sudo /usr/local/bin/k3s kubectl rollout restart -n karavi deploy/role-service

.PHONY: builder
builder: build download-csm-common
	$(eval include csm-common.mk)
	$(BUILDER) build -t localhost/proxy-server:$(BUILDER_TAG) --build-arg APP=proxy-server --build-arg BASEIMAGE=$(DEFAULT_BASEIMAGE) ./bin/.
	$(BUILDER) build -t localhost/sidecar-proxy:$(SIDECAR_TAG) --build-arg APP=sidecar-proxy --build-arg BASEIMAGE=$(DEFAULT_BASEIMAGE) ./bin/.
	$(BUILDER) build -t localhost/tenant-service:$(BUILDER_TAG) --build-arg APP=tenant-service --build-arg BASEIMAGE=$(DEFAULT_BASEIMAGE) ./bin/.
	$(BUILDER) build -t localhost/role-service:$(BUILDER_TAG) --build-arg APP=role-service --build-arg BASEIMAGE=$(DEFAULT_BASEIMAGE) ./bin/.
	$(BUILDER) build -t localhost/storage-service:$(BUILDER_TAG) --build-arg APP=storage-service --build-arg BASEIMAGE=$(DEFAULT_BASEIMAGE) ./bin/.

.PHONY: protoc
protoc:
	protoc -I. \
		--go_out=paths=source_relative:. ./pb/*.proto --go-grpc_out=paths=source_relative:. \
		./pb/*.proto

.PHONY: dist
dist: builder dep
	cd ./deploy/ && ./airgap-prepare.sh
	curl -kL -o ./deploy/dist/microos-k3s-selinux.rpm https://rpm.rancher.io/k3s/latest/common/microos/noarch/k3s-selinux-${K3S_SELINUX_VERSION}.sle.noarch.rpm
	curl -kL -o ./deploy/dist/centos7-k3s-selinux.rpm https://rpm.rancher.io/k3s/latest/common/centos/7/noarch/k3s-selinux-${K3S_SELINUX_VERSION}.el7.noarch.rpm
	curl -kL -o ./deploy/dist/centos8-k3s-selinux.rpm https://rpm.rancher.io/k3s/latest/common/centos/8/noarch/k3s-selinux-${K3S_SELINUX_VERSION}.el8.noarch.rpm

.PHONY: dep
dep:
	# Pulls third party docker.io images that we depend on.
	for image in `grep "image: docker.io" deploy/deployment.yaml | awk -F' ' '{ print $$2 }' | xargs echo`; do \
		$(BUILDER) pull $$image; \
	done

.PHONY: distclean
distclean:
	-rm -r ./deploy/dist

.PHONY: test
test: testopa
	go test -count=1 -cover -race -timeout 30s -short ./...

.PHONY: testopa
testopa:
	$(BUILDER) run --rm -it -v ${PWD}/policies:/policies/ openpolicyagent/opa test -v /policies/

.PHONY: package
package:
	mkdir -p karavi_authorization_${BUILDER_TAG}
	cp ./deploy/rpm/x86_64/karavi-authorization-${VERSION_TAG}.x86_64.rpm karavi_authorization_${BUILDER_TAG}/
	cp ./deploy/dist/microos-k3s-selinux.rpm karavi_authorization_${BUILDER_TAG}/
	cp ./deploy/dist/centos7-k3s-selinux.rpm karavi_authorization_${BUILDER_TAG}/
	cp ./deploy/dist/centos8-k3s-selinux.rpm karavi_authorization_${BUILDER_TAG}/
	cp ./scripts/install_karavi_auth.sh karavi_authorization_${BUILDER_TAG}/
	cp ./scripts/traefik_nodeport.sh karavi_authorization_${BUILDER_TAG}/
	cp -r ./policies karavi_authorization_${BUILDER_TAG}/
	mkdir -p package
	tar -czvf package/karavi_authorization_${BUILDER_TAG}.tar.gz karavi_authorization_${BUILDER_TAG}
	rm -rf karavi_authorization_${BUILDER_TAG}

.PHONY: download-csm-common
download-csm-common:
	curl -O -L https://raw.githubusercontent.com/dell/csm/main/config/csm-common.mk
