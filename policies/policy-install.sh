#!/bin/sh
set -x
[ $(id -u) -eq 0 ] || exec sudo $0 $@

cd "$(dirname "$0")"
K3S=/usr/local/bin/k3s
$K3S kubectl create configmap common -n karavi --from-file=./common.rego --save-config
$K3S kubectl create configmap volumes-create -n karavi --from-file=./volumes_create.rego --save-config
$K3S kubectl create configmap volumes-delete -n karavi --from-file=./volumes_delete.rego --save-config
$K3S kubectl create configmap volumes-unmap -n karavi --from-file=./volumes_unmap.rego --save-config
$K3S kubectl create configmap volumes-map -n karavi --from-file=./volumes_map.rego --save-config
$K3S kubectl create configmap authz -n karavi --from-file=./url.rego --save-config
