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
	ROLES_CONFIGMAP          = "common"
	ROLES_CONFIGMAP_DATA_KEY = "common.rego"

	STORAGE_SECRET                    = "karavi-storage-secret"
	STORAGE_SECRET_DATA_KEY           = "storage-systems.yaml"
	STORAGE_SECRET_DATA_STORAGE_FIELD = "storage"
)

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
		"ConfigMap":        ROLES_CONFIGMAP,
		"ConfigMapDataKey": ROLES_CONFIGMAP_DATA_KEY,
	}).Debug("Getting configMap containing configured roles")

	common, err := api.Client.CoreV1().ConfigMaps(api.Namespace).Get(ctx, ROLES_CONFIGMAP, meta.GetOptions{})
	if err != nil {
		return nil, err
	}

	rolesRego := common.Data[ROLES_CONFIGMAP_DATA_KEY]
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

	var roleNamesBuilder strings.Builder
	for i, role := range roles.Instances() {
		if i == len(roles.Instances())-1 {
			roleNamesBuilder.Write([]byte(role.Name))
		} else {
			roleNamesBuilder.Write([]byte(fmt.Sprintf("%s,", role.Name)))
		}
	}

	api.Log.WithFields(logrus.Fields{
		"ConfigMap":        ROLES_CONFIGMAP,
		"ConfigMapDataKey": ROLES_CONFIGMAP_DATA_KEY,
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
		"Secret":        STORAGE_SECRET,
		"SecretDataKey": STORAGE_SECRET_DATA_KEY,
	}).Debug("Getting secret containing configured storage systems")

	storageSecret, err := api.Client.CoreV1().Secrets(api.Namespace).Get(ctx, STORAGE_SECRET, meta.GetOptions{})
	if err != nil {
		return nil, err
	}

	var data []byte
	if v, ok := storageSecret.Data[STORAGE_SECRET_DATA_KEY]; ok {
		data = v
	} else {
		return nil, fmt.Errorf("%s data key not found in secret %s", STORAGE_SECRET_DATA_KEY, STORAGE_SECRET)
	}

	var storage map[string]types.Storage
	if err := yaml.Unmarshal(data, &storage); err != nil {
		return nil, err
	}

	if v, ok := storage[STORAGE_SECRET_DATA_STORAGE_FIELD]; ok {
		return v, nil
	}

	return nil, fmt.Errorf("%s key not found in secret %s", STORAGE_SECRET_DATA_KEY, STORAGE_SECRET)
}

func (api *API) getApplyConfig(roles *roles.JSON) (*clientv1.ConfigMapApplyConfiguration, error) {
	data, err := json.MarshalIndent(&roles, "", "  ")
	if err != nil {
		return nil, err
	}

	stdFormat := (`package karavi.common
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
