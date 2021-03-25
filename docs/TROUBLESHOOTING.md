# Troubleshooting

## Table of Contents
[`karavictl inject` leaves vxflexos-controller in `Pending` state](#karavictl-inject-leaves-vxflexos-controller-in-pending-state)

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

#### Resolution
To resolve this issue, we need to temporarily reduce the number of replicas that the Vxflexos driver deployment is using.

1. Edit the deployment
```
$ kubectl edit -n <vxflexos-namespace> deploy/vxflexos-controller
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
