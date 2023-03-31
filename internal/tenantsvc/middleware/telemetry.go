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

// TelemetryMW logs the time taken for the request and sets span attributes
type TelemetryMW struct {
	pb.UnimplementedTenantServiceServer
	next pb.TenantServiceServer
	log  *logrus.Entry
}

// NewTelemetryMW returns new TelemetryMW
func NewTelemetryMW(log *logrus.Entry, next pb.TenantServiceServer) *TelemetryMW {
	return &TelemetryMW{
		next: next,
		log:  log,
	}
}

// CreateTenant wraps CreateTenant
func (t *TelemetryMW) CreateTenant(ctx context.Context, req *pb.CreateTenantRequest) (*pb.Tenant, error) {
	now := time.Now()
	defer t.timeSince(now, "CreateTenant")

	span := trace.SpanFromContext(ctx)
	setAttributes(span, map[string]interface{}{
		"tenant":      req.Tenant.Name,
		"approve_sdc": req.Tenant.Approvesdc,
	})

	t.log.WithFields(logrus.Fields{
		"tenant":      req.Tenant.Name,
		"approve_sdc": req.Tenant.Approvesdc,
	}).Info("Creating tenant")

	tenant, err := t.next.CreateTenant(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return tenant, nil
}

// UpdateTenant wraps UpdateTenant
func (t *TelemetryMW) UpdateTenant(ctx context.Context, req *pb.UpdateTenantRequest) (*pb.Tenant, error) {
	now := time.Now()
	defer t.timeSince(now, "UpdateTenant")

	span := trace.SpanFromContext(ctx)
	setAttributes(span, map[string]interface{}{
		"tenant":      req.TenantName,
		"approve_sdc": req.Approvesdc,
	})

	t.log.WithFields(logrus.Fields{
		"tenant":      req.TenantName,
		"approve_sdc": req.Approvesdc,
	}).Info("Updating tenant")

	tenant, err := t.next.UpdateTenant(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return tenant, nil
}

// GetTenant wraps GetTenant
func (t *TelemetryMW) GetTenant(ctx context.Context, req *pb.GetTenantRequest) (*pb.Tenant, error) {
	now := time.Now()
	defer t.timeSince(now, "GetTenant")

	span := trace.SpanFromContext(ctx)
	setAttributes(span, map[string]interface{}{
		"tenant": req.Name,
	})

	t.log.WithFields(logrus.Fields{
		"tenant": req.Name,
	}).Info("Getting tenant")

	tenant, err := t.next.GetTenant(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return tenant, nil
}

// DeleteTenant wraps DeleteTenant
func (t *TelemetryMW) DeleteTenant(ctx context.Context, req *pb.DeleteTenantRequest) (*pb.DeleteTenantResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "DeleteTenant")

	span := trace.SpanFromContext(ctx)
	setAttributes(span, map[string]interface{}{
		"tenant": req.Name,
	})

	t.log.WithFields(logrus.Fields{
		"tenant": req.Name,
	}).Info("Deleting tenant")

	_, err := t.next.DeleteTenant(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return &pb.DeleteTenantResponse{}, nil

}

// ListTenant wraps ListTenant
func (t *TelemetryMW) ListTenant(ctx context.Context, req *pb.ListTenantRequest) (*pb.ListTenantResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "ListTenant")

	span := trace.SpanFromContext(ctx)

	t.log.Info("Listing tenants")

	tenants, err := t.next.ListTenant(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return tenants, nil
}

// BindRole wraps BindRole
func (t *TelemetryMW) BindRole(ctx context.Context, req *pb.BindRoleRequest) (*pb.BindRoleResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "BindRole")

	span := trace.SpanFromContext(ctx)
	setAttributes(span, map[string]interface{}{
		"tenant": req.TenantName,
		"role":   req.RoleName,
	})

	t.log.WithFields(logrus.Fields{
		"tenant": req.TenantName,
		"role":   req.RoleName,
	}).Info("Binding tenant")

	_, err := t.next.BindRole(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return &pb.BindRoleResponse{}, nil
}

// UnbindRole wraps UnbindRole
func (t *TelemetryMW) UnbindRole(ctx context.Context, req *pb.UnbindRoleRequest) (*pb.UnbindRoleResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "UnbindRole")

	span := trace.SpanFromContext(ctx)
	setAttributes(span, map[string]interface{}{
		"tenant": req.TenantName,
		"role":   req.RoleName,
	})

	t.log.WithFields(logrus.Fields{
		"tenant": req.TenantName,
		"role":   req.RoleName,
	}).Info("Unbinding tenant")

	_, err := t.next.UnbindRole(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return &pb.UnbindRoleResponse{}, nil
}

// GenerateToken wraps GenerateToken
func (t *TelemetryMW) GenerateToken(ctx context.Context, req *pb.GenerateTokenRequest) (*pb.GenerateTokenResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "GenerateToken")

	span := trace.SpanFromContext(ctx)
	setAttributes(span, map[string]interface{}{
		"tenant":            req.TenantName,
		"access_token_TTL":  time.Duration(req.AccessTokenTTL).String(),
		"refresh_token_TTL": time.Duration(req.RefreshTokenTTL).String(),
	})

	t.log.WithFields(logrus.Fields{
		"tenant":          req.TenantName,
		"AccessTokenTTL":  time.Duration(req.AccessTokenTTL).String(),
		"RefreshTokenTTL": time.Duration(req.RefreshTokenTTL).String(),
	}).Info("Generating token")

	resp, err := t.next.GenerateToken(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return resp, nil
}

// RefreshToken wraps RefreshToken
func (t *TelemetryMW) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "RefreshToken")

	span := trace.SpanFromContext(ctx)

	t.log.Info("Refreshing token")

	resp, err := t.next.RefreshToken(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return resp, nil
}

// RevokeTenant wraps RevokeTenant
func (t *TelemetryMW) RevokeTenant(ctx context.Context, req *pb.RevokeTenantRequest) (*pb.RevokeTenantResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "RevokeTenant")

	span := trace.SpanFromContext(ctx)
	setAttributes(span, map[string]interface{}{
		"tenant": req.TenantName,
	})

	t.log.WithFields(logrus.Fields{
		"tenant": req.TenantName,
	}).Info("Revoking tenant")

	resp, err := t.next.RevokeTenant(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return resp, nil
}

// CancelRevokeTenant wraps CancelRevokeTenant
func (t *TelemetryMW) CancelRevokeTenant(ctx context.Context, req *pb.CancelRevokeTenantRequest) (*pb.CancelRevokeTenantResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "CancelRevokeTenant")

	span := trace.SpanFromContext(ctx)
	setAttributes(span, map[string]interface{}{
		"tenant": req.TenantName,
	})

	t.log.WithFields(logrus.Fields{
		"tenant": req.TenantName,
	}).Info("Cancelling tenant revocation")

	resp, err := t.next.CancelRevokeTenant(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		return nil, err
	}

	return resp, nil
}

func (t *TelemetryMW) timeSince(start time.Time, fName string) {
	t.log.WithFields(logrus.Fields{
		"function": fName,
		"duration": fmt.Sprintf("%v", time.Since(start)),
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
