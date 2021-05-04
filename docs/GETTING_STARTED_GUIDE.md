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

| Platforms | Dell CSI Driver |
| -------- | --------- |
| Storage Array | v3.0, v3.5 |
| Kubernetes | 1.18, 1,19, 1.20 | 
| OpenShift | 4.5, 4.6 |

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
    "keyFile": "path_to_host_cert_key_file",
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

4. To install the rpm package on the system, run the below command:

```
rpm -ivh <rpm_file_name>
```

5. After deployment, application data will be stored on the system under `/var/lib/rancher/k3s/storage/`.

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
  -e, --endpoint string    Endpoint of REST API gateway (default "https://10.0.0.1")
  -i, --insecure           Insecure skip verify
  -p, --pass string        Password (default "****")
  -s, --system-id string   System identifier (default "systemid")
  -t, --type string        Type of storage system (default "powerflex")
  -u, --user string        Username (default "admin")
```

#### Update Storage System

```
karavictl storage update [flags]

Flags:
  -e, --endpoint string    Endpoint of REST API gateway (default "https://10.0.0.1")
  -i, --insecure           Insecure skip verify
  -p, --pass string        Password (default "****")
  -s, --system-id string   System identifier (default "systemid")
  -t, --type string        Type of storage system (default "powerflex")
  -u, --user string        Username (default "admin")
```

#### Get Storage System

```
karavictl storage get [flags]

Flags:
  -t, --type string        Type of storage system (default "powerflex")
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
  -t, --type string        Type of storage system (default "powerflex")
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

Given a setup where Kubernetes, a storage system, a CSI driver, and Karavi Authorization are deployed, follow the steps below to configure the the CSI Driver with Authorization:
<details>
  <summary> Configure Authorization host</summary>
  Run the following commands on the Authorization host

  ```console
  # Specify any desired name
  RoleName=""
  RoleQuota=""
  TenantName=""

  # Specify all array information
  Type=""
  SystemID=""
  User=""
  Password=""
  Pool=""

  # Specify IPs
  DriverHostVMIP="" 
  DriverHostVMPassword=""
  DriverHostVMUser=""

  echo === Creating Storage ===
  karavictl storage create \
            --type $Type \
            --endpoint https://${DriverHostVMIP} \
            --system-id $SystemID \
            --user $User \
			      --password $Password \
            --insecure
    
  echo === Creating Tenant ===
  karavictl tenant create -n $TenantName

  echo === Creating Role ===
  karavictl role create --role=${RoleName}=${Type}=${SystemID}=${Pool}=${RoleQuota}   

  echo === === Binding Role ===
  karavictl rolebinding create --tenant $TenantName  --role $RoleName

  echo === Generating token ===
  token=$(karavictl generate token --tenant $TenantName)

  echo === Copy token to Driver Host ===
  sshpass -p $DriverHostPassword \
          ssh -o StrictHostKeyChecking=no ${DriverHostVMUser}@{DriverHostVMIP} \
          cat > /tmp/token.yaml << EOF ${token} EOF
  ```
  </details>
  <details>
    <summary> CSI Driver host </summary>
    Run the following commands on the CSI Driver host

   ```console
    DriverNameSpace=""
    AuthorizationHostIP=""

    echo === Applying token token ===
    kubectl apply -f /tmp/token.yaml -n $DriverNameSpace

    echo === injecting sidecar in all CSI driver hosts === 
    sudo curl -k https://${AuthorizationHostIP}/install | sh
   ```
  </details>

### Test setup

To test the setup, follow the steps below:

- Create a StorageClass
- Create a PVC request from the StorageClass with any storage capacity less than the RoleQuota you specified during configuration
- Request a Pod to consume the PVC created above. If everything is well configured, the PVC will be bound to storage and the volume will be created on on the storage system.

You can also test failure case, buy repeating the above steps but specify a quota larger than RoleQuota you specified. Conversely, when you request a Pod to use PVC, you'll get request is denied as PVC exceeds capacity and pv will be in a pending state.
