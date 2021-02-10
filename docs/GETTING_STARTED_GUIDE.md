<!--
Copyright (c) 2021 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
-->
# Getting Started Guide

This project provides storage and Kubernetes administrators the ability to apply RBAC for Dell EMC CSI Drivers. A proxy is deployed between the CSI driver and the storage system which will enforce role based rules defined by the administrator to determine which requests should be forwarded to the storage system and which requests should be denied.

Storage administrators of compatible storage platforms will have a simple to use interface to apply quota and RBAC rules that are applied isntantly and automatically to restrict cluster tenants usage of storage resources. Users of storage through Karavi Authorization will also not need to have storage admin root credentials to access the storage system.

Kubernetes administrators will also have a simple to use interface to create, delete, and manage roles/groups that storage rules may be applied to. Administrators or users may generate authentication tokens that may be used by tenants to access storage with proper access policies being automatically enforced.


## Karavi Authorization Capabilities

| Feature | PowerFlex |
| ------- | --------- |
| Enforcing quota limits| Yes |
| Sheilding storage admin credentials | Yes |
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
| PowerFlex | [csi-powerflex](https://github.com/dell/csi-powerflex) | v1.1.5, 1.2.0, 1.2.1 |

## Deploying Karavi Authorization

### CSI Driver Proxy Mode

Check your CSI Driver documentation for how to enable **proxy mode**.

Once enabled, the IP address or DNS name of the Karavi Authorization instance should be used as the gateway for the driver.

### DNS

To properly use Karavi Authorization, the following DNS hostnames are expected to resolve to the IP address of the Storage Keeper:
- admin.gatekeeper.com
- grpc.gatekeeper.com

### Installation

To deploy Karavi Authorization simply fork this repository and create the Kubernetes resources using `kubectl`:
```console
$ git clone https://github.com/dell/karavi-authorization.git
$ kubectl apply -f deploy/deployment.yaml
```

## Usage

### Requesting Tokens

An `auth-client` console application is included with the Karavi Authorization repository. Install it using `go` and use it to request a token.
```console
$ go install ./cmd/auth-client
$ auth-client | kubectl apply -f -
```
Follow the directions presented in the console in order to authenticate yourself with Karavi Authorization.

Once authentication has completed successfully, a Kubernetes secret will be applied to the current namespace. This secret contains token values required for authorized access to Karavi Authorization.

### Revoking Access

Currently, the process for revoking access to one or more tenants is as follows:

1. Browse to the Redis Commander Web-UI at https://admin.gatekeeper.com/.
2. If not already set, create a Set with key name *tenant:deny*.
3. Add the tenant name to the *tenant:deny* set.

### Viewing Tenants

Currently, the process for viewing tenants is as follows:

1. Browse to the Redis Commander Web-UI at https://admin.gatekeeper.com/.
2. Inspect Hash key values with key names prefixed with *tenant:*.

The available information shows:

* refresh_sha: Base64 encoded SHA256 of the tenants' refresh token.
* refresh_isa: Unix time of when the refresh token was issued.
* refresh_count: Number of times this refresh token has been used for refreshing access tokens.
