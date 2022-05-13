// Copyright © 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	type validateFn func(ctx context.Context, log *logrus.Entry, system types.System, systemID string) error
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

	return vFn(ctx, v.log, system, systemID)
}

func validSystemType(sysType string) bool {
	for k := range types.SupportedStorageTypes {
		if sysType == k {
			return true
		}
	}
	return false
}
