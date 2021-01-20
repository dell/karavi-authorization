<!--
Copyright (c) 2021 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
-->

# Karavi Authorization

The open-source solution that provides Kubernetes administrators the ability to apply RBAC for Dell EMC CSI Drivers.

[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-v2.0%20adopted-ff69b4.svg)](CODE_OF_CONDUCT.md) 
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)
[![Releases](https://img.shields.io/badge/Releases-green.svg)](https://eos2git.cec.lab.emc.com/DevCon/NewProjectTemplate/releases)

**Warning: Karavi Authorization is in pre-alpha status. Information is this guide is subject to change.**

## Supported Dell EMC Products

| Dell EMC Product | Version |
| ---------------- | ------- |
| VxFlex OS        | 3.0/3.5 |

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
