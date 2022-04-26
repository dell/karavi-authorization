package validate

import (
	"context"
	"fmt"
	"karavi-authorization/internal/k8s"
	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/internal/types"

	"github.com/sirupsen/logrus"
)

type Kube interface {
	GetConfiguredStorage(ctx context.Context) (types.Storage, error)
}

type RoleValidator struct {
	kube      Kube
	namespace string
	log       *logrus.Entry
}

func NewRoleValidator(kube Kube, log *logrus.Entry, namespace string) *RoleValidator {
	return &RoleValidator{
		kube:      kube,
		namespace: namespace,
		log:       log,
	}
}

func (v *RoleValidator) Validate(ctx context.Context, role *roles.Instance) error {
	if !validSystemType(role.SystemType) {
		return fmt.Errorf("system type %s is not supported", role.SystemType)
	}

	v.log.WithFields(logrus.Fields{
		"SystemId": role.SystemID,
	}).Debug("Getting storage system from configured systems")

	system, systemType, err := v.getStorageSystem(ctx, role.SystemID)
	if err != nil {
		return err
	}

	// quota is in kilobytes (kb)
	type validateFn func(ctx context.Context, log *logrus.Entry, system types.System, systemId string, pool string, quota int64) error
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

	return vFn(ctx, v.log, system, role.SystemID, role.Pool, int64(role.Quota))
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
	storage, err := v.kube.GetConfiguredStorage(ctx)
	if err != nil {
		return types.System{}, "", fmt.Errorf("failed to get configured storage systems: %+v", err)
	}

	for systemType, storageSystems := range storage {
		if _, ok := storageSystems[systemId]; ok {
			return storageSystems[systemId], systemType, nil
		}
	}

	return types.System{}, "", fmt.Errorf("unable to find storage system %s in secret %s", systemId, k8s.STORAGE_SECRET)
}
