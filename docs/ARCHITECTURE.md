# Architecture

Karavi Authorization is designed as a service mesh solution and consists of many internal components that work together in concert to achieve its overall functionality.

This document provides an overview of the major components, including how they fit together and pointers to implementation details.

If you are a developer who is new to Karavi Authorization and wants to build a mental map of how it works, you're in the right place.

## Terminology

* **Service Mesh** - An infrastructure layer consisting of proxies that intercept and route requests between existing services.
* **CSI** - Acronym for the Container Storage Interface.
* **Proxy (L7)** - A gateway between networked services that inspects request traffic.
* **Sidecar Proxy** - A service mesh proxy that runs alongside existing services, rather than within them.
* **Pod** - A Kubernetes abstraction for a set of related containers that are to be considered as one unit.

## Bird's Eye View

```
+-----------------------------------+                                                                                 
|   Kubernetes                      |                                                                                 
|                                   |                                                                                 
|  +---------+         +---------+  |            +---------------+                                            
|  | CSI     |         | Sidecar |  |            | Karavi        |              +---------+        
|  | Driver  |---------> Proxy   |---------------> Authorization |--------------> Storage |                              
|  +---------+         +---------+  |            |  Server       |              | Array   |                              
|                                   |            +---------------+              +---------+                              
+-----------------------------------+                  ^                                                              
                                                       |                                                              
                                                       |                                                              
                                                       |                                                              
                                                 +------------+                                                       
                                                 |  karavictl |                                                       
                                                 |  CLI       |                                                       
                                                 +------------+                                                 
```

NOTE: Arrows indicate request or connection initiation, not necessarily data flow direction.

The sections below will explain each component in the diagram.

### Kubernetes

The architecture assumes a Kubernetes cluster that intends to offer external storage to applications hosted therein.
The mechanism for managing this storage would utilize a CSI Driver.

**Architecture Invariant**: We assume there may be many Kubernetes clusters, potentially containing multiple CSI Drivers each with their own Sidecar Proxy.

### CSI Driver

A CSI Driver supports the Container Service Interface (CSI) specification. Dell EMC provides customers with CSI Drivers for its various storage arrays.
Karavi Authorization intends to support a majority, if not all, of these drivers.

A CSI Driver will typically be configured to communicate directly to its intended storage array and as such will be limited in using only the authentication
methods supported by the Storage Array itself, e.g. Basic authentication over TLS.

**Architecture Invariant**: We try to avoid having to make any code changes to CSI Driver when adding support for it.  Any CSI Driver should ideally not be aware that it is communicating to the Sidecar Proxy.

### Sidecar Proxy

The Karavi Authorization Sidecar Proxy is a sidecar container that gets "injected" into the CSI Driver's Pod. It acts as a proxy and forwards all requests to a
Karavi Authorization Server.

The CSI Driver section noted the limitation of a CSI Driver using Storage Array supported authentication methods only. By nature of being a proxy, the Karavi Authorization
Sidecar Proxy is able to override the Authorization HTTP header for outbound requests to use Bearer tokens.  Such tokens are managed by Karavi Authorization as will
be described later in this document.

### Karavi Authorization Server

The Karavi Authorization Server is, at its core, a Layer 7 proxy for intercepting traffic between a CSI Driver and a Storage Array.

Inbound requests are expected to originate from the Karavi Authorization Sidecar Proxy, for the following reasons:

* Processing a set of agreed upon HTTP headers (added by the Karavi Authorization Sidecar Proxy) to assist in routing traffic to the intended Storage Array.
* Inspection of Karavi-specific Authorization Bearer tokens.

### karavictl CLI

The *karavictl* CLI (Command Line Interface) application allows Storage Admins to manage and interact with a running Karavi Authorization Server.

Additionally, *karavictl* provides functionality for supporting the sidecar proxy injection mechanism mentioned above. Injection is discussed in more detail later
on in this document.

### Storage Array

A Storage Array is typically considered to be one of the various Dell EMC storage offerings, e.g. Dell EMC PowerFlex which is supported by Karavi Authorization
today.  Support for more Storage Arrays will come in the future.

## Project Structure

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

### `cmd/`

Location for files that use the `main` package and are intended to be built into executable binaries.

### `deploy/`

This is Karavi Authorization's directory for building a distributable deployment binary that can be used to install the solution even in network-restricted
environments.

Basic support for building an RPM is also supported, see `deploy/rpm/`.

### `docs/`

Documentation like the one you're reading right now.

### `internal/decision`

This is a convenience package for making requests into the Open Policy Agent (OPA) service.  The OPA service is used for making policy decisions.

### `internal/powerflex`

This package contains various supporting structs, functions to allow proxying requests into a Dell EMC PowerFlex, e.g.

* Caching a PowerFlex Basic login token and periodically refreshing it; this helps to maintain a single authenticated client per PowerFlex and avoids logging in on every request.
* Caching storage pool names against the storage pool ID; the CSI Driver for PowerFlex only uses the storage pool IDs in the request and Karavi Authorization requires the more human-friendly name.

### `internal/proxy`

This package contains HTTP handlers to facilitate the proxying of requests.  For example, the `DispatchHandler` ensures that an inbound HTTP request intended for the `ACME` storage array is dispatched to the ACME-related handlers.

### `internal/quota`

This package contains code for enforcing storage quota restrictions per Tenant. The implementation uses Redis as a data store due to its fast performance and simplicity.

### `internal/tenantsvc`

This package contains service logic for a gRPC service used to handle requests from `karavictl`.

### `internal/token`

This package contains supporting functions for token management in Karavi. If you're looking for how to generate JSON web tokens, look no further than here.

### `internal/web`

This package contains HTTP middlewares to help with adding cross-cutting concerns like logging, authentication and tracing.

### `pb/`

Directory for contain protobuf files.

### `policies/`

Directory for containing Rego files for the Open Policy Agent service.

## Authorization

Karavi Authorization intends to override the existing authorization methods between a CSI Driver and its Storage Array. This may be desirable for several reasons, if:

* The CSI Driver requires privileged login credentials (e.g. "root") in order to function.
* The Storage Array does not natively support the concept of RBAC and/or multi-tenancy.

This section of of the document will describe how Karavi Authorization provides a solution to these problems.

### Bearer Tokens

Karavi Authorization overrides any existing authorization mechanism with the use of JSON Web Tokens (JWTs).  The CSI Driver and Storage Array will not be aware of this taking place.

In the context of [RFC-6749](https://tools.ietf.org/html/rfc6749#section-1.5) there are two such JWTs that are used:

* Access token: a single token valid for a short period of time.
* Refresh token: a single token used to obtain access tokens.  Typically valid for a longer period of time.

Both tokens are opaque to the client, yet provide meaningful information to the server, specifically:

* The Tenant for whom the token is associated with.
* The Roles that are bound to the Tenant.

Tokens encode the following set of claims:

```
{
  "aud": "karavi",
  "exp": 1915585883,
  "iss": "com.dell.karavi",
  "sub": "karavi-tenant",
  "roles": "role-a,role-b,role-c",
  "group": "Tenant-1"
}
```

Both tokens are signed using a server-side secret preventing the risk of tampering by any client. For example, a bad-actor is unable to modify a token to give themselves a role that they should not have, at least without knowing the server-side secret.

The refresh approach is beneficial for the following reasons:

* Accidental exposure of an access token poses a lesser security concern, given the set expiration time is short (e.g. 30 seconds).
* Karavi Authorization Server can fully trust the access token without having to perform a database check on each request (doing so would nullify the benefits of using tokens in the first place).
* Karavi Authorization Server can defer Tenant checks at refresh time only, e.g. do not allow refresh if the Tenant's access has been revoked by a Storage Admin. There may be a short time window inbetween revocation and enforcement, depending on the access token's expiration time.

The following diagram shows the access and refresh tokens in play and how a valid access token is required for a request to be proxied to the intended Storage Array.

```
  +---------+                                           +---------------+
  |         |                                           |               |
  |         |                                           |               |       +----------+
  |         |--(A)------------ Access Token ----------->|               |------>|          |
  |         |                                           |     Karavi    |       |          |
  |         |<-(B)---------- Protected Resource --------| Authorization |<------|  Storage |
  | Sidecar |                                           |     Server    |       |   Array  |
  | Proxy   |--(C)------------ Access Token ----------->|               |       |          |
  |         |                                           |               |       |          |
  |         |<-(D)------ Invalid Token Error -----------|               |       |          |
  |         |                                           |               |       +----------+
  |         |                                           |               |
  |         |--(E)----------- Refresh Token ----------->|               |
  |         |            & Expired Access Token         |               |
  |         |<-(F)----------- Access Token -------------|               |
  +---------+                                           +---------------+
```

* A) CSI Driver makes a request to the Storage Array:
  * request is intercepted by the Sidecar Proxy to add the access token.
  * Karavi Authorization Server deems the access token valid.
  * Karavi Authorization Server permits the request to be proxied to the intended Storage Array.
* B) Storage Array response is sent back as expected.
* C) CSI Driver makes a request to the Storage Array:
  * request is intercepted by the Sidecar Proxy to add the access token.
  * Karavi Authorization Server deems the access token is invalid; it has since expired.
* D) Karavi Authorization Server responds with HTTP 401 Unauthorized.
* E) Sidecar Proxy requests a new access token by passing both refresh token and expired token.
* F) Karavi Authorization Server processes the request:
  * is the refresh token valid?
  * is the access token expired?
  * has the Tenant had access revoked?
  * a new access token is sent in response if the checks pass.

### Roles

So we know a token encodes both the identification of a Tenant and their Roles, but what's in a Role?

A role can be defined as follows:

* It has a name, e.g. "role-a".
* It can be bound to a Tenant
* It can be unbound from a Tenant.
* It determines access to zero or more storage pools and assigns a storage quota for each.
  * Quota represents the upper-limit of the total aggregation of used storage capacity for a Tenant's resources in a storage pool.
* It prevents ambiguity by identifying each storage pool in the form of *system-type:system-id:pool-name*.

Below is an example of how roles are represented internally in JSON:

```
{
  "Developer": {
    "system_types": {
      "powerflex": {
        "system_ids": {
          "542a2d5f5122210f": {
            "pool_quotas": {
              "bronze": 99000000
            }
          }
        }
      }
    }
  }
}
```

### Policy

Karavi Authorization leverages the [Open Policy Agent](https://www.openpolicyagent.org/) to use a policy-as-code approach to policy management. It stores a collection of policy files written in Rego language.  Each policy file defines a set of policy rules that form the basis of a policy decision. A policy decision is made by processing the inputs provided. For Karavi Authorization, the inputs are:

* The set of roles defined by the Storage Admin.
* The claims section of a validated JWT.
* The JSON payload of the storage request.

Given these inputs, many decisions can be made to answer questions like "Can Tenant X, with _these_ roles provision _this_ volume of size Y?".  The result of the policy decision will determine whether or not the request is proxied.

### Quota

## Cross-Cutting Concerns

This section documents the pieces of code that are general in nature and shared across multiple packages.

### Logging

Karavi Authorization uses the [Logrus](https://github.com/sirupsen/logrus) package when logging messages.

### Testing

## Observability

Both the Karavi Authorization Server and Sidecar Proxy are long-running processes, so it's important to understand what's going on inside. We use OpenTelemetry (otel) to help with that.

The following otel exporters are used:

* `go.opentelemetry.io/otel/exporters/metric/prometheus`
* `go.opentelemetry.io/otel/exporters/trace/zipkin`
* `go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp`

