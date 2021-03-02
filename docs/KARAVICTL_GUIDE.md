# karavictl CLI Guide

karavictl is a command line interface (CLI) used to interact with and manage your Karavi Authorization deployment.
This document outlines all karavictl commands, their intended use, options that can be provided to alter their execution, and expected output from those commands.

If you feel that something is missing or unclear in this document, please open up an [issue](https://github.com/dell/karavi-authorization/issues).



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



---



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

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

```
$ karavictl cluster-info
NAME                   READY   UP-TO-DATE   AVAILABLE   AGE
github-auth-provider   1/1     1            1           59m
tenant-service         1/1     1            1           59m
redis-primary          1/1     1            1           59m
proxy-server           1/1     1            1           59m
redis-commander        1/1     1            1           59m
```



---



### karavictl inject

Inject the sidecar proxy into to a CSI driver pod

#### Synopsis

Injects the sidecar proxy into a CSI driver pod.

You can inject resources coming from stdin.

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

#### Examples:

Inject into an existing vxflexos CSI driver
```
kubectl get secrets,deployments,daemonsets -n vxflexos -o yaml \
   | karavictl inject --image-addr 10.0.0.1:5000/sidecar-proxy:latest --proxy-host 10.0.0.1 \
   | kubectl apply -f -
```

#### Output

```
$ kubectl get secrets,deployments,daemonsets -n vxflexos -o yaml \
| karavictl inject --image-addr 10.247.142.130:5000/sidecar-proxy:latest --proxy-host 10.247.142.130 \
| kubectl apply -f -

secret/karavi-authorization-config created
deployment.apps/vxflexos-controller configured
daemonset.apps/vxflexos-node configured
```


---



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



---



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



---



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



---



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



---



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



---



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



---



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



---



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



---



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



---



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

```
$ karavictl storage get --type powerflex --system-id 3000000000011111
{
  "User": "admin",
  "Password": "(omitted)",
  "Endpoint": "https://10.0.0.2",
  "Insecure": true
}
```



---



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

```
$ karavictl storage list

{
  "storage": {
    "powerflex": {
      "3000000000011111": {
        "Endpoint": "https://10.0.0.2",
        "Insecure": true,
        "Password": "(omitted)",
        "User": "admin"
      }
    }
  }
}
```



---



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
  -p, --password string        Password (default "****")
  -s, --system-id string   System identifier (default "systemid")
  -t, --type string        Type of storage system (default "powerflex")
  -u, --user string        Username (default "admin")
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

```
$ karavictl storage create --endpoint https://10.0.0.2 --insecure --system-id 3000000000011111 --type powerflex --user admin --password ********
```
On success, there will be no output. You may run `karavictl storage get --type <storage-system-type> --system-id <storage-system-id>` to confirm the creation occurred.


---



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

```
$ karavictl storage update --endpoint https://10.0.0.2 --insecure --system-id 3000000000011111 --type powerflex --user admin --password ********
```
On success, there will be no output. You may run `karavictl storage get --type <storage-system-type> --system-id <storage-system-id>` to confirm the update occurred.



---



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
```
$ karavictl storage delete --type powerflex --system-id 3000000000011111
```
On success, there will be no output. You may run `karavictl storage get --type <storage-system-type> --system-id <storage-system-id>` to confirm the deletion occurred.



## Tenant Commands



### karavictl tenant

Manage tenants

#### Synopsis

Management fortenants

```
karavictl tenant [flags]
```

#### Options

```
  -h, --help   help for tenant
```

#### Options inherited from parent commands

```
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

Outputs help text



---



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
  -n, --name string   Tenant name
```

#### Options inherited from parent commands

```
      --addr string     Address of the server (default "localhost:443")
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output
```
$ karavictl tenant create --name Alice
```
On success, there will be no output. You may run `karavictl tenant get --name <tenant-name>` to confirm the creation occurred.



---




### karavictl tenant get

Get a tenant resource within Karavi

#### Synopsis

Gets a tenant resource within Karavi

```
karavictl tenant get [flags]
```

#### Options

```
  -h, --help   help for create
  -n, --name string   Tenant name
```

#### Options inherited from parent commands

```
      --addr string     Address of the server (default "localhost:443")
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

```
$ karavictl tenant get --name Alice

{
  "name": "Alice"
}

```



---



### karavictl tenant list

Lists tenant resources within Karavi

#### Synopsis

Lists tenant resources within Karavi

```
karavictl tenant list [flags]
```

#### Options

```
  -h, --help   help for create
```

#### Options inherited from parent commands

```
      --addr string     Address of the server (default "localhost:443")
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output

```
$ karavictl tenant list

{
  "tenants": [
    {
      "name": "Alice"
    }
  ]
}

```



---



### karavictl tenant delete

Deletes a tenant resource within Karavi

#### Synopsis

Deletes a tenant resource within Karavi

```
karavictl tenant delete [flags]
```

#### Options

```
  -h, --help   help for create
  -n, --name string   Tenant name
```

#### Options inherited from parent commands

```
      --addr string     Address of the server (default "localhost:443")
      --config string   config file (default is $HOME/.karavictl.yaml)
```

#### Output
```
$ karavictl tenant delete --name Alice
```
On success, there will be no output. You may run `karavictl tenant get --name <tenant-name>` to confirm the deletion occurred.
