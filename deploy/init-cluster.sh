#!/bin/bash

VERSION=${1:=latest} # of the proxy image.

# Create a K8s cluster with Kind patches to enable ports to talk to an ingress.
cat <<EOF | kind create cluster --name=gatekeeper --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 443
    hostPort: 443
    protocol: TCP
EOF

if [ $? -ne 0 ]; then
	exit
fi

# Load our Docker container image into the local registry.
kind load docker-image powerflex-reverse-proxy:$VERSION --name=gatekeeper
kind load docker-image github-auth-provider:$VERSION --name=gatekeeper

# Install the ingress nginx controller and wait for it to be available.
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/master/deploy/static/provider/kind/deploy.yaml
kubectl wait --namespace ingress-nginx \
	--for=condition=available deploy ingress-nginx-controller \
	--timeout=90s
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s

# Finally, deploy our application.
kubectl apply -f deploy/deployment.yaml
