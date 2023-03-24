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
	pb.UnimplementedTenantServiceServer
	next pb.TenantServiceServer
	log  *logrus.Entry
}

// TelemetryMW logs and traces the tenant service
func TelemetryMW(log *logrus.Entry, next pb.TenantServiceServer) *telmetryMW {
	return &telmetryMW{
		next: next,
		log:  log,
	}
}

// CreateTenant wraps CreateTenant
func (t *telmetryMW) CreateTenant(ctx context.Context, req *pb.CreateTenantRequest) (*pb.Tenant, error) {
	now := time.Now()
	defer t.timeSince(now, "CreateTenant")

	attrs := trace.WithAttributes(attribute.String("name", req.Tenant.Name), attribute.Bool("approveSdc", req.Tenant.Approvesdc))
	ctx, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-tenant-service").Start(ctx, "tenantCreate", attrs)
	defer span.End()

	t.log.WithFields(logrus.Fields{
		"name":       req.Tenant.Name,
		"approveSdc": req.Tenant.Approvesdc,
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
func (t *telmetryMW) UpdateTenant(ctx context.Context, req *pb.UpdateTenantRequest) (*pb.Tenant, error) {
	now := time.Now()
	defer t.timeSince(now, "UpdateTenant")

	attrs := trace.WithAttributes(attribute.String("tenant", req.TenantName), attribute.Bool("approveSdc", req.Approvesdc))
	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-tenant-service").Start(ctx, "tenantUpdate", attrs)
	defer span.End()

	t.log.WithFields(logrus.Fields{
		"name":       req.TenantName,
		"approveSdc": req.Approvesdc,
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
func (t *telmetryMW) GetTenant(ctx context.Context, req *pb.GetTenantRequest) (*pb.Tenant, error) {
	now := time.Now()
	defer t.timeSince(now, "GetTenant")

	attrs := trace.WithAttributes(attribute.String("tenant", req.Name))
	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-tenant-service").Start(ctx, "tenantGet", attrs)
	defer span.End()

	t.log.WithFields(logrus.Fields{
		"name": req.Name,
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
func (t *telmetryMW) DeleteTenant(ctx context.Context, req *pb.DeleteTenantRequest) (*pb.DeleteTenantResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "DeleteTenant")

	attrs := trace.WithAttributes(attribute.String("tenant", req.Name))
	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-tenant-service").Start(ctx, "tenantDelete", attrs)
	defer span.End()

	t.log.WithFields(logrus.Fields{
		"name": req.Name,
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
func (t *telmetryMW) ListTenant(ctx context.Context, req *pb.ListTenantRequest) (*pb.ListTenantResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "ListTenant")

	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-tenant-service").Start(ctx, "tenantList")
	defer span.End()

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
func (t *telmetryMW) BindRole(ctx context.Context, req *pb.BindRoleRequest) (*pb.BindRoleResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "BindRole")

	attrs := trace.WithAttributes(attribute.String("tenant", req.TenantName), attribute.String("role", req.RoleName))
	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-tenant-service").Start(ctx, "tenantBindRole", attrs)
	defer span.End()

	t.log.WithFields(logrus.Fields{
		"name": req.TenantName,
		"role": req.RoleName,
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
func (t *telmetryMW) UnbindRole(ctx context.Context, req *pb.UnbindRoleRequest) (*pb.UnbindRoleResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "UnbindRole")

	attrs := trace.WithAttributes(attribute.String("tenant", req.TenantName), attribute.String("role", req.RoleName))
	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-tenant-service").Start(ctx, "tenantUnbindRole", attrs)
	defer span.End()

	t.log.WithFields(logrus.Fields{
		"name": req.TenantName,
		"role": req.RoleName,
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
func (t *telmetryMW) GenerateToken(ctx context.Context, req *pb.GenerateTokenRequest) (*pb.GenerateTokenResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "GenerateToken")

	attrs := trace.WithAttributes(attribute.String("tenant", req.TenantName))
	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-tenant-service").Start(ctx, "tenantGenerateToken", attrs)
	defer span.End()

	t.log.WithFields(logrus.Fields{
		"name":            req.TenantName,
		"AccessTokenTTL":  req.AccessTokenTTL,
		"RefreshTokenTTL": req.RefreshTokenTTL,
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
func (t *telmetryMW) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "RefreshToken")

	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-tenant-service").Start(ctx, "tenantRefreshToken")
	defer span.End()

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
func (t *telmetryMW) RevokeTenant(ctx context.Context, req *pb.RevokeTenantRequest) (*pb.RevokeTenantResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "RevokeTenant")

	attrs := trace.WithAttributes(attribute.String("tenant", req.TenantName))
	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-tenant-service").Start(ctx, "tenantRevoke", attrs)
	defer span.End()

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
func (t *telmetryMW) CancelRevokeTenant(ctx context.Context, req *pb.CancelRevokeTenantRequest) (*pb.CancelRevokeTenantResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "CancelRevokeTenant")

	attrs := trace.WithAttributes(attribute.String("tenant", req.TenantName))
	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-tenant-service").Start(ctx, "tenantCancelRevoke", attrs)
	defer span.End()

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

func (t *telmetryMW) timeSince(start time.Time, fName string) {
	t.log.WithFields(logrus.Fields{
		"duration": fmt.Sprintf("%v", time.Since(start)),
		"function": fName,
	}).Debug("Duration")
}
