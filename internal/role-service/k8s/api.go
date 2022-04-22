package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"karavi-authorization/internal/role-service/roles"
	"strings"
	"sync"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientv1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// API holds data used to access the K8S API
type API struct {
	Client    kubernetes.Interface
	Lock      sync.Mutex
	Namespace string
}

const (
	ROLES_CONFIGMAP          = "common"
	ROLES_CONFIGMAP_DATA_KEY = "common.rego"
)

func (api *API) GetExistingRoles(ctx context.Context) (*roles.JSON, error) {
	api.Lock.Lock()
	defer api.Lock.Unlock()
	if api.Client == nil {
		err := ConnectFn(api)
		if err != nil {
			return nil, err
		}
	}

	common, err := api.Client.CoreV1().ConfigMaps(api.Namespace).Get(ctx, ROLES_CONFIGMAP, meta.GetOptions{})
	if err != nil {
		return nil, err
	}

	rolesRego := common.Data["common.rego"]
	if err != nil {
		return nil, err
	}

	rolesJSON := strings.Replace(rolesRego, "package karavi.common\ndefault roles = {}\nroles = ", "", 1)

	var existing roles.JSON
	dec := json.NewDecoder(strings.NewReader(rolesJSON))
	if err := dec.Decode(&existing); err != nil {
		return nil, fmt.Errorf("decoding roles json: %w", err)
	}

	return &existing, nil
}

// GetCSINodes will return a list of CSI nodes in the kubernetes cluster
func (api *API) UpdateRoles(ctx context.Context, roles *roles.JSON) error {
	api.Lock.Lock()
	defer api.Lock.Unlock()
	if api.Client == nil {
		err := ConnectFn(api)
		if err != nil {
			return err
		}
	}

	config, err := api.getApplyConfig(roles)
	if err != nil {
		return err
	}

	_, err = api.Client.CoreV1().ConfigMaps(api.Namespace).Apply(ctx, config, meta.ApplyOptions{})
	if err != nil {
		return err
	}
	return nil
}

func (api *API) getApplyConfig(roles *roles.JSON) (*clientv1.ConfigMapApplyConfiguration, error) {
	data, err := json.MarshalIndent(&roles, "", "  ")
	if err != nil {
		return nil, err
	}

	stdFormat := (`
package karavi.common
default roles = {}
roles = ` + string(data))

	config := clientv1.ConfigMap(ROLES_CONFIGMAP, api.Namespace)
	config.WithData(map[string]string{
		ROLES_CONFIGMAP_DATA_KEY: stdFormat,
	})

	return config, nil
}

// ConnectFn will connect the client to the k8s API
var ConnectFn = func(api *API) error {
	config, err := getConfig()
	if err != nil {
		return err
	}
	api.Client, err = NewConfigFn(config)
	if err != nil {
		return err
	}
	return nil
}

// InClusterConfigFn will return a valid configuration if we are running in a Pod on a kubernetes cluster
var InClusterConfigFn = func() (*rest.Config, error) {
	return rest.InClusterConfig()
}

// NewConfigFn will return a valid kubernetes.Clientset
var NewConfigFn = func(config *rest.Config) (*kubernetes.Clientset, error) {
	return kubernetes.NewForConfig(config)
}

func getConfig() (*rest.Config, error) {
	config, err := InClusterConfigFn()
	if err != nil {
		return nil, err
	}
	return config, nil
}
