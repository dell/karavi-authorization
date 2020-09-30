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
