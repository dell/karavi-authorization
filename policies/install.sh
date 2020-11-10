sudo k3s kubectl create configmap volumes-create -n karavi --from-file=./volumes_create.rego --save-config
sudo k3s kubectl create configmap volumes-delete -n karavi --from-file=./volumes_delete.rego --save-config
sudo k3s kubectl apply -n karavi -f role-data.yaml
