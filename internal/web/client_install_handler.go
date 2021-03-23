package web

import (
	"fmt"
	"karavi-authorization/internal/token"
	"net/http"
	"strings"
	"time"
)

// DefaultSidecarProxyAddr is the default location where a client can
// download the sidecar proxy container image.
var DefaultSidecarProxyAddr = "10.0.0.1:5000/sidecar-proxy:latest"

// Guest is used for the Guest tenant and role name.
const Guest = "Guest"

// ClientInstallHandler returns a handler that will serve up an installer
// script to requesting clients.
func ClientInstallHandler(imageAddr, jwtSigningSecret, rootCA string, insecure bool) http.Handler {
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

		var sb strings.Builder

		fmt.Fprintln(&sb, "kubectl get secrets,deployments,daemonsets -n vxflexos -o yaml")
		fmt.Fprintln(&sb, " | karavictl inject")
		fmt.Fprintf(&sb, " --image-addr %s\n", imageAddr)
		fmt.Fprintf(&sb, " --proxy-host %s\n", host)
		fmt.Fprintf(&sb, " --insecure=%v\n", insecure)
		if rootCA != "" {
			fmt.Fprintf(&sb, " --root-certificate %s\n", rootCA)
		}
		fmt.Fprintf(&sb, " --guest-access-token %s\n", tp.Access)
		fmt.Fprintf(&sb, " --guest-refresh-token %s\n", tp.Refresh)
		fmt.Fprintln(&sb, " | kubectl apply -f -")
		fmt.Fprintln(&sb, "kubectl rollout status -n vxflexos deploy/vxflexos-controller")
		fmt.Fprintln(&sb, "kubectl rollout status -n vxflexos ds/vxflexos-node")

		fmt.Fprintf(w, sb.String())
	})
}
