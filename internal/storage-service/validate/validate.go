package validate

import (
	"context"
	"fmt"
	"karavi-authorization/internal/types"

	"github.com/sirupsen/logrus"
)

type Kube interface {
	GetConfiguredStorage(ctx context.Context) (types.Storage, error)
}

type StorageValidator struct {
	kube Kube
	log  *logrus.Entry
}

func NewStorageValidator(kube Kube, log *logrus.Entry) *StorageValidator {
	return &StorageValidator{
		kube: kube,
		//namespace: namespace,
		log: log,
	}
}

func (v *StorageValidator) Validate(ctx context.Context, systemID string, systemType string, system types.System) error {

	v.log.Info("Validating storage")
	if !validSystemType(systemType) {
		return fmt.Errorf("system type %s is not supported", systemType)
	}

	type validateFn func(ctx context.Context, system types.System, systemID string) error
	var vFn validateFn

	switch systemType {
	case "powerflex":
		vFn = ValidatePowerFlex
	case "powermax":
		vFn = ValidatePowerMax
	case "powerscale":
		vFn = ValidatePowerScale
	default:
		return fmt.Errorf("system type %s is not supported", systemType)
	}

	return vFn(ctx, system, systemID)
}

func validSystemType(sysType string) bool {
	for k := range types.SupportedStorageTypes {
		if sysType == k {
			return true
		}
	}
	return false
}
