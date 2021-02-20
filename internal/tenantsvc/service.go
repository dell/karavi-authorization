package tenantsvc

import (
	"context"
	"fmt"
	"karavi-authorization/pb"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrTenantAlreadyExists = status.Error(codes.InvalidArgument, "tenant already exists")
	ErrTenantNotFound      = status.Error(codes.InvalidArgument, "tenant not found")
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
	return t.createOrUpdateTenant(ctx, req.Tenant, false)
}

func (t *TenantService) GetTenant(ctx context.Context, req *pb.GetTenantRequest) (*pb.Tenant, error) {
	m, err := t.rdb.HGetAll(tenantKey(req.Name)).Result()
	if err != nil {
		return nil, err
	}

	if len(m) == 0 {
		return nil, ErrTenantNotFound
	}

	roles, err := t.rdb.SMembers(tenantRolesKey(req.Name)).Result()
	if err != nil {
		return nil, err
	}

	return &pb.Tenant{
		Name:  req.Name,
		Roles: strings.Join(roles, ","),
	}, nil
}

func (t *TenantService) UpdateTenant(ctx context.Context, req *pb.UpdateTenantRequest) (*pb.Tenant, error) {
	return t.createOrUpdateTenant(ctx, req.Tenant, true)
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

func (t *TenantService) BindRole(ctx context.Context, req *pb.BindRoleRequest) (*pb.BindRoleResponse, error) {
	// Update a set with role -> tenants mappings
	t.rdb.SAdd(rolesTenantKey(req.RoleName), req.TenantName)
	// Update a set with tenant -> roles mappings
	t.rdb.SAdd(tenantRolesKey(req.TenantName), req.RoleName)

	return &pb.BindRoleResponse{}, nil
}

func (t *TenantService) UnbindRole(ctx context.Context, req *pb.UnbindRoleRequest) (*pb.UnbindRoleResponse, error) {
	// Update a set with role -> tenants mappings
	t.rdb.SRem(rolesTenantKey(req.RoleName), req.TenantName)
	// Update a set with tenant -> roles mappings
	t.rdb.SRem(tenantRolesKey(req.TenantName), req.RoleName)

	return &pb.UnbindRoleResponse{}, nil
}

func (t *TenantService) createOrUpdateTenant(ctx context.Context, v *pb.Tenant, isUpdate bool) (*pb.Tenant, error) {
	if v == nil {
		return nil, ErrNilTenant
	}

	exists, err := t.rdb.Exists(tenantKey(v.Name)).Result()
	if err != nil {
		return nil, err
	}
	if isUpdate && exists == 0 {
		return nil, ErrTenantNotFound
	}
	if !isUpdate && exists == 1 {
		return nil, ErrTenantAlreadyExists
	}

	_, err = t.rdb.HSet(tenantKey(v.Name), "created_at", time.Now().Unix()).Result()
	if err != nil {
		return nil, err
	}

	return &pb.Tenant{
		Name:  v.Name,
		Roles: v.Roles,
	}, nil
}

func tenantKey(name string) string {
	return fmt.Sprintf("tenant:%s", name)
}

func tenantRolesKey(name string) string {
	return fmt.Sprintf("tenant:%s:roles", name)
}

func rolesTenantKey(name string) string {
	return fmt.Sprintf("role:%s:tenants", name)
}
