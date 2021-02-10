<!--
Copyright (c) 2021 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
-->

# Karavi Authorization

[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-v2.0%20adopted-ff69b4.svg)](docs/CODE_OF_CONDUCT.md)
[![License](https://img.shields.io/github/license/dell/karavi-authorization)](LICENSE)
[![Go version](https://img.shields.io/github/go-mod/go-version/dell/karavi-authorization)](go.mod)
[![GitHub release (latest by date including pre-releases)](https://img.shields.io/github/v/release/dell/karavi-authorization?include_prereleases&label=latest&style=flat-square)](https://github.com/dell/karavi-authorization/releases/latest)

This project provides storage and Kubernetes administrators the ability to apply RBAC for Dell EMC CSI Drivers by deploying a proxy between the CSI driver and the storage system to enforce role based access and usage rules. 

Storage administrators of compatible storage platforms will be able to apply quota and RBAC rules that instantly and automatically restrict cluster tenants usage of storage resources. Users of storage through Karavi Authorization will also not need to have storage admin root credentials to access the storage system.

Kubernetes administrators will also have an interface to create, delete, and manage roles/groups that storage rules may be applied to. Administrators and/or users may then generate authentication tokens that may be used by tenants to use storage with proper access policies being automatically enforced.

**Warning: Karavi Authorization is in pre-alpha status. Information is this guide is subject to change.**

## Table of Contents
- [Code of Conduct](./docs/CODE_OF_CONDUCT.md)
- [Getting Started Guide](./docs/GETTING_STARTED_GUIDE.md)
- [Branching Strategy](./docs/BRANCHING.md)
- [Contributing Guide](./docs/CONTRIBUTING.md)
- [Maintainers](./docs/MAINTAINERS.md)
- [About](#about)

## Getting Started

### Installation

```
kubectl apply -f deploy/deployment.yaml
```

Check your CSI Driver for how to enable *proxy mode*.

Once enabled, the IP address or DNS name of the Karavi Authorization instance should be used as the gateway.

Tokens (JWTs) will be required for authorized access to Karavi Authorization.

The following DNS hostnames are expected to resolve to the IP address of Karavi Authorization:

* admin.gatekeeper.com
* grpc.gatekeeper.com

### Requesting Tokens

```
go install ./cmd/auth-client
auth-client | kubectl apply -f -
```

Follow the directions in order to authenticate yourself with Karavi Authorization.

Once authentication has completed successfully, a Kubernetes secret will be applied to the current namespace.

This secret contains token values required for authorized access to Karavi Authorization.

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

## Support

Don’t hesitate to ask! Contact the team and community on the [mailing lists](https://group) or on [slack](https://<slack instance>.slack.com/) if you need any help.
Open an issue if you found a bug on [GitHub
Issues](https://eos2git.cec.lab.emc.com/DevCon/NewProjectTemplate/issues).

## About

Karavi Authorization is 100% open source and community-driven. All components are available
under [Apache 2 License](https://www.apache.org/licenses/LICENSE-2.0.html) on
GitHub.
