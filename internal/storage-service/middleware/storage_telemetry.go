// Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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

type telemetryMW struct {
	pb.UnimplementedStorageServiceServer
	next pb.StorageServiceServer
	log  *logrus.Entry
}

// TelemetryMW logs and traces the storage service
func NewStorageTelemetryMW(log *logrus.Entry, next pb.StorageServiceServer) *telemetryMW {
	return &telemetryMW{
		next: next,
		log:  log,
	}
}

// CreateStorage wraps Create
func (t *telemetryMW) CreateStorage(ctx context.Context, req *pb.StorageCreateRequest) (*pb.StorageCreateResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "Create")

	span := trace.SpanFromContext(ctx)
	setAttributes(span, map[string]interface{}{
		"StorageType": req.StorageType,
		"Endpoint":    req.Endpoint,
		"SystemId":    req.SystemId,
		"UserName":    req.UserName,
		"Password":    req.Password,
		"Insecure":    req.Insecure,
	})

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

// Update wraps Update
func (t *telemetryMW) Update(ctx context.Context, req *pb.StorageUpdateRequest) (*pb.StorageUpdateResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "Update")

	span := trace.SpanFromContext(ctx)
	setAttributes(span, map[string]interface{}{
		"StorageType": req.StorageType,
		"Endpoint":    req.Endpoint,
		"SystemId":    req.SystemId,
		"UserName":    req.UserName,
		"Password":    req.Password,
		"Insecure":    req.Insecure,
	})

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

// Get wraps Get
func (t *telemetryMW) Get(ctx context.Context, req *pb.StorageGetRequest) (*pb.StorageGetResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "Get")

	span := trace.SpanFromContext(ctx)
	setAttributes(span, map[string]interface{}{
		"StorageType": req.StorageType,
		"SystemId":    req.SystemId,
	})

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

// Delete wraps Delete
func (t *telemetryMW) Delete(ctx context.Context, req *pb.StorageDeleteRequest) (*pb.StorageDeleteResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "DeleteStorage")

	span := trace.SpanFromContext(ctx)
	setAttributes(span, map[string]interface{}{
		"StorageType": req.StorageType,
		"SystemId":    req.SystemId,
	})

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

// List wraps List
func (t *telemetryMW) List(ctx context.Context, req *pb.StorageListRequest) (*pb.StorageListResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "ListStorage")

	span := trace.SpanFromContext(ctx)

	t.log.Info("Listing storage")

	storages, err := t.next.List(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return storages, nil
}

func (t *telemetryMW) timeSince(start time.Time, fName string) {
	t.log.WithFields(logrus.Fields{
		"duration": fmt.Sprintf("%v", time.Since(start)),
		"function": fName,
	}).Debug()
}

func setAttributes(span trace.Span, data map[string]interface{}) {
	var attr []attribute.KeyValue
	for k, v := range data {
		switch d := v.(type) {
		case string:
			attr = append(attr, attribute.KeyValue{Key: attribute.Key(k), Value: attribute.StringValue(d)})
		case bool:
			attr = append(attr, attribute.KeyValue{Key: attribute.Key(k), Value: attribute.BoolValue(d)})
		}
	}
	span.SetAttributes(attr...)
}
