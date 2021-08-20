<!--
Copyright (c) 2021 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
-->
# Getting Started Guide

This guide contains sections detailing Karavi Authorization capabilities, supported platforms, supported CSI Drivers, deployment instructions, roles and responsibilities, and CLI commands that can be used to manage the system.

## Karavi Authorization Capabilities

| Feature | Dell CSI Driver |
| ------- | --------- |
| Enforcing quota limits| Yes |
| Shielding storage admin credentials | Yes |
| LDAP Support | No |

## Supported Platforms

The following matrix provides a list of all supported versions for each Dell EMC Storage product.

| Platforms | Version|
| -------- | --------- |
| PowerFlex | v3.0, v3.5 |
| PowerMax| 5978.669.669, 5978.711.711, Unisphere 9.2 |
| Kubernetes | 1.18, 1,19, 1.20 |
| OpenShift | 4.6, 4.7 |

## CSI Drivers

Karavi Authorization supports the following CSI drivers and versions.

| Storage Array | CSI Driver | Supported Versions |
| ------------- | ---------- | ------------------ |
| CSI Driver for Dell EMC PowerFlex | [csi-powerflex](https://github.com/dell/csi-powerflex) | v1.4.0 |
| CSI Driver for Dell EMC PowerMax | [csi-powermax](https://github.com/dell/csi-powermax) | v1.6.0 |

**NOTE:** If the deployed CSI driver has a number of controller pods equal to the number of schedulable nodes in your cluster, Karavi Authorization may not be able to inject properly into the driver's controller pod.
To resolve this, please refer to our [troubleshooting guide](TROUBLESHOOTING.md#karavictl-inject-leaves-vxflexos-controller-in-pending-state) on the topic.

## Deploying Karavi Authorization

1. A single binary installer can be built and executed to perform the deployment of Karavi Authorization.
Use the following Makefile targets to build the installer:

```
make dist build-installer rpm
```

The `build-installer` step creates a binary at `bin/deploy` and embeds all components required for installation. The `rpm` step generates an RPM package and stores it at `deploy/rpm/x86_64/`.
This allows for Karavi Authorization to be installed in network-restricted environments.
A Storage Administrator can execute the installer or rpm package as a root user or via `sudo`.

2. Before installing the rpm, some network and security configuration inputs need to be provided in json format. The json file should be created in the location `$HOME/.karavi/config.json` having the following contents:

```
{
  "web": {
    "sidecarproxyaddr": "docker_registry/sidecar-proxy:latest",
    "jwtsigningsecret": "secret"
  },
  "proxy": {
    "host": ":8080"
  },
  "zipkin": {
    "collectoruri": "http://DNS_host_name:9411/api/v2/spans",
    "probability": 1
  },
  "certificate": {
    "keyFile": "path_to_private_key_file",
    "crtFile": "path_to_host_cert_file",
    "rootCertificate": "path_to_root_CA_file"
  },
  "hostName": "DNS_host_name"
}
```

In the above template, `DNS_host_name` refers to the host name of the system in which Karavi Authorization server will be installed. This host name can be found by running the below command on the system:

```
nslookup <IP_address>
```

3. In order to configure secure grpc connectivity, an additional subdomain in the format `grpc.DNS_host_name` is also required. All traffic from `grpc.DNS_host_name` needs to be routed to `DNS_host_name` address, this can be configured by adding a new DNS entry for `grpc.DNS_host_name` or providing a temporary path in the `/etc/hosts` file.  
NOTE: The certificate provided in `crtFile` should be valid for both the `DNS_host_name` and the `grpc.DNS_host_name` address.  
For example, create the certificate config file with alternate names (to include example.com and grpc.example.com) and then create the .crt file:  

```  
CN = example.com
subjectAltName = @alt_names
[alt_names]
DNS.1 = grpc.example.com

openssl x509 -req -in cert_request_file.csr -CA root_CA.pem -CAkey private_key_File.key -CAcreateserial -out example.com.crt -days 365 -sha256
```

4. To install the rpm package on the system, run the below command:

```
rpm -ivh <rpm_file_name>
```

5. After deployment, application data will be stored on the system under `/var/lib/rancher/k3s/storage/`.

### Dynamic parameters

Karavi Authorization has a subset of configuration parameters that can be updated dynamically:

| Parameter | Type | Default | Description |
| --------- | ---- | ------- | ----------- |
| certificate.crtFile | String | "" |Path to the host certificate file |
| certificate.keyFile | String | "" |Path to the host private key file |
| certificate.rootCertificate | String | "" |Path to the root CA file  |
| web.sidecarproxyaddr | String |"127.0.0.1:5000/sidecar-proxy:latest" |Docker registry address of the Karavi Authorization sidecar-proxy |
| web.jwtsigningsecret | String | "secret" |The secret used to sign JWT tokens | 

Updating configuration parameters can be done by editing the `karavi-config-secret`. The secret can be queried using k3s and kubectl like so: 

`k3s kubectl -n karavi get secret/karavi-config-secret`. 

To update or add parameters, you must edit the base64 encoded data in the secret. The` karavi-config-secret` data can be decoded like so:

`k3s kubectl -n karavi get secret/karavi-config-secret -o yaml | grep config.yaml | head -n 1 | awk '{print $2}' | base64 -d`

Save the output to a file or copy it to an editor to make changes. Once you are done with the changes, you must encode the data to base64. If your changes are in a file, you can encode it like so:

`cat <file> | base64`

Copy the new, encoded data and edit the `karavi-config-secret` with the new data. Run this command to edit the secret:

`k3s kubectl -n karavi edit secret/karavi-config-secret`

Replace the data in `config.yaml` under the `data` field with your new, encoded data. Save the changes and Karavi Authorization will read the changed secret.

**Note**: If you are updating the signing secret, the tenants need to be updated with new tokens via the `karavictl generate token` command like so:

`karavictl generate token --tenant $TenantName --insecure --addr "grpc.${AuthorizationHost}" | jq -r '.Token' > kubectl -n $namespace apply -f -`

## Other Dynamic Configuration Settings

Some settings are not stored in the `karavi-config-secret` but in the csm-config-params ConfigMap, such as LOG_LEVEL and LOG_FORMAT. To update the karavi-authorization logging settings during runtime, run the below command on the K3s cluster, make your changes, and save the updated configmap data.

```
k3s kubectl -n karavi edit configmap/csm-config-params
```

This edit will not update the logging level for the sidecar-proxy containers running in the csi-driver pods. To update the sidecar-proxy logging levels, you must update the associated csi-driver ConfigMap in a similar fashion:

```
kubectl -n <driver_namespace> edit configmap/<release_name>-config-params
```

Using PowerFlex as an example, `kubectl -n vxflexos edit configmap/vxflexos-config-params` can be used to update the logging level of the sidecar-proxy and the driver.

## Roles and Responsibilities

### Storage Administrators

Storage Administrators can perform the following operations within Karavi Authorization

- Tenant Management (create, get, list, delete, bind roles, unbind roles)
- Token Management (generate, revoke)
- Storage System Management (create, get, list, update, delete)
- Storage Access Roles Management (assign to a storage system with an optional quota)

### Tenants

Tenants of Karavi Authorization can use the token provided by the Storage Administrators in their storage requests.

## Usage

### Tenant Management

Storage Administrators can use `karavictl` to perform tenant management operations.

#### Create Tenant

```
karavictl tenant create [flags]
```

#### Get Tenant

```
karavictl tenant get [flags]
```

#### List Tenants

```
karavictl tenant list [flags]
```

#### Delete Tenant

```
karavictl tenant delete [flags]
```

### Bind Role to Tenant

```
karavictl tenant bind role_name [flags]
```

#### Unbind Role from Tenant

```
karavictl tenant unbind role_name [flags]
```

### Token Management

Storage Administrators can use `karavictl` to perform token management operations:

#### Generate Token

```
karavictl generate token -t <tenant> [flags]
```

#### Revoke Tokens

```
karavictl tenant revoke -n <tenant>
```

#### Cancel Revoke Tokens

```
karavictl tenant revoke -n <tenant> --cancel
```

### Storage System Management

Storage Administrators can use `karavictl` to perform storage system management operations.

#### Create Storage System

```
karavictl storage create [flags]

Flags:
  -e, --endpoint string    Endpoint of REST API gateway
  -i, --insecure           Insecure skip verify
  -p, --pass string        Password (default "****")
  -s, --system-id string   System identifier (default "systemid")
  -t, --type string        Type of storage system ("powerflex", "powermax")
  -u, --user string        Username (default "admin")
```

#### Update Storage System

```
karavictl storage update [flags]

Flags:
  -e, --endpoint string    Endpoint of REST API gateway
  -i, --insecure           Insecure skip verify
  -p, --pass string        Password (default "****")
  -s, --system-id string   System identifier (default "systemid")
  -t, --type string        Type of storage system ("powerflex", "powermax")
  -u, --user string        Username (default "admin")
```

#### Get Storage System

```
karavictl storage get [flags]

Flags:
  -t, --type string        Type of storage system ("powerflex", "powermax")
  -s, --system-id string   System identifier (default "systemid")
```

#### List Storage Systems

```
karavictl storage list
```

#### Delete Storage System

```
karavictl storage delete [flags]

Flags:
  -t, --type string        Type of storage system ("powerflex", "powermax")
  -s, --system-id string   System identifier (default "systemid")
```

### Storage Access Roles Management

Storage Administrators can use `karavictl` to perform storage access role management operations.



#### Creating Storage Access Roles

```
karavictl roles create [file]

Flags:
  -h, --help               help for create
      --role strings       role in the form <name>=<type>=<id>=<pool>=<quota>
```

#### Updating Storage Access Roles

```
karavictl roles update [flags]

Flags:
  -h, --help               help for create
      --role strings       role in the form <name>=<type>=<id>=<pool>=<quota>
```

#### Listing Storage Access Roles

```
karavictl roles list
```

#### Getting Storage Access Role

```
karavictl roles get <rolename>
```

#### Deleting Storage Access Role

```
karavictl roles delete <rolename>
```

## Building Karavi Authorization

If you wish to clone and build Karavi Authorization, a Linux host is required with the following installed:

| Component       | Version   | Additional Information                                                                                                                     |
| --------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| Docker          | v19+      | [Docker installation](https://docs.docker.com/engine/install/)                                                                                                    |
| Golang          | v1.16    | [Golang installation](https://github.com/travis-ci/gimme)                                                                                                         |
| git             | latest    | [Git installation](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git)                                                                              |
| kubectl         | 1.17-1.19 | Ensure you copy the kubeconfig file from the Kubernetes cluster to the linux host. [kubectl installation](https://Kubernetes.io/docs/tasks/tools/install-kubectl/) |
| Helm            | v.3.3.0   | [Helm installation](https://helm.sh/docs/intro/install/)                                                                                                        |

Once all prerequisites are on the Linux host, follow the steps below to clone, build and deploy Karavi Authorization:

1. Clone the repository: `git clone https://github.com/dell/karavi-authorization.git`
1. In the karavi-authorization directory, run the following to build and deploy: `make build docker dist deploy`

## Testing Karavi Authorization

From the root directory where the repo was cloned, the unit tests can be executed as follows:

```
make test
```

This will also provide code coverage statistics for the various Go packages.

## Configuring CSI Driver with Authorization

Given a setup where Kubernetes, a storage system, CSI driver(s), and Karavi Authorization are deployed, follow the steps below to configure the CSI Drivers to work with Authorization sidecar:
<details>
  <summary> Configure Authorization host</summary>
  Run the following commands on the Authorization host

  ```console
  # Specify any desired name
  export RoleName=""
  export RoleQuota=""
  export TenantName=""

  # Specify info about Array1
  export Array1Type=""
  export Array1SystemID=""
  export Array1User=""
  export Array1Password=""
  export Array1Pool=""
  export Array1Endpoint=""
  
  # Specify info about Array2
  export Array2Type=""
  export Array2SystemID=""
  export Array2User=""
  export Array2Password=""
  export Array2Pool=""
  export Array2Endpoint=""

  # Specify IPs
  export DriverHostVMIP="" 
  export DriverHostVMPassword=""
  export DriverHostVMUser=""

  # Specify Authorization host address. NOTE: this is not the same as IP
  export AuthorizationHost=""

  echo === Creating Storage(s) ===
  # Add array1 to authorization
  karavictl storage create \
            --type ${Array1Type} \
            --endpoint  ${Array1Endpoint} \
            --system-id ${Array1SystemID} \
            --user ${Array1User} \
			      --password ${Array1Password} \
            --insecure
  
  # Add array2 to authorization
   karavictl storage create \
            --type ${Array2Type} \
            --endpoint  ${Array2Endpoint} \
            --system-id ${Array2SystemID} \
            --user ${Array2User} \
			      --password ${Array2Password} \
            --insecure
    
  echo === Creating Tenant ===
  karavictl tenant create -n $TenantName --insecure --addr "grpc.${AuthorizationHost}"

  echo === Creating Role ===
  karavictl role create \
           --role=${RoleName}=${Array1Type}=${Array1SystemID}=${Array1Pool}=${RoleQuota} \
           --role=${RoleName}=${Array2Type}=${Array2SystemID}=${Array2Pool}=${RoleQuota}   

  echo === === Binding Role ===
  karavictl rolebinding create --tenant $TenantName  --role $RoleName --insecure --addr "grpc.${AuthorizationHost}"

  echo === Generating token ===
  karavictl generate token --tenant $TenantName --insecure --addr "grpc.${AuthorizationHost}" | jq -r '.Token' > token.yaml

  echo === Copy token to Driver Host ===
  sshpass -p $DriverHostPassword scp token.yaml ${DriverHostVMUser}@{DriverHostVMIP}:/tmp/token.yaml 
  ```
  </details>
  <details>
    <summary> CSI Driver host </summary>
    Run the following commands on the CSI Driver host

   ```console
    # Specify Authorization host address. NOTE: this is not the same as IP
    export AuthorizationHost=""

    echo === Applying token token ===
    # It is assumed that array type powermax has the namespace "powermax" and powerflex has the namepace "vxflexos"
    kubectl apply -f /tmp/token.yaml -n powermax
    kubectl apply -f /tmp/token.yaml -n vxflexos

    echo === injecting sidecar in all CSI driver hosts that token has been applied to === 
    sudo curl -k https://${AuthorizationHost}/install | sh
    
    # NOTE: you can also query parameters("namespace" and "proxy-port") with the curl url if you desire a specific behavior.
    # 1) For instance, if you want to inject into just powermax, you can run
    #    sudo curl -k https://${AuthorizationHost}/install?namespace=powermax | sh
    # 2) If you want to specify the proxy-port for powermax to be 900001, you can run
    #    sudo curl -k https://${AuthorizationHost}/install?proxy-port=powermax:900001 | sh
    # 3) You can mix behaviors
    #    sudo curl -k https://${AuthorizationHost}/install?namespace=powermax&proxy-port=powermax:900001&namespace=vxflexos | sh
   ```
  </details>

### Test setup

To test the setup, follow the steps below:

- Create a StorageClass
- Create a PVC request from the StorageClass with any storage capacity less than the RoleQuota you specified during configuration
- Request a Pod to consume the PVC created above. If everything is well configured, the PVC will be bound to storage and the volume will be created on on the storage system.

You can also test failure case, by repeating the above steps but specify a quota larger than RoleQuota you specified. Conversely, when you request a Pod to use PVC, you'll get request is denied as PVC exceeds capacity and PV will be in a pending state.
