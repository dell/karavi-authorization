DOCKER_TAG ?= latest

.PHONY: build
build:
	-mkdir -p ./bin
	cp Dockerfile ./bin/Dockerfile
	CGO_ENABLED=0 go build -o ./bin ./cmd/storage-gatekeeper/
	CGO_ENABLED=0 go build -o ./bin ./cmd/github-auth-provider/
	CGO_ENABLED=0 go build -o ./bin ./cmd/karavictl/
	CGO_ENABLED=0 go build -o ./bin ./cmd/sidecar-proxy/

.PHONY: redeploy
redeploy: build docker
	# powerflex-reverse-proxy
	docker save --output ./bin/powerflex-reverse-proxy-$(DOCKER_TAG).tar powerflex-reverse-proxy:$(DOCKER_TAG) 
	sudo k3s ctr images import ./bin/powerflex-reverse-proxy-$(DOCKER_TAG).tar
	sudo k3s kubectl rollout restart -n karavi deploy/powerflex-reverse-proxy
	# github-auth-provider
	docker save --output ./bin/github-auth-provider-$(DOCKER_TAG).tar github-auth-provider:$(DOCKER_TAG) 
	sudo k3s ctr images import ./bin/github-auth-provider-$(DOCKER_TAG).tar
	sudo k3s kubectl rollout restart -n karavi deploy/github-auth-provider

.PHONY: docker
docker: build
	docker build -t powerflex-reverse-proxy:$(DOCKER_TAG) --build-arg APP=storage-gatekeeper ./bin/.
	docker build -t github-auth-provider:$(DOCKER_TAG)  --build-arg APP=github-auth-provider ./bin/.
	docker build -t sidecar-proxy:$(DOCKER_TAG) --build-arg APP=sidecar-proxy ./bin/.

.PHONY: deploy
deploy:
	./deploy/init-cluster.sh $(DOCKER_TAG)

.PHONY: down
down:
	kind delete cluster --name=gatekeeper

.PHONY: protoc
protoc:
	protoc -I. \
		--go_out=paths=source_relative:. ./pb/*.proto --go-grpc_out=paths=source_relative:. \
		./pb/*.proto

.PHONY: dist
dist:
	cd ./deploy/ && ./airgap-prepare.sh

.PHONY: distclean
distclean:
	-rm -r ./deploy/dist
