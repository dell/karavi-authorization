package role

import (
	"context"
	"encoding/json"
	"fmt"
	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/pb"
	"strings"

	"github.com/sirupsen/logrus"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/kubernetes"
)

type Option func(*Service)

func defaultOptions() []Option {
	return []Option{
		WithLogger(logrus.NewEntry(logrus.New())),
	}
}

// WithLogger provides a logger.
func WithLogger(log *logrus.Entry) func(*Service) {
	return func(t *Service) {
		t.log = log
	}
}

type Validator interface {
	Validate(ctx context.Context, role *roles.Instance) error
}

type Service struct {
	namespace string
	kube      kubernetes.Interface
	validator Validator
	log       *logrus.Entry
	pb.UnimplementedRoleServiceServer
}

func NewService(kube kubernetes.Interface, namespace string, opts ...Option) *Service {
	var s Service
	for _, opt := range defaultOptions() {
		opt(&s)
	}
	for _, opt := range opts {
		opt(&s)
	}

	return &Service{
		namespace: namespace,
	}
}

func (s *Service) Create(ctx context.Context, req *pb.RoleCreateRequest) (*pb.RoleCreateResponse, error) {
	rff, err := createNewRole(req)
	if err != nil {
		return nil, err
	}

	existingRoles, err := s.getExistingRoles(ctx)
	if err != nil {
		return nil, err
	}

	err = checkForDuplicates(ctx, existingRoles, rff)
	if err != nil {
		return nil, err
	}

	err = s.validateRole(ctx, existingRoles, rff)
	if err != nil {
		return nil, err
	}

	err = s.updateRoles(ctx, existingRoles)
	if err != nil {
		return nil, err
	}

	return &pb.RoleCreateResponse{}, nil
}

func createNewRole(req *pb.RoleCreateRequest) (*roles.JSON, error) {
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

	return &rff, nil
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

func checkForDuplicates(ctx context.Context, existingRoles *roles.JSON, rff *roles.JSON) error {
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
		return fmt.Errorf("duplicate roles: %v", dups)
	}

	return nil
}

func (s *Service) validateRole(ctx context.Context, existingRoles *roles.JSON, rff *roles.JSON) error {
	adding := rff.Instances()
	for _, role := range adding {
		err := s.validator.Validate(ctx, s.kube, role)
		if err != nil {
			err = fmt.Errorf("%s failed validation: %+v", role.Name, err)
			return err
		}

		err = existingRoles.Add(role)
		if err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) updateRoles(ctx context.Context, roles *roles.JSON) error {
	data, err := json.MarshalIndent(&roles, "", "  ")
	if err != nil {
		return err
	}

	stdFormat := (`package karavi.common
default roles = {}
roles = ` + string(data))

	cm := &v1.ConfigMapApplyConfiguration{
		Data: map[string]string{
			"common.rego": stdFormat,
		},
	}

	_, err = s.kube.CoreV1().ConfigMaps(s.namespace).Apply(ctx, cm, meta.ApplyOptions{})
	if err != nil {
		return err
	}
	return nil
}
