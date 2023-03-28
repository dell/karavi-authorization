// Copyright © 2021 - 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package middleware

import (
	"context"
	"fmt"
	"karavi-authorization/pb"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type telmetryMW struct {
	pb.UnimplementedStorageServiceServer
	next pb.StorageServiceServer
	log  *logrus.Entry
}

// TelemetryMW logs and traces the storage service
func TelemetryMW(log *logrus.Entry, next pb.StorageServiceServer) *telmetryMW {
	return &telmetryMW{
		next: next,
		log:  log,
	}
}

// CreateStorage wraps Create
func (t *telmetryMW) CreateStorage(ctx context.Context, req *pb.StorageCreateRequest) (*pb.StorageCreateResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "Create")

	attrs := trace.WithAttributes(attribute.String("StorageType", req.StorageType), attribute.String("StorageType", req.StorageType), attribute.String("Endpoint", req.Endpoint),
		attribute.String("SystemId", req.SystemId), attribute.String("UserName", req.UserName), attribute.String("Password", req.Password), attribute.Bool("Insecure", req.Insecure))
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-storage-service").Start(ctx, "storageCreate", attrs)
	defer span.End()

	t.log.WithFields(logrus.Fields{
		"StorageType": req.StorageType,
		"Endpoint":    req.Endpoint,
		"SystemId":    req.SystemId,
		"UserName":    req.UserName,
		"Password":    req.Password,
		"Insecure":    req.Insecure,
	}).Info("Creating storage")

	storage, err := t.next.Create(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return storage, nil
}

// UpdateStorage wraps Update
func (t *telmetryMW) UpdateStorage(ctx context.Context, req *pb.StorageUpdateRequest) (*pb.StorageUpdateResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "Update")

	attrs := trace.WithAttributes(attribute.String("StorageType", req.StorageType), attribute.String("StorageType", req.StorageType), attribute.String("Endpoint", req.Endpoint),
		attribute.String("SystemId", req.SystemId), attribute.String("UserName", req.UserName), attribute.String("Password", req.Password), attribute.Bool("Insecure", req.Insecure))
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-storage-service").Start(ctx, "storageUpdate", attrs)
	defer span.End()

	t.log.WithFields(logrus.Fields{
		"StorageType": req.StorageType,
		"Endpoint":    req.Endpoint,
		"SystemId":    req.SystemId,
		"UserName":    req.UserName,
		"Password":    req.Password,
		"Insecure":    req.Insecure,
	}).Info("Updating storage")

	storage, err := t.next.Update(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return storage, nil
}

// GetStorage wraps Get
func (t *telmetryMW) GetStorage(ctx context.Context, req *pb.StorageGetRequest) (*pb.StorageGetResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "Get")

	attrs := trace.WithAttributes(attribute.String("StorageType", req.StorageType), attribute.String("SystemId", req.SystemId))
	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-storage-service").Start(ctx, "storageGet", attrs)
	defer span.End()

	t.log.WithFields(logrus.Fields{
		"StorageType": req.StorageType,
		"SystemId":    req.SystemId,
	}).Info("Getting storage")

	storage, err := t.next.Get(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return storage, nil
}

// DeleteStorage wraps Delete
func (t *telmetryMW) DeleteStorage(ctx context.Context, req *pb.StorageDeleteRequest) (*pb.StorageDeleteResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "DeleteStorage")

	attrs := trace.WithAttributes(attribute.String("StorageType", req.StorageType), attribute.String("SystemId", req.SystemId))
	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-storage-service").Start(ctx, "storageGet", attrs)
	defer span.End()

	t.log.WithFields(logrus.Fields{
		"StorageType": req.StorageType,
		"SystemId":    req.SystemId,
	}).Info("Deleting tenant")

	_, err := t.next.Delete(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return &pb.StorageDeleteResponse{}, nil

}

// ListStorage wraps List
func (t *telmetryMW) ListStorage(ctx context.Context, req *pb.StorageListRequest) (*pb.StorageListResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "ListStorage")

	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-storage-service").Start(ctx, "storageList")
	defer span.End()

	t.log.Info("Listing tenants")

	storages, err := t.next.List(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return storages, nil
}

// GetPowerFlexVolumes wraps GetPowerFlexVolumes
func (t *telmetryMW) GetPowerflexVolumes(ctx context.Context, req *pb.GetPowerflexVolumesRequest) (*pb.GetPowerflexVolumesResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "GetPowerFlexVolumes")

	attrs := trace.WithAttributes(attribute.StringSlice("VolumeName", req.VolumeName), attribute.String("SystemId", req.SystemId))
	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-storage-service").Start(ctx, "storageGet", attrs)
	defer span.End()

	t.log.WithFields(logrus.Fields{
		"VolumeName": req.VolumeName,
		"SystemId":   req.SystemId,
	}).Info("Getting storage")

	storage, err := t.next.GetPowerflexVolumes(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return storage, nil
}

func (t *telmetryMW) timeSince(start time.Time, fName string) {
	t.log.WithFields(logrus.Fields{
		"duration": fmt.Sprintf("%v", time.Since(start)),
		"function": fName,
	}).Debug()
}
