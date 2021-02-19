package tenantsvc

import (
	"context"
	"karavi-authorization/pb"

	"github.com/go-redis/redis"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/sirupsen/logrus"
)

type TenantService struct {
	pb.UnimplementedTenantServiceServer

	log *logrus.Entry
	rdb *redis.Client
}

type Option func(*TenantService)

func defaultOptions() []Option {
	return []Option{
		WithLogger(logrus.NewEntry(logrus.New())),
	}
}

func WithLogger(log *logrus.Entry) func(*TenantService) {
	return func(t *TenantService) {
		t.log = log
	}
}

func NewTenantService(opts ...Option) *TenantService {
	var t TenantService
	for _, opt := range defaultOptions() {
		opt(&t)
	}
	for _, opt := range opts {
		opt(&t)
	}
	return &t
}

func (t *TenantService) CreateTenant(ctx context.Context, req *pb.CreateTenantRequest) (*pb.Tenant, error) {
	t.log.Printf("creating tenant: %+v", req)
	return nil, nil
}

func (t *TenantService) GetTenant(ctx context.Context, req *pb.GetTenantRequest) (*pb.Tenant, error) {
	t.log.Printf("getting tenant: %+v", req)
	return nil, nil
}

func (t *TenantService) UpdateTenant(ctx context.Context, req *pb.UpdateTenantRequest) (*pb.Tenant, error) {
	t.log.Printf("updating tenant: %+v", req)
	return nil, nil
}

func (t *TenantService) DeleteTenant(ctx context.Context, req *pb.DeleteTenantRequest) (*empty.Empty, error) {
	t.log.Printf("deleting tenant: %+v", req)
	return nil, nil
}

func (t *TenantService) ListTenant(ctx context.Context, req *pb.ListTenantRequest) (*pb.ListTenantResponse, error) {
	t.log.Printf("listing tenants: %+v", req)
	return nil, nil
}
