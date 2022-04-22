package role

import (
	"context"
	"encoding/json"
	"fmt"
	"karavi-authorization/internal/role-service/validate"
	"karavi-authorization/internal/roles"
	"karavi-authorization/pb"
	"strings"

	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Service struct {
	kube      kubernetes.Interface
	namespace string
}

// Validate validates the role
var Validate = func(ctx context.Context, role *roles.Instance) error {
	return validate.Validate(ctx, role)
}

func (s *Service) Create(ctx context.Context, req *pb.RoleCreateRequest) (*pb.RoleCreateResponse, error) {
	// create a new role
	parts := []string{
		req.StorageType,
		req.SystemId,
		req.Pool,
		req.Quota,
	}

	newRole, err := roles.NewInstance(req.Name, parts...)
	if err != nil {
		return nil, err
	}

	var rff roles.JSON
	err = rff.Add(newRole)
	if err != nil {
		return nil, err
	}

	// get existing roles
	existingRoles, err := s.getExistingRoles(ctx)
	if err != nil {
		return nil, err
	}

	// check for duplicates
	adding := rff.Instances()
	var dups []string
	for _, role := range adding {
		if existingRoles.Get(role.RoleKey) != nil {
			var dup bool
			if dup {
				dups = append(dups, role.Name)
			}
		}
	}
	if len(dups) > 0 {
		return nil, fmt.Errorf("duplicates %+v", dups)
	}

	// validate each role with backend storage
	for _, role := range adding {
		err := Validate(ctx, role)
		if err != nil {
			err = fmt.Errorf("%s failed validation: %+v", role.Name, err)
			return nil, err
		}

		err = existingRoles.Add(role)
		if err != nil {
			return nil, err
		}
	}

	// update roles
	if err = modifyCommonConfigMap(existingRoles); err != nil {
		return nil, err
	}

	return &pb.RoleCreateResponse{}, nil
}

func (s *Service) getExistingRoles(ctx context.Context) (*roles.JSON, error) {
	ccm, err := s.kube.CoreV1().ConfigMaps(s.namespace).Get(ctx, "common", meta.GetOptions{})
	if err != nil {
		return nil, err
	}

	rolesRego := ccm.Data["common.rego"]
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
func modifyCommonConfigMap(roles *roles.JSON) error {
	/*var err error

		data, err := json.MarshalIndent(&roles, "", "  ")
		if err != nil {
			return err
		}

		stdFormat := (`package karavi.common
	default roles = {}
	roles = ` + string(data))

		createCmd := execCommandContext(context.Background(), K3sPath,
			"kubectl",
			"create",
			"configmap",
			"common",
			"--from-literal=common.rego="+stdFormat,
			"-n", "karavi",
			"--dry-run=client",
			"-o", "yaml")
		applyCmd := execCommandContext(context.Background(), K3sPath, "kubectl", "apply", "-f", "-")

		pr, pw := io.Pipe()
		createCmd.Stdout = pw
		applyCmd.Stdin = pr
		applyCmd.Stdout = io.Discard

		if err := createCmd.Start(); err != nil {
			return fmt.Errorf("create: %w", err)
		}
		if err := applyCmd.Start(); err != nil {
			return fmt.Errorf("apply: %w", err)
		}

		eg := errgroup.Group{}
		eg.Go(func() error {
			defer pw.Close()
			if err := createCmd.Wait(); err != nil {
				return fmt.Errorf("create wait: %w", err)
			}
			return nil
		})
		if err := applyCmd.Wait(); err != nil {
			return fmt.Errorf("apply wait: %w", err)
		}
		if err := eg.Wait(); err != nil {
			return err
		}
		return nil*/
	return nil
}
