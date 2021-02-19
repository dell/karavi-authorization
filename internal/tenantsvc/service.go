package tenantsvc

import (
	"context"
	"errors"
	"fmt"
	"karavi-authorization/pb"
	"log"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrTenantAlreadyExists = errors.New("tenant already exists")
	ErrTenantNotFound      = errors.New("tenant not found")
	ErrNilTenant           = status.Error(codes.InvalidArgument, "nil tenant")
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
	key := fmt.Sprintf("tenant:%s", req.Tenant.Name)

	b, err := t.rdb.HSet(key, "created_at", time.Now().Unix()).Result()
	if err != nil {
		return nil, err
	}

	if !b {
		return nil, ErrTenantAlreadyExists
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
	if req.Tenant == nil {
		return nil, ErrNilTenant
	}

	exists, err := t.rdb.Exists(tenantKey(req.Tenant.Name)).Result()
	if err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, ErrTenantNotFound
	}

	// TODO(ian): Update tenant attributes here.
	return &pb.Tenant{}, nil
}

func (t *TenantService) DeleteTenant(ctx context.Context, req *pb.DeleteTenantRequest) (*empty.Empty, error) {
	var emp empty.Empty
	n, err := t.rdb.Del(tenantKey(req.Name)).Result()
	if err != nil {
		return &emp, err
	}
	if n == 0 {
		return nil, ErrTenantNotFound
	}

	return &emp, nil
}

func (t *TenantService) ListTenant(ctx context.Context, req *pb.ListTenantRequest) (*pb.ListTenantResponse, error) {
	var tenants []*pb.Tenant

	var cursor uint64
	for {
		keys, nextCursor, err := t.rdb.Scan(cursor, "tenant:*", 10).Result()
		if err != nil {
			return nil, err
		}
		for _, v := range keys {
			split := strings.Split(v, ":")
			if len(split) != 2 {
				continue
			}
			tenants = append(tenants, &pb.Tenant{
				Name: split[1],
			})
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return &pb.ListTenantResponse{
		Tenants: tenants,
	}, nil
}

func tenantKey(name string) string {
	return fmt.Sprintf("tenant:%s", name)
}
