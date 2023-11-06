<!--
Copyright (c) 2021-2022 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
-->

# Dell Container Storage Modules (CSM) for Authorization

[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-v2.0%20adopted-ff69b4.svg)](https://github.com/dell/csm/blob/main/docs/CODE_OF_CONDUCT.md)
[![License](https://img.shields.io/github/license/dell/karavi-authorization)](LICENSE)
[![Docker Pulls](https://img.shields.io/docker/pulls/dellemc/csm-authorization-sidecar)](https://hub.docker.com/r/dellemc/csm-authorization-sidecar)
[![Go version](https://img.shields.io/github/go-mod/go-version/dell/karavi-authorization)](go.mod)
[![GitHub release (latest by date including pre-releases)](https://img.shields.io/github/v/release/dell/karavi-authorization?include_prereleases&label=latest&style=flat-square)](https://github.com/dell/karavi-authorization/releases/latest)

CSM for Authorization is part of the [CSM (Container Storage Modules)](https://github.com/dell/csm) open-source suite of Kubernetes storage enablers for Dell products. CSM for Authorization provides storage and Kubernetes administrators the ability to apply RBAC for Dell CSI Drivers. It does this by deploying a proxy between the CSI driver and the storage system to enforce role-based access and usage rules.

Storage administrators of compatible storage platforms will be able to apply quota and RBAC rules that instantly and automatically restrict cluster tenants usage of storage resources. Users of storage through CSM for Authorization do not need to have storage admin root credentials to access the storage system.

For documentation, please visit [Container Storage Modules documentation](https://dell.github.io/csm-docs/).

## Table of Contents

* [Code of Conduct](https://github.com/dell/csm/blob/main/docs/CODE_OF_CONDUCT.md)
* [Maintainer Guide](https://github.com/dell/csm/blob/main/docs/MAINTAINER_GUIDE.md)
* [Committer Guide](https://github.com/dell/csm/blob/main/docs/COMMITTER_GUIDE.md)
* [Contributing Guide](https://github.com/dell/csm/blob/main/docs/CONTRIBUTING.md)
* [List of Adopters](https://github.com/dell/csm/blob/main/docs/ADOPTERS.md)
* [Support](https://github.com/dell/csm/blob/main/docs/SUPPORT.md)
* [Security](https://github.com/dell/csm/blob/main/docs/SECURITY.md)
* [Project Structure](./docs/PROJECT_STRUCTURE.md)
* [About](#about)

## Building CSM for Authorization

If you wish to clone and build CSM for Authorization, a Linux host is required with the following installed:

| Component       | Version   | Additional Information                                                                                                                     |
| --------------- | --------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| Docker or Podman| v19+  ,v4.4.1+    | [Docker installation](https://docs.docker.com/engine/install/) , [Podman installation](https://podman.io/docs/installation)         |
| Golang          | v1.16    | [Golang installation](https://github.com/travis-ci/gimme)                                                                                                         |
| git             | latest    | [Git installation](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git)                                                                              |
| kubectl         | 1.17-1.19 | Ensure you copy the kubeconfig file from the Kubernetes cluster to the linux host. [kubectl installation](https://Kubernetes.io/docs/tasks/tools/install-kubectl/) |
| Helm            | v.3.3.0   | [Helm installation](https://helm.sh/docs/intro/install/)                                                                                                        |

Once all prerequisites are on the Linux host, follow the steps below to clone, build and deploy CSM for Authorization:

1. Clone the repository: `git clone https://github.com/dell/karavi-authorization.git`
2. In the karavi-authorization directory, run the following to build and deploy: `make build builder dist`

## Testing CSM for Authorization

From the root directory where the repo was cloned, the unit tests can be executed as follows:

```
make test
```

This will also provide code coverage statistics for the various Go packages.

### Test setup

To test the setup, follow the steps below:

- Create a StorageClass
- Create a PVC request from the StorageClass with any storage capacity less than the RoleQuota you specified during configuration
- Request a Pod to consume the PVC created above. If everything is well configured, the PVC will be bound to storage and the volume will be created on the storage system.

You can also test failure cases, by repeating the above steps but specify a quota larger than RoleQuota you specified. Conversely, when you request a Pod to use PVC, you'll get the request is denied as PVC exceeds capacity and PV will be in a pending state.

## Versioning

This project is adhering to [Semantic Versioning](https://semver.org/).

## About

Dell Container Storage Modules (CSM) is 100% open source and community-driven. All components are available
under [Apache 2 License](https://www.apache.org/licenses/LICENSE-2.0.html) on
GitHub.
