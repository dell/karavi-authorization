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
export DOCKER_TAG ?= 1.6.0
export SIDECAR_TAG ?= 1.6.0
export VERSION_TAG ?= 1.6-0
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
	docker run --rm \
		-v $$PWD/deploy/rpm/pkg:/srv/pkg \
		-v $$PWD/bin/deploy:/home/builder/rpm/deploy \
		-v $$PWD/deploy/rpm:/home/builder/rpm \
		rpmbuild/centos7

.PHONY: redeploy
redeploy: build docker
	# proxy-server
	docker save --output ./bin/proxy-server-$(DOCKER_TAG).tar proxy-server:$(DOCKER_TAG) 
	sudo /usr/local/bin/k3s ctr images import ./bin/proxy-server-$(DOCKER_TAG).tar
	sudo /usr/local/bin/k3s kubectl rollout restart -n karavi deploy/proxy-server
	# tenant-service
	docker save --output ./bin/tenant-service-$(DOCKER_TAG).tar tenant-service:$(DOCKER_TAG) 
	sudo /usr/local/bin/k3s ctr images import ./bin/tenant-service-$(DOCKER_TAG).tar
	sudo /usr/local/bin/k3s kubectl rollout restart -n karavi deploy/tenant-service
	# storage-service
	docker save --output ./bin/storage-service-$(DOCKER_TAG).tar storage-service:$(DOCKER_TAG) 
	sudo /usr/local/bin/k3s ctr images import ./bin/storage-service-$(DOCKER_TAG).tar
	sudo /usr/local/bin/k3s kubectl rollout restart -n karavi deploy/storage-service
	# role-service
	docker save --output ./bin/role-service-$(DOCKER_TAG).tar role-service:$(DOCKER_TAG) 
	sudo /usr/local/bin/k3s ctr images import ./bin/role-service-$(DOCKER_TAG).tar
	sudo /usr/local/bin/k3s kubectl rollout restart -n karavi deploy/role-service

.PHONY: docker
docker: build
	docker build -t proxy-server:$(DOCKER_TAG) --build-arg APP=proxy-server ./bin/.
	docker build -t sidecar-proxy:$(SIDECAR_TAG) --build-arg APP=sidecar-proxy ./bin/.
	docker build -t tenant-service:$(DOCKER_TAG) --build-arg APP=tenant-service ./bin/.
	docker build -t role-service:$(DOCKER_TAG) --build-arg APP=role-service ./bin/.
	docker build -t storage-service:$(DOCKER_TAG) --build-arg APP=storage-service ./bin/.

.PHONY: protoc
protoc:
	protoc -I. \
		--go_out=paths=source_relative:. ./pb/*.proto --go-grpc_out=paths=source_relative:. \
		./pb/*.proto

.PHONY: dist
dist: docker dep
	cd ./deploy/ && ./airgap-prepare.sh
	curl -kL -o ./deploy/dist/microos-k3s-selinux.rpm https://rpm.rancher.io/k3s/latest/common/microos/noarch/k3s-selinux-${K3S_SELINUX_VERSION}.sle.noarch.rpm
	curl -kL -o ./deploy/dist/centos7-k3s-selinux.rpm https://rpm.rancher.io/k3s/latest/common/centos/7/noarch/k3s-selinux-${K3S_SELINUX_VERSION}.el7.noarch.rpm
	curl -kL -o ./deploy/dist/centos8-k3s-selinux.rpm https://rpm.rancher.io/k3s/latest/common/centos/8/noarch/k3s-selinux-${K3S_SELINUX_VERSION}.el8.noarch.rpm

.PHONY: dep
dep:
	# Pulls third party docker.io images that we depend on.
	for image in `grep "image: docker.io" deploy/deployment.yaml | awk -F' ' '{ print $$2 }' | xargs echo`; do \
		docker pull $$image; \
	done

.PHONY: distclean
distclean:
	-rm -r ./deploy/dist

.PHONY: test
test: testopa
	go test -count=1 -cover -race -timeout 30s -short ./...

.PHONY: testopa
testopa:
	docker run --rm -it -v ${PWD}/policies:/policies/ openpolicyagent/opa test -v /policies/

.PHONY: package
package:
	mkdir -p karavi_authorization_${DOCKER_TAG}
	cp ./deploy/rpm/x86_64/karavi-authorization-${VERSION_TAG}.x86_64.rpm karavi_authorization_${DOCKER_TAG}/
	cp ./deploy/dist/microos-k3s-selinux.rpm karavi_authorization_${DOCKER_TAG}/
	cp ./deploy/dist/centos7-k3s-selinux.rpm karavi_authorization_${DOCKER_TAG}/
	cp ./deploy/dist/centos8-k3s-selinux.rpm karavi_authorization_${DOCKER_TAG}/
	cp ./scripts/install_karavi_auth.sh karavi_authorization_${DOCKER_TAG}/
	cp ./scripts/traefik_nodeport.sh karavi_authorization_${DOCKER_TAG}/
	cp -r ./policies karavi_authorization_${DOCKER_TAG}/
	mkdir -p package
	tar -czvf package/karavi_authorization_${DOCKER_TAG}.tar.gz karavi_authorization_${DOCKER_TAG}
	rm -rf karavi_authorization_${DOCKER_TAG}
