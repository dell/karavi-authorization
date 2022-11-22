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

if [[ $(k3s kubectl get secret karavi-storage-secret -n karavi | grep karavi-storage-secret | wc -l) -ne 1 ]]
then
    echo "Creating karavi storage secret"
    k3s kubectl apply -f ./karavi-storage-secret.yaml
fi

if [[ $(k3s kubectl get configmap common -n karavi | grep common | wc -l) -ne 1 ]]
then
    k3s kubectl create configmap common -n karavi --from-file=./common.rego --save-config --dry-run=client -o yaml | k3s kubectl apply -f -
fi
k3s kubectl create configmap powermax-volumes-create -n karavi --from-file=./volumes_powermax_create.rego --save-config --dry-run=client -o yaml | k3s kubectl apply -f -
k3s kubectl create configmap powerscale-volumes-create -n karavi --from-file=./volumes_powerscale_create.rego --save-config --dry-run=client -o yaml | k3s kubectl apply -f -
k3s kubectl create configmap volumes-create -n karavi --from-file=./volumes_create.rego --save-config --dry-run=client -o yaml | k3s kubectl apply -f -
k3s kubectl create configmap volumes-delete -n karavi --from-file=./volumes_delete.rego --save-config --dry-run=client -o yaml | k3s kubectl apply -f -
k3s kubectl create configmap volumes-unmap -n karavi --from-file=./volumes_unmap.rego --save-config --dry-run=client -o yaml | k3s kubectl apply -f -
k3s kubectl create configmap volumes-map -n karavi --from-file=./volumes_map.rego --save-config --dry-run=client -o yaml | k3s kubectl apply -f -
k3s kubectl create configmap powerflex-urls -n karavi --from-file=./url.rego --save-config --dry-run=client -o yaml | k3s kubectl apply -f -
k3s kubectl create configmap powermax-urls -n karavi --from-file=./powermax_url.rego --save-config --dry-run=client -o yaml | k3s kubectl apply -f -
k3s kubectl create configmap powerscale-urls -n karavi --from-file=./powerscale_url.rego --save-config --dry-run=client -o yaml | k3s kubectl apply -f -
