package web

import (
	"fmt"
	"net/http"
)

// DefaultSidecarProxyAddr is the default location where a client can
// download the sidecar proxy container image.
var DefaultSidecarProxyAddr = "10.0.0.1:5000/sidecar-proxy:latest"

// InstallScriptFormat is a format string containing a small shell script
// that a client can download in order to inject the sidecar proxy into a
// running CSI driver.
//
// E.g. `curl https://10.0.0.1/install | sh
var InstallScriptFormat = `
kubectl get secrets,deployments,daemonsets -n vxflexos -o yaml \
  | karavictl inject \
  --image-addr %s \
  --proxy-host %s \
  | kubectl apply -f -
kubectl rollout status -n vxflexos deploy/vxflexos-controller
kubectl rollout status -n vxflexos ds/vxflexos-node`

// ClientInstallHandler returns a handler that will serve up an installer
// script to requesting clients.
func ClientInstallHandler(imageAddr string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		fmt.Fprintf(w, InstallScriptFormat, imageAddr, host)
	})
}
