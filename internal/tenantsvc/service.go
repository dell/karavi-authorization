package tenantsvc

import (
	"context"
	"errors"
	"fmt"
	"karavi-authorization/pb"
	"log"
	"time"

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

func WithRedis(rdb *redis.Client) func(*TenantService) {
	return func(t *TenantService) {
		t.rdb = rdb
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

	key := fmt.Sprintf("tenant:%s", req.Tenant.Name)
	b, err := t.rdb.HSet(key, "created_at", time.Now().Unix()).Result()
	if err != nil {
		return nil, err
	}

	if !b {
		return nil, errors.New("tenant not created")
	}

	return &pb.Tenant{
		Name: req.Tenant.Name,
	}, nil
}

func (t *TenantService) GetTenant(ctx context.Context, req *pb.GetTenantRequest) (*pb.Tenant, error) {
	t.log.Printf("getting tenant: %+v", req)
	key := fmt.Sprintf("tenant:%s", req.Name)
	m, err := t.rdb.HGetAll(key).Result()
	if err != nil {
		return nil, err
	}

	log.Println(m)

	return &pb.Tenant{
		Name: req.Name,
	}, nil
}

func (t *TenantService) UpdateTenant(ctx context.Context, req *pb.UpdateTenantRequest) (*pb.Tenant, error) {
	t.log.Printf("updating tenant: %+v", req)
	return &pb.Tenant{}, nil
}

func (t *TenantService) DeleteTenant(ctx context.Context, req *pb.DeleteTenantRequest) (*empty.Empty, error) {
	t.log.Printf("deleting tenant: %+v", req)
	return &empty.Empty{}, nil
}

func (t *TenantService) ListTenant(ctx context.Context, req *pb.ListTenantRequest) (*pb.ListTenantResponse, error) {
	t.log.Printf("listing tenants: %+v", req)
	return &pb.ListTenantResponse{}, nil
}
