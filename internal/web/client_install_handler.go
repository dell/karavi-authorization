package web

import (
	"fmt"
	"karavi-authorization/internal/token"
	"net/http"
	"time"
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
  --guest-access-token %s \
  --guest-refresh-token %s \
  | kubectl apply -f -
kubectl rollout status -n vxflexos deploy/vxflexos-controller
kubectl rollout status -n vxflexos ds/vxflexos-node`

const Guest = "Guest" // Guest is used for the Guest tenant and role name.

// ClientInstallHandler returns a handler that will serve up an installer
// script to requesting clients.
func ClientInstallHandler(imageAddr, jwtSigningSecret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host := r.Host
		tp, err := token.Create(token.Config{
			Tenant:            Guest,
			Roles:             []string{Guest},
			JWTSigningSecret:  jwtSigningSecret,
			AccessExpiration:  24 * time.Hour,
			RefreshExpiration: 24 * time.Hour,
		})
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		fmt.Fprintf(w, InstallScriptFormat,
			imageAddr,
			host,
			tp.Access,
			tp.Refresh)
	})
}
