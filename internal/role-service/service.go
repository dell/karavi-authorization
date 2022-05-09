package role

import (
	"context"
	"fmt"
	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/pb"
	"log"
	"strings"

	"github.com/sirupsen/logrus"
)

// Option allows for functional option arguments on the RoleService.
type Option func(*Service)

func defaultOptions() []Option {
	return []Option{
		WithLogger(logrus.NewEntry(logrus.New())),
	}
}

// WithLogger provides a logger.
func WithLogger(log *logrus.Entry) func(*Service) {
	return func(s *Service) {
		s.log = log
	}
}

// Validator validates a role instance
type Validator interface {
	Validate(ctx context.Context, role *roles.Instance) error
}

// Kube operates on roles in Kubernetes
type Kube interface {
	GetConfiguredRoles(ctx context.Context) (*roles.JSON, error)
	UpdateRoles(ctx context.Context, roles *roles.JSON) error
}

// Service implements the RoleService protobuf definiton
type Service struct {
	kube      Kube
	validator Validator
	log       *logrus.Entry
	pb.UnimplementedRoleServiceServer
}

// NewService returns a new RoleService
func NewService(kube Kube, validator Validator, opts ...Option) *Service {
	var s Service
	for _, opt := range defaultOptions() {
		opt(&s)
	}
	for _, opt := range opts {
		opt(&s)
	}

	s.kube = kube
	s.validator = validator
	return &s
}

// Create creates a role
func (s *Service) Create(ctx context.Context, req *pb.RoleCreateRequest) (*pb.RoleCreateResponse, error) {
	s.log.WithFields(logrus.Fields{
		"Name":        req.Name,
		"StorageType": req.StorageType,
		"SystemId":    req.SystemId,
		"Pool":        req.Pool,
		"Quota(kb)":   req.Quota,
	}).Info("Serving create role request")

	roleInstance, err := roles.NewInstance(req.Name, req.StorageType, req.SystemId, req.Pool, req.Quota)
	if err != nil {
		return nil, err
	}

	var rff roles.JSON
	err = rff.Add(roleInstance)
	if err != nil {
		return nil, err
	}

	s.log.Debug("Getting existing roles in Kubernetes")
	existingRoles, err := s.kube.GetConfiguredRoles(ctx)
	if err != nil {
		s.log.WithError(err).Debug()
		return nil, err
	}

	s.log.Debug("Validating roles")
	err = s.validateRoles(ctx, existingRoles, &rff)
	if err != nil {
		s.log.WithError(err).Debug()
		return nil, err
	}

	s.log.Debug("Updating roles in Kubernetes")
	err = s.kube.UpdateRoles(ctx, existingRoles)
	if err != nil {
		s.log.WithError(err).Debug()
		return nil, err
	}

	return &pb.RoleCreateResponse{}, nil
}

// Delete deletes a role
func (s *Service) Delete(ctx context.Context, req *pb.RoleDeleteRequest) (*pb.RoleDeleteResponse, error) {
	s.log.WithFields(logrus.Fields{
		"Name":        req.Name,
		"StorageType": req.StorageType,
		"SystemId":    req.SystemId,
		"Pool":        req.Pool,
		"Quota(kb)":   req.Quota,
	}).Info("Serving delete role request")

	roleInstance, err := roles.NewInstance(req.Name, req.StorageType, req.SystemId, req.Pool, req.Quota)
	if err != nil {
		return nil, err
	}

	s.log.Debug("Getting existing roles from Kubernetes")
	existingRoles, err := s.kube.GetConfiguredRoles(ctx)
	if err != nil {
		s.log.WithError(err).Debug()
		return nil, err
	}

	s.log.WithFields(logrus.Fields{
		"Role": roleInstance.RoleKey.String(),
	}).Debug("Deleting role")

	matched := make(map[roles.Instance]struct{})
	existingRoles.Select(func(e roles.Instance) {
		log.Println(e.RoleKey.String())
		if strings.Contains(e.RoleKey.String(), roleInstance.RoleKey.String()) {
			matched[e] = struct{}{}
		}
	})

	if len(matched) == 0 {
		return nil, fmt.Errorf("role not found")
	}

	for k := range matched {
		err = existingRoles.Remove(&k)
		if err != nil {
			return nil, err
		}
	}

	s.log.Debug("Updating roles in Kubernetes")
	err = s.kube.UpdateRoles(ctx, existingRoles)
	if err != nil {
		s.log.WithError(err).Debug()
		return nil, err
	}

	return &pb.RoleDeleteResponse{}, nil
}

func (s *Service) validateRoles(ctx context.Context, existingRoles *roles.JSON, rff *roles.JSON) error {
	adding := rff.Instances()
	for _, role := range adding {
		err := s.validator.Validate(ctx, role)
		if err != nil {
			err = fmt.Errorf("%s failed validation: %+v", role.Name, err)
			return err
		}

		s.log.WithField("role", role.Name).Debug("Checking if role is duplicated")
		err = existingRoles.Add(role)
		if err != nil {
			return err
		}
	}
	return nil
}
