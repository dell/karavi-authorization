// Copyright © 2022 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8s

import (
	"context"
	"encoding/json"
	"fmt"
	"karavi-authorization/cmd/karavictl/cmd"
	storage "karavi-authorization/cmd/karavictl/cmd"
	"karavi-authorization/internal/role-service/roles"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientv1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

// API holds data used to access the K8S API
type API struct {
	Client    kubernetes.Interface
	Lock      sync.Mutex
	Namespace string
	Log       *logrus.Entry
}

const (
	// RolesConfigMap is the configMap containing configured roles
	RolesConfigMap = "common"
	// RolesConfigMapDataKey is the key value for the roles in the configMap
	RolesConfigMapDataKey = "common.rego"

	// StorageSecret is the secret containing configured storage
	StorageSecret = "karavi-storage-secret"
	// StorageSecretDataKey is the key value for the storage in the secret
	StorageSecretDataKey = "storage-systems.yaml"
	// StorageSecretDataStorageField is the top level field in the storage data itself
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
func (api *API) GetConfiguredStorage(ctx context.Context) (storage.Storage, error) {
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

	var storage map[string]storage.Storage
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

// UpdateStorages updates the storage secret with supplied collection of storages
func (api *API) UpdateStorages(ctx context.Context, storages cmd.Storage) error {
	api.Lock.Lock()
	defer api.Lock.Unlock()
	if api.Client == nil {
		err := ConnectFn(api)
		if err != nil {
			return err
		}
	}

	secret, err := api.getStorageSecret(storages)
	if err != nil {
		return err
	}

	api.Log.WithFields(logrus.Fields{
		"Secret":        StorageSecret,
		"SecretDataKey": StorageSecretDataKey,
	}).Debug("Applying new storage to a secret containing configured storages")

	_, err = api.Client.CoreV1().Secrets(api.Namespace).Apply(ctx, secret, meta.ApplyOptions{FieldManager: "application/apply-patch", Force: true})
	if err != nil {
		return err
	}

	return nil
}

func (api *API) getStorageSecret(storages storage.Storage) (*clientv1.SecretApplyConfiguration, error) {

	var data map[string]storage.Storage = make(map[string]storage.Storage)

	data["storage"] = storages

	b, err := yaml.Marshal(&data)
	if err != nil {
		return nil, err
	}

	secret := clientv1.Secret(StorageSecret, api.Namespace)
	secret.WithData(map[string][]byte{
		StorageSecretDataKey: b,
	})

	return secret, nil
}
