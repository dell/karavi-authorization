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

| Feature | PowerFlex |
| ------- | --------- |
| Enforcing quota limits| Yes |
| Shielding storage admin credentials | Yes |
| LDAP Support | No |

## Supported Platforms

The following matrix provides a list of all supported versions for each Dell EMC Storage product.

| Platforms | PowerFlex |
| -------- | --------- |
| Storage Array | v3.0, v3.5 |
| Kubernetes | 1.17, 1,18, 1.19 |
| Openshift | 4.5, 4.6 |

## CSI Drivers

Karavi Authorization supports the following CSI drivers and versions.

| Storage Array | CSI Driver | Supported Versions |
| ------------- | ---------- | ------------------ |
| PowerFlex | [csi-powerflex](https://github.com/dell/csi-powerflex) | v1.4.0 |

## Deploying Karavi Authorization

A single binary installer can be built and executed to perform the deployment of Karavi Authorization.

Use the following Makefile targets to build the installer:

```
make dist build-installer rpm
```

The `build-installer` step creates a binary at `bin/deploy` and embeds all components required for installation. The `rpm` step generates an RPM package and stores it at `deploy/rpmbuild/RPMS/x86_64/`.
This allows for Karavi Authorization to be installed in network-restricted environments.

A Storage Administrator can execute the installer or rpm package as a root user or via `sudo`.

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
karavictl generate token -t <tenant>
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

#### Sample file defining storage access roles:

```
"CSIGold":[
    {
        "storage_system_id":"system_id1",
        "pool_quotas":[
            {
            "pool":"gold",
            "quota":32000000
            }
        ]
    }
],
"CSISilver":[
    {
        "storage_system_id":"system_id2",
        "pool_quotas":[
            {
            "pool":"silver",
            "quota":16000000
            }
        ]
    }
]
```

#### Creating Storage Access Roles

```
karavictl roles create [file]

Flags:
  -f, --from-file string   role data from a file
  -h, --help               help for create
```

#### Updating Storage Access Roles

```
karavictl roles update [flags]

Flags:
  -f, --from-file string   role data from a file
  -h, --help               help for create
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
| Golang          | v1.15+    | [Golang installation](https://github.com/travis-ci/gimme)                                                                                                         |
| git             | latest    | [Git installation](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git)                                                                              |
| kubectl         | 1.17-1.19 | Ensure you copy the kubeconfig file from the Kubernetes cluster to the linux host. [kubectl installation](https://kubernetes.io/docs/tasks/tools/install-kubectl/) |
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
