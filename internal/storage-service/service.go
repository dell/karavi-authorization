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

package storage

import (
	"context"
	"encoding/json"
	"fmt"
	storage "karavi-authorization/cmd/karavictl/cmd"
	"karavi-authorization/pb"
	"net/url"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"

	"github.com/dell/goscaleio"
	"github.com/sirupsen/logrus"
)

const (
	// KbInGb is the number of kilobytes in a gigabyte. Used for converting Powerflex volume size in Kb to Gb
	KbInGb = 1048576
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

// Validator validates a storage instance
type Validator interface {
	Validate(ctx context.Context, systemID string, systemType string, system storage.System) error
}

// Kube operates on storages in Kubernetes
type Kube interface {
	GetConfiguredStorage(ctx context.Context) (storage.Storage, error)
	UpdateStorages(ctx context.Context, storages storage.Storage) error
}

// Service implements the StorageService protobuf definiton
type Service struct {
	kube                        Kube
	validator                   Validator
	log                         *logrus.Entry
	concurrentPowerFlexRequests int
	powerFlexConfigurationLock  sync.Mutex // lock for concurrent powerflex requests
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
	s.log.Debug("Getting configured storages")
	existingStorages, err := s.kube.GetConfiguredStorage(ctx)
	if err != nil {
		return nil, err
	}
	if existingStorages == nil {
		existingStorages = make(map[string]storage.SystemType)
	}

	newSystem := storage.System{
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
	s.log.Debug("Applying new storage in Kubernetes")
	systemType := existingStorages[req.StorageType]
	if systemType == nil {
		systemType = make(map[string]storage.System)
	}
	systemType[req.SystemId] = newSystem
	existingStorages[req.StorageType] = systemType
	err = s.kube.UpdateStorages(ctx, existingStorages)
	if err != nil {
		return nil, err
	}

	return &pb.StorageCreateResponse{}, nil
}

// List lists the configured roles
func (s *Service) List(ctx context.Context, _ *pb.StorageListRequest) (*pb.StorageListResponse, error) {
	s.log.Info("Serving list storage request")

	// Get the current list of registered storage systems
	s.log.Debug("Getting configured storage")
	existingStorages, err := s.kube.GetConfiguredStorage(ctx)
	if err != nil {
		s.log.WithError(err).Debug()
		return nil, err
	}

	s.log.Debug("JSON marshaling configured storage")
	b, err := json.Marshal(&existingStorages)
	if err != nil {
		s.log.WithError(err).Debug()
		return nil, err
	}

	return &pb.StorageListResponse{Storage: b}, nil
}

// Update updates the configured storage
func (s *Service) Update(ctx context.Context, req *pb.StorageUpdateRequest) (*pb.StorageUpdateResponse, error) {
	s.log.WithFields(logrus.Fields{
		"StorageType": req.StorageType,
		"Endpoint":    req.Endpoint,
		"SystemId":    req.SystemId,
		"Username":    req.UserName,
		"Password":    req.Password,
	}).Info("Serving update storage request")

	// Get the current list of registered storage systems
	s.log.Debug("Getting configured storage")
	cfgStorage, err := s.kube.GetConfiguredStorage(ctx)
	if err != nil {
		s.log.WithError(err).Debug()
		return nil, err
	}

	var didUpdate bool
	for k := range cfgStorage {
		if k != req.StorageType {
			continue
		}
		_, ok := cfgStorage[k][req.SystemId]
		if !ok {
			continue
		}

		cfgStorage[k][req.SystemId] = storage.System{
			User:     req.UserName,
			Password: req.Password,
			Endpoint: req.Endpoint,
			Insecure: req.Insecure,
		}
		didUpdate = true
		break
	}

	if !didUpdate {
		return nil, fmt.Errorf("no matching storage systems to update")
	}

	s.log.Debug("Applying updated storage in Kubernetes")
	err = s.kube.UpdateStorages(ctx, cfgStorage)
	if err != nil {
		s.log.WithError(err).Debug()
		return nil, err
	}

	return &pb.StorageUpdateResponse{}, nil
}

// Delete deletes a storage
func (s *Service) Delete(ctx context.Context, req *pb.StorageDeleteRequest) (*pb.StorageDeleteResponse, error) {
	s.log.WithFields(logrus.Fields{
		"StorageType": req.StorageType,
		"SystemId":    req.SystemId,
	}).Info("Serving delete storage request")

	// Get the current list of registered storage systems
	s.log.Debug("Getting configured storage")
	existingStorages, err := s.kube.GetConfiguredStorage(ctx)
	if err != nil {
		return nil, err
	}

	s.log.Debug("Getting system type")
	systemType, ok := existingStorages[req.StorageType]
	if !ok {
		return nil, fmt.Errorf("error: storage of type %s is missing", req.StorageType)
	}

	s.log.Debug("Check the requested system ID exists")
	if _, systemIDExists := systemType[req.SystemId]; !systemIDExists {
		return nil, fmt.Errorf("error: system with ID %s does not exist", req.SystemId)
	}

	s.log.Debug("Deleting the storage and updating the secret in Kubernetes")
	delete(systemType, req.SystemId)
	existingStorages[req.StorageType] = systemType
	err = s.kube.UpdateStorages(ctx, existingStorages)
	if err != nil {
		return nil, err
	}

	return &pb.StorageDeleteResponse{}, nil
}

// Get retrieves a storage info
func (s *Service) Get(ctx context.Context, req *pb.StorageGetRequest) (*pb.StorageGetResponse, error) {
	s.log.WithFields(logrus.Fields{
		"StorageType": req.StorageType,
		"SystemId":    req.SystemId,
	}).Info("Serving get storage request")

	// Get the current list of registered storage systems
	s.log.Debug("Getting configured storage")
	existingStorages, err := s.kube.GetConfiguredStorage(ctx)
	if err != nil {
		return nil, err
	}

	s.log.Debug("Getting system type")
	systemType, ok := existingStorages[req.StorageType]
	if !ok {
		return nil, fmt.Errorf("error: storage of type %s is missing", req.StorageType)
	}

	s.log.Debug("Check the requested system ID exists")
	if _, systemIDExists := systemType[req.SystemId]; !systemIDExists {
		return nil, fmt.Errorf("error: system with ID %s does not exist", req.SystemId)
	}

	s.log.Debug("JSON marshaling configured storage")
	system := systemType[req.SystemId]
	system.Password = "(omitted)"
	b, err := json.Marshal(system)
	if err != nil {
		return nil, err
	}

	return &pb.StorageGetResponse{Storage: b}, nil
}

// GetPowerflexVolumes gets volume information from a list of volume names
func (s *Service) GetPowerflexVolumes(ctx context.Context, req *pb.GetPowerflexVolumesRequest) (*pb.GetPowerflexVolumesResponse, error) {
	s.log.WithFields(logrus.Fields{
		"SystemId": req.SystemId,
		"Volumes":  req.VolumeName,
	}).Info("Serving get powerflex volumes request")

	// Get the current list of registered storage systems
	s.log.Debug("Getting configured storage")
	existingStorages, err := s.kube.GetConfiguredStorage(ctx)
	if err != nil {
		return nil, err
	}

	// Extract relevant storage system from requested systemId
	systemType, ok := existingStorages["powerflex"]
	if !ok {
		return nil, fmt.Errorf("error: no powerflex storage configured")
	}

	system, ok := systemType[req.SystemId]
	if !ok {
		return nil, fmt.Errorf("error: system with ID %s does not exist", req.SystemId)
	}

	// Establish connection to powerflex
	s.log.Debug("Connecting to Powerflex")
	endpoint := GetPowerFlexEndpoint(system)
	epURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("endpoint %s is invalid: %v", epURL, err)
	}

	epURL.Scheme = "https"
	client, err := goscaleio.NewClientWithArgs(epURL.String(), "", 0, system.Insecure, false)
	if err != nil {
		return nil, fmt.Errorf("creating powerflex client for %s: %w", req.SystemId, err)
	}

	_, err = client.Authenticate(&goscaleio.ConfigConnect{
		Username: system.User,
		Password: system.Password,
	})
	if err != nil {
		return nil, fmt.Errorf("powerflex authentication failed: %v", err)
	}

	// rate limit the client
	rateLimitedClient := newRateLimitedPowerFlexClient(client, semaphore.NewWeighted(int64(s.GetConcurrentPowerFlexRequests())))

	volumes := make([]*pb.Volume, len(req.VolumeName))
	var eg errgroup.Group

	// Get each volume from powerflex
	for i, volumeName := range req.VolumeName {
		i := i
		volumeName := volumeName
		eg.Go(func() error {
			vol, err := rateLimitedClient.GetVolume(ctx, "", "", "", volumeName, false)
			if err != nil {
				return fmt.Errorf("getting volume %s: %w", volumeName, err)
			}

			if len(vol) == 0 {
				return fmt.Errorf("couldn't find volumes for %s", volumeName)
			}

			storagePoolName, err := rateLimitedClient.FindStoragePool(ctx, vol[0].StoragePoolID, "", "", "")
			if err != nil {
				return fmt.Errorf("getting storage pool name for %s: %w", volumeName, err)
			}

			volumes[i] = &pb.Volume{
				Name:     volumeName,
				Size:     float32(vol[0].SizeInKb) / float32(KbInGb),
				SystemId: req.SystemId,
				Id:       vol[0].ID,
				Pool:     storagePoolName.Name,
			}
			return nil
		})
	}
	err = eg.Wait()
	if err != nil {
		return nil, err
	}
	return &pb.GetPowerflexVolumesResponse{Volume: volumes}, nil
}

// CheckForDuplicates checks if requested systemID already exists
func CheckForDuplicates(_ context.Context, existingStorages storage.Storage, systemID string, storageType string) error {
	// Check that we are not duplicating, no errors, etc.
	sysIDs := strings.Split(systemID, ",")
	isDuplicate := func() (string, bool) {
		storType, ok := existingStorages[storageType]
		if !ok {
			existingStorages[storageType] = make(map[string]storage.System)
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

// GetConcurrentPowerFlexRequests gets the configured number of concurrent PowerFlex requests for the storage service
func (s *Service) GetConcurrentPowerFlexRequests() int {
	s.powerFlexConfigurationLock.Lock()
	defer s.powerFlexConfigurationLock.Unlock()
	return s.concurrentPowerFlexRequests
}

// SetConcurrentPowerFlexRequests configures the number of concurrent PowerFlex requests for the storage service
func (s *Service) SetConcurrentPowerFlexRequests(n int) {
	s.powerFlexConfigurationLock.Lock()
	defer s.powerFlexConfigurationLock.Unlock()
	s.concurrentPowerFlexRequests = n
}
