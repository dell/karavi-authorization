package validate

import (
	"context"
	"fmt"
	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/internal/types"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/yaml"
)

const (
	STORAGE_SECRET          = "karavi-storage-secret"
	STORAGE_SECRET_DATA_KEY = "storage-systems.yaml"
)

type RoleValidator struct {
	kube      kubernetes.Interface
	namespace string
	log       *logrus.Entry
}

func NewRoleValidator(kube kubernetes.Interface, namespace string) *RoleValidator {
	return &RoleValidator{
		kube:      kube,
		namespace: namespace,
	}
}

func (v *RoleValidator) Validate(ctx context.Context, role *roles.Instance) error {
	v.log.Info("Validating role")

	if !validSystemType(role.SystemType) {
		return fmt.Errorf("system type %s is not supported", role.SystemType)
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
		return fmt.Errorf("system type %s is not supported", systemType)
	}

	return vFn(ctx, system, role.SystemID, role.Pool, int64(role.Quota))
}

func validSystemType(sysType string) bool {
	for k := range types.SupportedStorageTypes {
		if sysType == k {
			return true
		}
	}
	return false
}

func (v *RoleValidator) getStorageSystem(ctx context.Context, systemId string) (types.System, string, error) {
	authorizedSystems, err := v.getConfiguredStorage(ctx)
	if err != nil {
		return types.System{}, "", fmt.Errorf("failed to get configured storage systems: %+v", err)
	}

	for systemType, storageSystems := range authorizedSystems["storage"] {
		if _, ok := storageSystems[systemId]; ok {
			return storageSystems[systemId], systemType, nil
		}
	}
	return types.System{}, "", fmt.Errorf("unable to find storage system %s in secret %s", systemId, STORAGE_SECRET)
}

func (v *RoleValidator) getConfiguredStorage(ctx context.Context) (map[string]types.Storage, error) {
	storageSecret, err := v.kube.CoreV1().Secrets(v.namespace).Get(ctx, STORAGE_SECRET, v1.GetOptions{})
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

	return storage, nil
}
