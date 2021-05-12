# Troubleshooting

## Table of Contents
- `karavictl inject` leaves the following arrays controllers in pending states
- [vxflexos-controller in `Pending` state](#karavictl-inject-leaves-vxflexos-controller-in-pending-state)
- [powermax-controller in `Pending` state](#karavictl-inject-leaves-powermax-controller-in-pending-state)

---

### `karavictl inject` leaves vxflexos-controller in `Pending` state
This situation may occur when the number of Vxflexos controller pods that are deployed is equal to the number of schedulable nodes.
```
$ kubectl get pods -n vxflexos                                                                  

NAME                                   READY   STATUS    RESTARTS   AGE
vxflexos-controller-696cc5945f-4t94d   0/6     Pending   0          3m2s
vxflexos-controller-75cdcbc5db-k25zx   5/5     Running   0          3m41s
vxflexos-controller-75cdcbc5db-nkxqh   5/5     Running   0          3m42s
vxflexos-node-mjc74                    3/3     Running   0          2m44s
vxflexos-node-zgswp                    3/3     Running   0          2m44s
```

### `karavictl inject` leaves powermax-controller in `Pending` state
This situation may occur when the number of Powermax controller pods that are deployed is equal to the number of schedulable nodes.
```
$ kubectl get pods -n powermax --show-labels                                                                 

powermax-controller-58d8779f5d-v7t56   0/6     Pending   0          25s   name=powermax-controller,pod-template-hash=58d8779f5d
powermax-controller-78f749847-jqphx    5/5     Running   0          10m   name=powermax-controller,pod-template-hash=78f749847
powermax-controller-78f749847-w6vp5    5/5     Running   0          10m   name=powermax-controller,pod-template-hash=78f749847
powermax-node-gx5pk                    3/3     Running   0          21s   app=powermax-node,controller-revision-hash=5c4fdf478f,pod-template-generation=3
powermax-node-k5gwc                    3/3     Running   0          17s   app=powermax-node,controller-revision-hash=5c4fdf478f,pod-template-generation=3
```

#### Resolution
To resolve this issue, we need to temporarily reduce the number of replicas that the the driver deployment is using.

1. Edit the deployment
```
$ kubectl edit -n <namespace> deploy/<vxflexos/powermax>-controller
```

2. Find `replicas` under the `spec` section of the deployment manifest.
3. Reduce the number of `replicas` by 1
4. Save the file
5. Confirm that the updated controller pods have been deployed
```
$ kubectl get pods -n vxflexos                                                                  

NAME                                   READY   STATUS    RESTARTS   AGE
vxflexos-controller-696cc5945f-4t94d   6/6     Running   0          4m41s
vxflexos-node-mjc74                    3/3     Running   0          3m44s
vxflexos-node-zgswp                    3/3     Running   0          3m44s
```
or
```
$ kubectl get pods -n powermax --show-labels
NAME                                   READY   STATUS    RESTARTS   AGE     LABELS
powermax-controller-58d8779f5d-cqx8d   6/6     Running   0          22s     name=powermax-controller,pod-template-hash=58d8779f5d
powermax-controller-58d8779f5d-v7t56   6/6     Running   22         8m7s    name=powermax-controller,pod-template-hash=58d8779f5d
powermax-node-gx5pk                    3/3     Running   3          8m3s    app=powermax-node,controller-revision-hash=5c4fdf478f,pod-template-generation=3
powermax-node-k5gwc                    3/3     Running   3          7m59s   app=powermax-node,controller-revision-hash=5c4fdf478f,pod-template-generation=3
```

6. Edit the deployment again
7. Find `replicas` under the `spec` section of the deployment manifest.
8. Increase the number of `replicas` by 1
9. Save the file
10. Confirm that the updated controller pods have been deployed
```
$ kubectl get pods -n vxflexos                                                                  

NAME                                   READY   STATUS    RESTARTS   AGE
vxflexos-controller-696cc5945f-4t94d   6/6     Running   0          5m41s
vxflexos-controller-696cc5945f-6xxhb   6/6     Running   0          5m41s
vxflexos-node-mjc74                    3/3     Running   0          4m44s
vxflexos-node-zgswp                    3/3     Running   0          4m44s
```
or
```
$ kubectl get pods -n powermax --show-labels
NAME                                   READY   STATUS    RESTARTS   AGE     LABELS
powermax-controller-58d8779f5d-cqx8d   6/6     Running   0          22s     name=powermax-controller,pod-template-hash=58d8779f5d
powermax-controller-58d8779f5d-v7t56   6/6     Running   22         8m7s    name=powermax-controller,pod-template-hash=58d8779f5d
powermax-node-gx5pk                    3/3     Running   3          8m3s    app=powermax-node,controller-revision-hash=5c4fdf478f,pod-template-generation=3
powermax-node-k5gwc                    3/3     Running   3          7m59s   app=powermax-node,controller-revision-hash=5c4fdf478f,pod-template-generation=3
```