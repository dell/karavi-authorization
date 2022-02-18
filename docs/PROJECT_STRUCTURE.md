# Project Structure

This section talks briefly about various important directories and data structures.

Below is the directory structure at the time of this writing.

```
.
├── bin
├── cmd
│   ├── karavictl
│   ├── proxy-server
│   ├── sidecar-proxy
│   └── tenant-service
├── deploy
│   ├── dist
│   ├── rpm
│   └── testdata
├── docs
├── internal
│   ├── decision
│   ├── powerflex
│   ├── proxy
│   ├── quota
│   ├── tenantsvc
│   ├── token
│   └── web
├── pb
├── policies
├── scripts
└── tests
```

## `cmd/`

Location for files that use the `main` package and are intended to be built into executable binaries.

## `deploy/`

This is Karavi Authorization's directory for building a distributable deployment binary that can be used to install the solution even in network-restricted
environments.

Basic support for building an RPM is also supported, see `deploy/rpm/`.

## `docs/`

Documentation like the one you're reading right now.

## `internal/decision`

This is a convenience package for making requests into the Open Policy Agent (OPA) service.  The OPA service is used for making policy decisions.

## `internal/powerflex`

This package contains various supporting structs, functions to allow proxying requests into a Dell PowerFlex, e.g.

* Caching a PowerFlex Basic login token and periodically refreshing it; this helps to maintain a single authenticated client per PowerFlex and avoids logging in on every request.
* Caching storage pool names against the storage pool ID; the CSI Driver for PowerFlex only uses the storage pool IDs in the request and Karavi Authorization requires the more human-friendly name.

## `internal/proxy`

This package contains HTTP handlers to facilitate the proxying of requests.  For example, the `DispatchHandler` ensures that an inbound HTTP request intended for the `ACME` storage array is dispatched to the ACME-related handlers.

## `internal/quota`

This package contains code for enforcing storage quota restrictions per Tenant. The implementation uses Redis as a data store due to its fast performance and simplicity.

## `internal/tenantsvc`

This package contains service logic for a gRPC service used to handle requests from `karavictl`.

## `internal/token`

This package contains supporting functions for token management in Karavi. If you're looking for how to generate JSON web tokens, look no further than here.

## `internal/web`

This package contains HTTP middlewares to help with adding cross-cutting concerns like logging, authentication and tracing.

## `pb/`

Directory for containing protobuf files.

## `policies/`

Directory for containing Rego files for the Open Policy Agent service.
