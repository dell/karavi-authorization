package role

import (
	"context"
	"fmt"
	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/pb"

	"github.com/sirupsen/logrus"
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

// Validator validates a role instance
type Validator interface {
	Validate(ctx context.Context, role *roles.Instance) error
}

// Kube operatoes on roles in Kubernetes
type Kube interface {
	GetExistingRoles(ctx context.Context) (*roles.JSON, error)
	UpdateRoles(ctx context.Context, roles *roles.JSON) error
}

type Service struct {
	namespace string
	kube      Kube
	validator Validator
	log       *logrus.Entry
	pb.UnimplementedRoleServiceServer
}

func NewService(kube Kube, validator Validator, namespace string, opts ...Option) *Service {
	var s Service
	for _, opt := range defaultOptions() {
		opt(&s)
	}
	for _, opt := range opts {
		opt(&s)
	}

	return &Service{
		namespace: namespace,
		kube:      kube,
		validator: validator,
	}
}

func (s *Service) Create(ctx context.Context, req *pb.RoleCreateRequest) (*pb.RoleCreateResponse, error) {
	rff, err := createNewRole(req)
	if err != nil {
		return nil, err
	}

	existingRoles, err := s.kube.GetExistingRoles(ctx)
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

	err = s.kube.UpdateRoles(ctx, existingRoles)
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
		err := s.validator.Validate(ctx, role)
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
