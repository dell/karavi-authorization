// Copyright © 2021-2023 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	storage "karavi-authorization/cmd/karavictl/cmd"
	"karavi-authorization/internal/k8s"
	"karavi-authorization/internal/role-service/roles"

	"github.com/sirupsen/logrus"
)

// Kube defines the interface for getting storage and/or role data
type Kube interface {
	GetConfiguredStorage(ctx context.Context) (storage.Storage, error)
}

// RoleValidator validates a role instance
type RoleValidator struct {
	kube Kube
	log  *logrus.Entry
}

// NewRoleValidator returns a RoleValidator
func NewRoleValidator(kube Kube, log *logrus.Entry) *RoleValidator {
	return &RoleValidator{
		kube: kube,
		log:  log,
	}
}

// Validate validates a role instance
func (v *RoleValidator) Validate(ctx context.Context, role *roles.Instance) error {
	if !validSystemType(role.SystemType) {
		return fmt.Errorf("system type %s is not supported", role.SystemType)
	}

	system, systemType, err := v.getStorageSystem(ctx, role.SystemID)
	if err != nil {
		return err
	}

	// quota is in kilobytes (kb)
	type validateFn func(ctx context.Context, log *logrus.Entry, system storage.System, systemID string, pool string, quota int64) error
	var vFn validateFn

	switch role.SystemType {
	case "powerflex":
		vFn = PowerFlex
	case "powermax":
		vFn = PowerMax
	case "powerscale":
		vFn = PowerScale
	default:
		return fmt.Errorf("system type %s is not supported", systemType)
	}

	return vFn(ctx, v.log, system, role.SystemID, role.Pool, int64(role.Quota))
}

func validSystemType(sysType string) bool {
	for k := range storage.SupportedStorageTypes {
		if sysType == k {
			return true
		}
	}
	return false
}

func (v *RoleValidator) getStorageSystem(ctx context.Context, systemID string) (storage.System, string, error) {
	cfgStorage, err := v.kube.GetConfiguredStorage(ctx)
	if err != nil {
		return storage.System{}, "", fmt.Errorf("failed to get configured storage systems: %+v", err)
	}

	for systemType, storageSystems := range cfgStorage {
		if _, ok := storageSystems[systemID]; ok {
			return storageSystems[systemID], systemType, nil
		}
	}

	return storage.System{}, "", fmt.Errorf("unable to find storage system %s in secret %s", systemID, k8s.StorageSecret)
}
