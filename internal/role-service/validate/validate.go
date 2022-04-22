package validate

import (
	"context"
	"fmt"
	"karavi-authorization/internal/roles"
	"karavi-authorization/internal/types"
)

func Validate(ctx context.Context, role *roles.Instance) error {
	if !validSystemType(role.SystemType) {
		return fmt.Errorf("%s is not supported", role.SystemType)
	}

	system, systemType, err := getStorageSystem(role.SystemID)
	if err != nil {
		return err
	}

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

func getStorageSystem(storageSystemID string) (types.System, string, error) {
	authorizedSystems, err := getConfiguredStorage()
	if err != nil {
		return types.System{}, "", fmt.Errorf("failed to get authorized storage systems: %+v", err)
	}

	for systemType, storageSystems := range authorizedSystems["storage"] {
		if _, ok := storageSystems[storageSystemID]; ok {
			return storageSystems[storageSystemID], systemType, nil
		}
	}
	return types.System{}, "", fmt.Errorf("unable to find authorized storage system with ID: %s", storageSystemID)
}

func getConfiguredStorage() (map[string]types.Storage, error) {
	/*k3sCmd := execCommandContext(context.Background(), K3sPath, "kubectl", "get",
		"--namespace=karavi",
		"--output=json",
		"secret/karavi-storage-secret")

	b, err := k3sCmd.Output()

	if err != nil {
		return nil, err
	}

	base64Systems := struct {
		Data map[string]string
	}{}

	if err := json.Unmarshal(b, &base64Systems); err != nil {
		return nil, err
	}

	decodedSystems, err := base64.StdEncoding.DecodeString(base64Systems.Data["storage-systems.yaml"])
	if err != nil {
		return nil, err
	}

	var listData map[string]types.Storage
	if err := yaml.Unmarshal(decodedSystems, &listData); err != nil {
		return nil, err
	}

	return listData, nil*/
	return nil, nil
}
