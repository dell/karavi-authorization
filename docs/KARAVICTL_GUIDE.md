# karavictl CLI Guide

karavictl is a command line interface (CLI) used to interact with and manage your Karavi Authorization deployment.
This document outlines all karavictl commands, their intended use, options that can be provided to alter their execution, and expected output from those commands.

If you feel that something is missing or unclear in this document, please open up an [issue](https://github.com/dell/karavi-authorization/issues).

---

## General Commands



### karavictl

karavictl is used to interact with karavi server

#### Synopsis

karavictl provides security, RBAC, and quota limits for accessing Dell EMC
storage products from Kubernetes clusters

#### Options

```
      --config string   config file (default is $HOME/.karavictl.yaml)
  -h, --help            help for karavictl
  -t, --toggle          Help message for toggle
```

#### Output

Outputs help text

### karavictl cluster-info

Display the state of resources within the cluster

#### Synopsis

Prints table of resources within the cluster, including their readiness

```
karavictl cluster-info [flags]
```

#### Options

```
  -h, --help    help for cluster-info
  -w, --watch   Watch for changes
```

### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

TODO



### karavictl inject

Inject the sidecar proxy into to a CSI driver pod

#### Synopsis

Injects the sidecar proxy into a CSI driver pod.

You can inject resources coming from stdin.

Usage:
karavictl inject [flags]

Examples:
##### Inject into an existing vxflexos CSI driver
kubectl get secrets,deployments,daemonsets -n vxflexos -o yaml \
  | karavictl inject --image-addr 10.0.0.1:5000/sidecar-proxy:latest --proxy-host 10.0.0.1 \
  | kubectl apply -f -

```
karavictl inject [flags]
```

#### Options

```
  -h, --help                help for inject
      --image-addr string   Help message for image-addr
      --proxy-host string   Help message for proxy-host
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

TODO



### karavictl generate

Generate resources for use with Karavi

#### Synopsis

Generates resources for use with Karavi

```
karavictl generate [flags]
```

#### Options

```
  -h, --help   help for generate
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

Outputs help text



### karavictl generate token

Generate tokens

#### Synopsis

Generate tokens for use with the CSI Driver when in proxy mode
The tokens are output as a Kubernetes Secret resource, so the results may
be piped directly to kubectl:

Example: karavictl generate token | kubectl apply -f -

```
karavictl generate token [flags]
```

#### Options

```
      --addr string            host:port address (default "grpc.gatekeeper.cluster:443")
      --from-config string     File providing self-generated token information
  -h, --help                   help for token
      --namespace string       Namespace of the CSI driver (default "vxflexos")
      --shared-secret string   Shared secret for token signing
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

TODO

---

## Role Commands



### karavictl role

Manage roles

#### Synopsis

Manage roles

```
karavictl role [flags]
```

#### Options

```
  -h, --help   help for role
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

Outputs help text



### karavictl role get

Get role

#### Synopsis

Get role

```
karavictl role get [flags]
```

#### Options

```
  -h, --help   help for get
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

TODO



### karavictl role list

List roles

#### Synopsis

List roles

```
karavictl role list [flags]
```

#### Options

```
  -h, --help   help for list
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

TODO



### karavictl role create

Create one or more Karavi roles

#### Synopsis

Creates one or more Karavi roles

```
karavictl role create [flags]
```

#### Options

```
  -f, --from-file string   role data from a file
  -h, --help               help for create
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

TODO



### karavictl role update

Update one or more Karavi roles

#### Synopsis

Updates one or more Karavi roles

```
karavictl role update [flags]
```

#### Options

```
  -f, --from-file string   role data from a file
  -h, --help               help for update
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

TODO



### karavictl role delete

Delete role

#### Synopsis

Delete role

```
karavictl role delete [flags]
```

#### Options

```
  -h, --help   help for delete
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

TODO



### karavictl rolebinding

Manage role bindings

#### Synopsis

Management for role bindings

```
karavictl rolebinding [flags]
```

#### Options

```
  -h, --help   help for rolebinding
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

Outputs help text


### karavictl rolebinding create

Create a rolebinding between role and tenant

#### Synopsis

Creates a rolebinding between role and tenant

```
karavictl rolebinding create [flags]
```

#### Options

```
  -h, --help   help for create
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

TODO


---

## Storage Commands



### karavictl storage

Manage storage systems

#### Synopsis

Manages storage systems

```
karavictl storage [flags]
```

#### Options

```
  -h, --help   help for storage
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

Outputs help text



### karavictl storage get

Get details on a registered storage system.

#### Synopsis

Gets details on a registered storage system.

```
karavictl storage get [flags]
```

#### Options

```
  -h, --help               help for get
  -s, --system-id string   System identifier (default "systemid")
  -t, --type string        Type of storage system (default "powerflex")
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

TODO



### karavictl storage list

List registered storage systems.

#### Synopsis

Lists registered storage systems.

```
karavictl storage list [flags]
```

#### Options

```
  -h, --help   help for list
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

TODO



### karavictl storage create

Create and register a storage system.

#### Synopsis

Creates and registers a storage system.

```
karavictl storage create [flags]
```

#### Options

```
  -e, --endpoint string    Endpoint of REST API gateway (default "https://10.0.0.1")
  -h, --help               help for create
  -i, --insecure           Insecure skip verify
  -p, --pass string        Password (default "****")
  -s, --system-id string   System identifier (default "systemid")
  -t, --type string        Type of storage system (default "powerflex")
  -u, --user string        Username (default "admin")
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

TODO



### karavictl storage update

Update a registered storage system.

#### Synopsis

Updates a registered storage system.

```
karavictl storage update [flags]
```

#### Options

```
  -e, --endpoint string    Endpoint of REST API gateway (default "https://10.0.0.1")
  -h, --help               help for update
  -i, --insecure           Insecure skip verify
  -p, --pass string        Password (default "****")
  -s, --system-id string   System identifier (default "systemid")
  -t, --type string        Type of storage system (default "powerflex")
  -u, --user string        Username (default "admin")
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

TODO



### karavictl storage delete

Delete a registered storage system.

#### Synopsis

Deletes a registered storage system.

```
karavictl storage delete [flags]
```

#### Options

```
  -h, --help               help for delete
  -s, --system-id string   System identifier (default "systemid")
  -t, --type string        Type of storage system (default "powerflex")
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

TODO



---

## Tenant Commands



## karavictl tenant

Manage tenants

### Synopsis

Management fortenants

```
karavictl tenant [flags]
```

### Options

```
  -h, --help   help for tenant
```

### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

Outputs help text



### karavictl tenant create

Create a tenant resource within Karavi

#### Synopsis

Creates a tenant resource within Karavi

```
karavictl tenant create [flags]
```

#### Options

```
  -h, --help   help for create
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

TODO
