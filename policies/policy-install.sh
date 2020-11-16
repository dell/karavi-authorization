#!/bin/bash

cd "$(dirname "$0")"
sudo k3s kubectl create configmap common -n karavi --from-file=./common.rego --save-config
sudo k3s kubectl create configmap secret -n karavi --from-file=./secret.rego --save-config
sudo k3s kubectl create configmap volumes-create -n karavi --from-file=./volumes_create.rego --save-config
sudo k3s kubectl create configmap volumes-delete -n karavi --from-file=./volumes_delete.rego --save-config
