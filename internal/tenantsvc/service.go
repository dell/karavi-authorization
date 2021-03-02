// Copyright © 2021 Dell Inc., or its subsidiaries. All Rights Reserved.
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

package tenantsvc

import (
	"context"
	"errors"
	"fmt"
	"karavi-authorization/internal/token"
	"karavi-authorization/pb"
	"log"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-redis/redis"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Common errors.
var (
	ErrTenantAlreadyExists = status.Error(codes.InvalidArgument, "tenant already exists")
	ErrTenantNotFound      = status.Error(codes.InvalidArgument, "tenant not found")
	ErrNilTenant           = status.Error(codes.InvalidArgument, "nil tenant")
)

// TenantService is the gRPC implementation of the TenantServiceServer.
type TenantService struct {
	pb.UnimplementedTenantServiceServer

	log              *logrus.Entry
	rdb              *redis.Client
	jwtSigningSecret string
}

// Option allows for functional option arguments on the TenantService.
type Option func(*TenantService)

func defaultOptions() []Option {
	return []Option{
		WithLogger(logrus.NewEntry(logrus.New())),
	}
}

// WithLogger provides a logger.
func WithLogger(log *logrus.Entry) func(*TenantService) {
	return func(t *TenantService) {
		t.log = log
	}
}

// WithRedis provides a redis client.
func WithRedis(rdb *redis.Client) func(*TenantService) {
	return func(t *TenantService) {
		t.rdb = rdb
	}
}

// WithJWTSigningSecret provides the JWT signing secret.
func WithJWTSigningSecret(s string) func(*TenantService) {
	return func(t *TenantService) {
		t.jwtSigningSecret = s
	}
}

// NewTenantService allocates a new TenantService.
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

// CreateTenant handles tenant creation requests.
func (t *TenantService) CreateTenant(ctx context.Context, req *pb.CreateTenantRequest) (*pb.Tenant, error) {
	return t.createOrUpdateTenant(ctx, req.Tenant, false)
}

// GetTenant handles tenant query requests.
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

// DeleteTenant handles tenant deletion requests.
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

// ListTenant handles tenant listing requests.
func (t *TenantService) ListTenant(ctx context.Context, req *pb.ListTenantRequest) (*pb.ListTenantResponse, error) {
	var tenants []*pb.Tenant

	var cursor uint64
	for {
		keys, nextCursor, err := t.rdb.Scan(cursor, "tenant:*:data", 10).Result()
		if err != nil {
			return nil, err
		}
		for _, v := range keys {
			split := strings.Split(v, ":")
			if len(split) != 3 { // 3 parts from having 2 colons, e.g. "foo:bar:baz"
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

// BindRole handles rolebinding creation requests.
func (t *TenantService) BindRole(ctx context.Context, req *pb.BindRoleRequest) (*pb.BindRoleResponse, error) {
	// Update a set with role -> tenants mappings
	t.rdb.SAdd(rolesTenantKey(req.RoleName), req.TenantName)
	// Update a set with tenant -> roles mappings
	t.rdb.SAdd(tenantRolesKey(req.TenantName), req.RoleName)

	return &pb.BindRoleResponse{}, nil
}

// UnbindRole handles rolebinding deletion requests.
func (t *TenantService) UnbindRole(ctx context.Context, req *pb.UnbindRoleRequest) (*pb.UnbindRoleResponse, error) {
	// Update a set with role -> tenants mappings
	t.rdb.SRem(rolesTenantKey(req.RoleName), req.TenantName)
	// Update a set with tenant -> roles mappings
	t.rdb.SRem(tenantRolesKey(req.TenantName), req.RoleName)

	return &pb.UnbindRoleResponse{}, nil
}

func (t *TenantService) GenerateToken(ctx context.Context, req *pb.GenerateTokenRequest) (*pb.GenerateTokenResponse, error) {
	// Check the tenant exists.
	exists, err := t.rdb.Exists(tenantKey(req.TenantName)).Result()
	if err != nil {
		return nil, err
	}
	if exists == 0 {
		return nil, ErrTenantNotFound
	}

	// Get the roles bound to this tenant.
	roles, err := t.rdb.SMembers(tenantRolesKey(req.TenantName)).Result()
	if err != nil {
		return nil, err
	}
	if len(roles) == 0 {
		return nil, errors.New("no roles for tenant")
	}

	// Get the expiration values from config.
	refreshExpDur := 24 * time.Hour
	accessExpDur := 5 * time.Minute

	// Generate the token.
	s, err := token.Create(req.TenantName, roles, t.jwtSigningSecret, refreshExpDur, accessExpDur)
	if err != nil {
		return nil, err
	}

	// Return the token.
	return &pb.GenerateTokenResponse{
		Token: s,
	}, nil
}

type Claims struct {
	jwt.StandardClaims
	Role  string `json:"role"`
	Group string `json:"group"`
}

func (t *TenantService) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	refreshToken := req.RefreshToken
	accessToken := req.AccessToken

	var refreshClaims Claims
	_, err := jwt.ParseWithClaims(refreshToken, &refreshClaims, func(t *jwt.Token) (interface{}, error) {
		return []byte(req.JWTSigningSecret), nil
	})
	if err != nil {
		log.Printf("parsing refresh token %q: %+v", refreshToken, err)
		return nil, err
	}

	// Check if the tenant is being denied.
	ok, err := t.rdb.SIsMember("tenant:deny", refreshClaims.Group).Result()
	if err != nil {
		log.Printf("%+v", err)
		return nil, err
	}
	if ok {
		log.Printf("user denied: %+v", err)
		return nil, errors.New("user has been denied")
	}

	var accessClaims Claims
	access, err := jwt.ParseWithClaims(accessToken, &accessClaims, func(t *jwt.Token) (interface{}, error) {
		return []byte(req.JWTSigningSecret), nil
	})
	if access.Valid {
		return nil, errors.New("access token was valid")
	} else if ve, ok := err.(*jwt.ValidationError); ok {
		switch {
		case ve.Errors&jwt.ValidationErrorExpired != 0:
			log.Println("Refreshing expired token for", accessClaims.Audience)
		default:
			log.Printf("%+v", err)
			return nil, err
		}
	}

	if tenant := strings.TrimSpace(accessClaims.Subject); tenant == "" {
		log.Printf("invalid tenant: %q", tenant)
		return nil, fmt.Errorf("invalid tenant: %q", tenant)
	}
	_, err = t.rdb.HIncrBy(
		"tenant:"+accessClaims.Group+":data",
		"refresh_count",
		1).Result()
	if err != nil {
		log.Printf("%+v", err)
		return nil, err
	}

	// Use the refresh token with a smaller expiration timestamp to be
	// the new access token.
	refreshClaims.ExpiresAt = time.Now().Add(30 * time.Second).Unix()
	newAccess := jwt.NewWithClaims(jwt.SigningMethodHS256, refreshClaims)
	newAccessStr, err := newAccess.SignedString([]byte(req.JWTSigningSecret))
	if err != nil {
		log.Printf("%+v", err)
		return nil, err
	}

	return &pb.RefreshTokenResponse{
		AccessToken: newAccessStr,
	}, nil
}

func (t *TenantService) RevokeTenant(ctx context.Context, req *pb.RevokeTenantRequest) (*pb.RevokeTenantResponse, error) {
	_, err := t.rdb.SAdd("tenant:deny", req.TenantName).Result()
	if err != nil {
		return nil, err
	}

	return &pb.RevokeTenantResponse{}, nil
}

func (t *TenantService) CancelRevokeTenant(ctx context.Context, req *pb.CancelRevokeTenantRequest) (*pb.CancelRevokeTenantResponse, error) {
	_, err := t.rdb.SRem("tenant:deny", req.TenantName).Result()
	if err != nil {
		return nil, err
	}

	return &pb.CancelRevokeTenantResponse{}, nil
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
	return fmt.Sprintf("tenant:%s:data", name)
}

func tenantRolesKey(name string) string {
	return fmt.Sprintf("tenant:%s:roles", name)
}

func rolesTenantKey(name string) string {
	return fmt.Sprintf("role:%s:tenants", name)
}
