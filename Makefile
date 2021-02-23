DOCKER_TAG ?= latest

.PHONY: build
build:
	-mkdir -p ./bin
	cp Dockerfile ./bin/Dockerfile
	CGO_ENABLED=0 go build -o ./bin ./cmd/proxy-server/
	CGO_ENABLED=0 go build -o ./bin ./cmd/github-auth-provider/
	CGO_ENABLED=0 go build -o ./bin ./cmd/karavictl/
	CGO_ENABLED=0 go build -o ./bin ./cmd/sidecar-proxy/
	CGO_ENABLED=0 go build -o ./bin ./cmd/tenant-service/

.PHONY: build-installer
build-installer: 
	# Requires dist artifacts
	go build -tags=prod -o ./bin ./deploy/

.PHONY: redeploy
redeploy: build docker
	# proxy-server
	docker save --output ./bin/proxy-server-$(DOCKER_TAG).tar proxy-server:$(DOCKER_TAG) 
	sudo k3s ctr images import ./bin/proxy-server-$(DOCKER_TAG).tar
	sudo k3s kubectl rollout restart -n karavi deploy/proxy-server
	# github-auth-provider
	docker save --output ./bin/github-auth-provider-$(DOCKER_TAG).tar github-auth-provider:$(DOCKER_TAG) 
	sudo k3s ctr images import ./bin/github-auth-provider-$(DOCKER_TAG).tar
	sudo k3s kubectl rollout restart -n karavi deploy/github-auth-provider
	# tenant-service
	docker save --output ./bin/tenant-service-$(DOCKER_TAG).tar tenant-service:$(DOCKER_TAG) 
	sudo k3s ctr images import ./bin/tenant-service-$(DOCKER_TAG).tar
	sudo k3s kubectl rollout restart -n karavi deploy/tenant-service

.PHONY: docker
docker: build
	docker build -t proxy-server:$(DOCKER_TAG) --build-arg APP=proxy-server ./bin/.
	docker build -t github-auth-provider:$(DOCKER_TAG)  --build-arg APP=github-auth-provider ./bin/.
	docker build -t sidecar-proxy:$(DOCKER_TAG) --build-arg APP=sidecar-proxy ./bin/.
	docker build -t tenant-service:$(DOCKER_TAG) --build-arg APP=tenant-service ./bin/.

.PHONY: deploy
deploy:
	./deploy/init-cluster.sh $(DOCKER_TAG)

.PHONY: protoc
protoc:
	protoc -I. \
		--go_out=paths=source_relative:. ./pb/*.proto --go-grpc_out=paths=source_relative:. \
		./pb/*.proto

.PHONY: dist
dist: docker
	cd ./deploy/ && ./airgap-prepare.sh

.PHONY: distclean
distclean:
	-rm -r ./deploy/dist

.PHONY: test
test:
	go test -count=1 -cover -race -timeout 30s -short ./...

