#!/bin/sh
set -x
[ $(id -u) -eq 0 ] || exec sudo $0 $@

cd "$(dirname "$0")"
k3s kubectl create configmap common -n karavi --from-file=./common.rego --save-config
k3s kubectl create configmap secret -n karavi --from-file=./secret.rego --save-config
k3s kubectl create configmap volumes-create -n karavi --from-file=./volumes_create.rego --save-config
k3s kubectl create configmap volumes-delete -n karavi --from-file=./volumes_delete.rego --save-config
k3s kubectl create configmap authz -n karavi --from-file=./url.rego --save-config
