#!/bin/sh
set -x
[ $(id -u) -eq 0 ] || exec sudo $0 $@

K3S=/usr/local/bin/k3s

counter=1
while [[ $($K3S kubectl get namespaces | grep karavi | wc -l) -ne 1 ]]
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
$K3S kubectl create configmap common -n karavi --from-file=./common.rego --save-config
$K3S kubectl create configmap powermax-volumes-create -n karavi --from-file=./volumes_powermax_create.rego --save-config
$K3S kubectl create configmap powerscale-volumes-create -n karavi --from-file=./volumes_powerscale_create.rego --save-config
$K3S kubectl create configmap volumes-create -n karavi --from-file=./volumes_create.rego --save-config
$K3S kubectl create configmap volumes-delete -n karavi --from-file=./volumes_delete.rego --save-config
$K3S kubectl create configmap volumes-unmap -n karavi --from-file=./volumes_unmap.rego --save-config
$K3S kubectl create configmap volumes-map -n karavi --from-file=./volumes_map.rego --save-config
$K3S kubectl create configmap powerflex-urls -n karavi --from-file=./url.rego --save-config
$K3S kubectl create configmap powermax-urls -n karavi --from-file=./powermax_url.rego --save-config
