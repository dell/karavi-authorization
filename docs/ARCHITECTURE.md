# Architecture

Karavi Authorization is designed as a service mesh solution and consists of many internal components that work together in concert to achieve its overall functionality.

This document provides an overview of the major components, including how they fit together and pointers to implementation details.

If you are a developer who is new to Karavi Authorization and wants to build a mental map of how it works, you're in the right place.

## Terminology

* **Service Mesh** - An infrastructure layer consisting of proxies that intercept and route requests between existing services.
* **CSI** - Acronym for the Container Storage Interface.
* **KAuthz** - Short form of Karavi Authorization, used in diagrams when appropriate.
* **Proxy (L7)** - A gateway between networked services that inspects request traffic.
* **Sidecar Proxy** - A service mesh proxy that runs alongside existing services, rather than within them.
* **Pod** - A Kubernetes abstraction for a set of related containers that are to be considered as one unit.

## Bird's Eye View

```
+-----------------------------------+                                                                                 
|   Kubernetes                      |                                                                                 
|                                   |                                                                                 
|  +---------+         +---------+  |            +------------+              +---------+                              
|  | CSI     |         | Sidecar |  |            | KAuthz     |              | Storage |                              
|  | Driver  |---------> Proxy   |---------------> Server     |--------------> Array   |                              
|  +---------+         +---------+  |            +------------+              +---------+                              
|                                   |                  ^                                                              
+-----------------------------------+                  |                                                              
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

Technically, a Kubernetes cluster is not required since a CSI Driver should not depend on it, but the remainder of this document assumes there is one.

### CSI Driver

A CSI Driver supports the Container Service Interface (CSI) specification. Dell EMC provides customers with CSI Drivers for its various storage arrays.
Karavi Authorization intends to support a majority, if not all, of these drivers.

A CSI Driver will typically be configured to communicate directly to its intended storage array and as such will be limited in using only the authentication
methods supported by the Storage Array itself, e.g. Basic authentication over TLS.

### Sidecar Proxy

The Karavi Authorization Sidecar Proxy is a sidecar container that gets "injected" into the CSI Driver's Pod. It acts as a proxy and forwards all requests to a
Karavi Authorization Server.

The CSI Driver section noted the limitation of a CSI Driver using Storage Array supported authentication methods only. By nature of being a proxy, the Karavi Authorization
Sidecar Proxy is able to override the Authorization HTTP header for outbound requests to use Bearer tokens.  Such tokens are managed by Karavi Authorization as will
be described later in this document.

### KAuthz Server

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

## Cross-Cutting Concerns

### JSON Web Tokens

## Observability
