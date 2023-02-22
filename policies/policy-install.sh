#!/bin/sh
# Copyright © 2021-2022 Dell Inc. or its subsidiaries. All Rights Reserved.
# 
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#      http://www.apache.org/licenses/LICENSE-2.0
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

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

if [[ $($K3S kubectl get secret karavi-storage-secret -n karavi | grep karavi-storage-secret | wc -l) -ne 1 ]]
then
    echo "Creating karavi storage secret"
    $K3S kubectl apply -f ./karavi-storage-secret.yaml
fi

if [[ $($K3S kubectl get configmap common -n karavi | grep common | wc -l) -ne 1 ]]
then
    $K3S kubectl create configmap common -n karavi --from-file=./common.rego --save-config --dry-run=client -o yaml | $K3S kubectl apply -f -
fi
$K3S kubectl create configmap powermax-volumes-create -n karavi --from-file=./volumes_powermax_create.rego --save-config --dry-run=client -o yaml | $K3S kubectl apply -f -
$K3S kubectl create configmap volumes-create -n karavi --from-file=./volumes_create.rego --save-config --dry-run=client -o yaml | $K3S kubectl apply -f -
$K3S kubectl create configmap volumes-delete -n karavi --from-file=./volumes_delete.rego --save-config --dry-run=client -o yaml | $K3S kubectl apply -f -
$K3S kubectl create configmap volumes-unmap -n karavi --from-file=./volumes_unmap.rego --save-config --dry-run=client -o yaml | $K3S kubectl apply -f -
$K3S kubectl create configmap volumes-map -n karavi --from-file=./volumes_map.rego --save-config --dry-run=client -o yaml | $K3S kubectl apply -f -
