DOCKER_TAG ?= latest

.PHONY: deploy
deploy:
	./deploy/init-cluster.sh $(DOCKER_TAG)

.PHONY: down
down:
	kind delete cluster --name=gatekeeper

.PHONY: docker
docker:
	docker build -t powerflex-reverse-proxy:$(DOCKER_TAG) .
	docker build -t github-auth-provider:$(DOCKER_TAG) -f github-auth-provider.Dockerfile .

.PHONY: protoc
protoc:
	protoc -I. \
		--go_out=paths=source_relative:. ./pb/*.proto --go-grpc_out=paths=source_relative:. \
		./pb/*.proto
