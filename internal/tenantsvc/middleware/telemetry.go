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

type TelmetryMW struct {
	pb.UnimplementedTenantServiceServer
	next pb.TenantServiceServer
	log  *logrus.Entry
}

func (t *TelmetryMW) CreateTenant(ctx context.Context, req *pb.CreateTenantRequest) (*pb.Tenant, error) {
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
func (t *TelmetryMW) UpdateTenant(ctx context.Context, req *pb.UpdateTenantRequest) (*pb.Tenant, error) {
	now := time.Now()
	defer t.timeSince(now, "UpdateTenant")

	attrs := trace.WithAttributes(attribute.String("name", req.TenantName), attribute.Bool("approveSdc", req.Approvesdc))
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
func (t *TelmetryMW) GetTenant(ctx context.Context, req *pb.GetTenantRequest) (*pb.Tenant, error) {
	now := time.Now()
	defer t.timeSince(now, "GetTenant")

	attrs := trace.WithAttributes(attribute.String("name", req.Name))
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
func (t *TelmetryMW) DeleteTenant(ctx context.Context, req *pb.DeleteTenantRequest) (*pb.DeleteTenantResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "DeleteTenant")

	attrs := trace.WithAttributes(attribute.String("name", req.Name))
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
func (t *TelmetryMW) ListTenant(ctx context.Context, req *pb.ListTenantRequest) (*pb.ListTenantResponse, error) {
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
func (t *TelmetryMW) BindRole(ctx context.Context, req *pb.BindRoleRequest) (*pb.BindRoleResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "BindRole")

	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-tenant-service").Start(ctx, "tenantBindRole")
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
func (t *TelmetryMW) UnbindRole(ctx context.Context, req *pb.UnbindRoleRequest) (*pb.UnbindRoleResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "UnbindRole")

	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-tenant-service").Start(ctx, "tenantUnbindRole")
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
func (t *TelmetryMW) GenerateToken(ctx context.Context, req *pb.GenerateTokenRequest) (*pb.GenerateTokenResponse, error) {
	now := time.Now()
	defer t.timeSince(now, "GenerateToken")

	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("csm-authorization-tenant-service").Start(ctx, "tenantGenerateToken")
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
func (t *TelmetryMW) RefreshToken(context.Context, *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
}
func (t *TelmetryMW) RevokeTenant(context.Context, *pb.RevokeTenantRequest) (*pb.RevokeTenantResponse, error) {
}
func (t *TelmetryMW) CancelRevokeTenant(context.Context, *pb.CancelRevokeTenantRequest) (*pb.CancelRevokeTenantResponse, error) {
}

func (t *TelmetryMW) timeSince(start time.Time, fName string) {
	t.log.WithFields(logrus.Fields{
		"duration": fmt.Sprintf("%v", time.Since(start)),
		"function": fName,
	}).Info("Function duration")
}
