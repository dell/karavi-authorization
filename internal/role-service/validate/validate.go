package validate

import (
	"context"
	"encoding/base64"
	"fmt"
	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/internal/types"

	"gopkg.in/yaml.v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	STORAGE_SECRET          = "karavi-storage-secret"
	STORAGE_SECRET_DATA_KEY = "storage-systems.yaml"
)

type RoleValidator struct {
	kube      kubernetes.Interface
	namespace string
}

func (v *RoleValidator) Validate(ctx context.Context, role *roles.Instance) error {
	if !validSystemType(role.SystemType) {
		return fmt.Errorf("%s is not supported", role.SystemType)
	}

	system, systemType, err := v.getStorageSystem(ctx, role.SystemID)
	if err != nil {
		return err
	}

	// quota is in kilobytes (kb)
	type validateFn func(ctx context.Context, system types.System, systemId string, pool string, quota int64) error
	var vFn validateFn

	switch role.SystemType {
	case "powerflex":
		vFn = ValidatePowerFlex
	case "powermax":
		vFn = ValidatePowerMax
	case "powerscale":
		vFn = ValidatePowerScale
	default:
		return fmt.Errorf("%s is not supported", systemType)
	}

	return vFn(ctx, system, role.SystemID, role.Pool, int64(role.Quota))
}

func validSystemType(sysType string) bool {
	for k, _ := range types.SupportedStorageTypes {
		if sysType == k {
			return true
		}
	}
	return false
}

func (v *RoleValidator) getStorageSystem(ctx context.Context, systemId string) (types.System, string, error) {
	authorizedSystems, err := v.getConfiguredStorage(ctx)
	if err != nil {
		return types.System{}, "", fmt.Errorf("failed to get authorized storage systems: %+v", err)
	}

	for systemType, storageSystems := range authorizedSystems["storage"] {
		if _, ok := storageSystems[systemId]; ok {
			return storageSystems[systemId], systemType, nil
		}
	}
	return types.System{}, "", fmt.Errorf("unable to find authorized storage system with ID: %s", systemId)
}

func (v *RoleValidator) getConfiguredStorage(ctx context.Context) (map[string]types.Storage, error) {
	storageSecret, err := v.kube.CoreV1().Secrets(v.namespace).Get(ctx, STORAGE_SECRET, v1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var data []byte
	if v, ok := storageSecret.Data["storage-systems.yaml"]; ok {
		data = v
	} else {
		return nil, fmt.Errorf("%s data key not found in %s", STORAGE_SECRET_DATA_KEY, STORAGE_SECRET)
	}

	decodedSystems, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return nil, err
	}

	var storage map[string]types.Storage
	if err := yaml.Unmarshal(decodedSystems, &storage); err != nil {
		return nil, err
	}

	return storage, nil
}
