package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/internal/types"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
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
	Log       *logrus.Entry
}

const (
	RolesConfigMap        = "common"
	RolesConfigMapDataKey = "common.rego"

	StorageSecret                 = "karavi-storage-secret"
	StorageSecretDataKey          = "storage-systems.yaml"
	StorageSecretDataStorageField = "storage"
)

// GetConfiguredRoles returns a wrapper for operations on a collection of role instances
func (api *API) GetConfiguredRoles(ctx context.Context) (*roles.JSON, error) {
	api.Lock.Lock()
	defer api.Lock.Unlock()
	if api.Client == nil {
		err := ConnectFn(api)
		if err != nil {
			return nil, err
		}
	}

	api.Log.WithFields(logrus.Fields{
		"ConfigMap":        RolesConfigMap,
		"ConfigMapDataKey": RolesConfigMapDataKey,
	}).Debug("Getting configMap containing configured roles")

	common, err := api.Client.CoreV1().ConfigMaps(api.Namespace).Get(ctx, RolesConfigMap, meta.GetOptions{})
	if err != nil {
		return nil, err
	}

	rolesRego := common.Data[RolesConfigMapDataKey]
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

// UpdateRoles updates the configured roles with supplied collection of role instances
func (api *API) UpdateRoles(ctx context.Context, roles *roles.JSON) error {
	api.Lock.Lock()
	defer api.Lock.Unlock()
	if api.Client == nil {
		err := ConnectFn(api)
		if err != nil {
			return err
		}
	}

	var roleNamesBuilder strings.Builder
	for i, role := range roles.Instances() {
		if i == len(roles.Instances())-1 {
			roleNamesBuilder.Write([]byte(role.Name))
		} else {
			roleNamesBuilder.Write([]byte(fmt.Sprintf("%s,", role.Name)))
		}
	}

	api.Log.WithFields(logrus.Fields{
		"ConfigMap":        RolesConfigMap,
		"ConfigMapDataKey": RolesConfigMapDataKey,
		"RoleNames":        roleNamesBuilder.String(),
	}).Debug("Applying roles to configMap containing configured roles")

	config, err := api.getApplyConfig(roles)
	if err != nil {
		return err
	}

	_, err = api.Client.CoreV1().ConfigMaps(api.Namespace).Apply(ctx, config, meta.ApplyOptions{FieldManager: "application/apply-patch", Force: true})
	if err != nil {
		return err
	}
	return nil
}

// GetConfiguredStorage returns the configured storage systems
func (api *API) GetConfiguredStorage(ctx context.Context) (types.Storage, error) {
	api.Lock.Lock()
	defer api.Lock.Unlock()
	if api.Client == nil {
		err := ConnectFn(api)
		if err != nil {
			return nil, err
		}
	}

	api.Log.WithFields(logrus.Fields{
		"Secret":        StorageSecret,
		"SecretDataKey": StorageSecretDataKey,
	}).Debug("Getting secret containing configured storage systems")

	storageSecret, err := api.Client.CoreV1().Secrets(api.Namespace).Get(ctx, StorageSecret, meta.GetOptions{})
	if err != nil {
		return nil, err
	}

	var data []byte
	if v, ok := storageSecret.Data[StorageSecretDataKey]; ok {
		data = v
	} else {
		return nil, fmt.Errorf("%s data key not found in secret %s", StorageSecretDataKey, StorageSecret)
	}

	var storage map[string]types.Storage
	if err := yaml.Unmarshal(data, &storage); err != nil {
		return nil, err
	}

	if v, ok := storage[StorageSecretDataStorageField]; ok {
		return v, nil
	}

	return nil, fmt.Errorf("%s key not found in secret %s", StorageSecretDataKey, StorageSecret)
}

func (api *API) getApplyConfig(roles *roles.JSON) (*clientv1.ConfigMapApplyConfiguration, error) {
	data, err := json.MarshalIndent(&roles, "", "  ")
	if err != nil {
		return nil, err
	}

	stdFormat := (`package karavi.common
default roles = {}
roles = ` + string(data))

	config := clientv1.ConfigMap(RolesConfigMap, api.Namespace)
	config.WithData(map[string]string{
		RolesConfigMapDataKey: stdFormat,
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