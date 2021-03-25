#!/bin/sh
set -x
[ $(id -u) -eq 0 ] || exec sudo $0 $@

counter=1
while [[ $(k3s kubectl get namespaces | grep karavi | wc -l) -ne 1 ]]
do 
    if [[ "$counter" -eq 30 ]]
    then
        echo "karavi namespace not available in k3s"
        exit 1
    fi
    ((counter++))
    echo "Waiting for karavi namespace in k3s to be available..."
    sleep 1
done

cd "$(dirname "$0")"
K3S=/usr/local/bin/k3s
$K3S kubectl create configmap common -n karavi --from-file=./common.rego --save-config
$K3S kubectl create configmap volumes-create -n karavi --from-file=./volumes_create.rego --save-config
$K3S kubectl create configmap volumes-delete -n karavi --from-file=./volumes_delete.rego --save-config
$K3S kubectl create configmap volumes-unmap -n karavi --from-file=./volumes_unmap.rego --save-config
$K3S kubectl create configmap volumes-map -n karavi --from-file=./volumes_map.rego --save-config
$K3S kubectl create configmap authz -n karavi --from-file=./url.rego --save-config
