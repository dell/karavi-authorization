#!/bin/sh

kubectl -n kauth-stress delete -f helm/templates/stress-tester.yaml
kubectl -n kauth-stress apply -f helm/templates/stress-tester.yaml
kubectl get secrets,deployments,daemonsets -n kauth-stress -o yaml | /git/karavi-authorization/bin/karavictl inject --image-addr 10.247.98.98:5000/sidecar-proxy:latest --proxy-host 10.247.66.155 | kubectl apply -f -
kubectl rollout status -n kauth-stress deploy/vxflexos-controller
kubectl rollout status -n kauth-stress ds/vxflexos-node