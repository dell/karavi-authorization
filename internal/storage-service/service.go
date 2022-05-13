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

package storage

import (
	"context"
	"fmt"
	"karavi-authorization/internal/types"
	"karavi-authorization/pb"
	"strings"

	"github.com/sirupsen/logrus"
)

// Option allows for functional option arguments on the StorageService.
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
	Validate(ctx context.Context, systemID string, systemType string, system types.System) error
}

// Kube operators on storages in Kubernetes
type Kube interface {
	GetConfiguredStorage(ctx context.Context) (types.Storage, error)
	UpdateStorages(ctx context.Context, storages types.Storage) error
}

// Service implements the StorageService protobuf definiton
type Service struct {
	kube      Kube
	validator Validator
	log       *logrus.Entry
	pb.UnimplementedStorageServiceServer
}

// NewService returns a new StorageService
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

// Create creates a storage
func (s *Service) Create(ctx context.Context, req *pb.StorageCreateRequest) (*pb.StorageCreateResponse, error) {
	s.log.WithFields(logrus.Fields{
		"StorageType": req.StorageType,
		"Endpoint":    req.Endpoint,
		"SystemId":    req.SystemId,
		"Username":    req.UserName,
		"Password":    req.Password,
	}).Info("Create storage request")

	// Get the current list of registered storage systems
	s.log.Debug("Getting existing storages")
	existingStorages, err := s.kube.GetConfiguredStorage(ctx)
	if err != nil {
		return nil, err
	}
	if existingStorages == nil {
		existingStorages = make(map[string]types.SystemType)
	}

	newSystem := types.System{
		User:     req.UserName,
		Password: req.Password,
		Endpoint: req.Endpoint,
		Insecure: req.Insecure,
	}

	// Check that we are not duplicating
	s.log.Debug("Checking for duplicates")
	err = CheckForDuplicates(ctx, existingStorages, req.SystemId, req.StorageType)
	if err != nil {
		return nil, err
	}

	// Validating storage
	s.log.Debug("Validating storage")
	err = s.validator.Validate(ctx, req.SystemId, req.StorageType, newSystem)
	if err != nil {
		return nil, err
	}

	// Creating new storage and adding it to the list of existing storages
	s.log.Debug("Creating new storage")
	systemType := existingStorages[req.StorageType]
	if systemType == nil {
		systemType = make(map[string]types.System)
	}
	systemType[req.SystemId] = newSystem
	existingStorages[req.StorageType] = systemType
	err = s.kube.UpdateStorages(ctx, existingStorages)
	if err != nil {
		return nil, err
	}

	return &pb.StorageCreateResponse{}, nil
}

// CheckForDuplicates checks if requested systemID already exists
func CheckForDuplicates(ctx context.Context, existingStorages types.Storage, systemID string, storageType string) error {

	// Check that we are not duplicating, no errors, etc.
	sysIDs := strings.Split(systemID, ",")
	isDuplicate := func() (string, bool) {
		storType, ok := existingStorages[storageType]
		if !ok {
			existingStorages[storageType] = make(map[string]types.System)
			return "", false
		}
		for _, id := range sysIDs {
			if _, ok = storType[fmt.Sprint(id)]; ok {
				return id, true
			}
		}
		return "", false
	}

	if id, result := isDuplicate(); result {
		err := fmt.Errorf("error: %s system with ID %s is already registered", storageType, id)
		return err
	}

	return nil
}
