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

package tenantsvc_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"karavi-authorization/internal/tenantsvc"
	"karavi-authorization/pb"
	"log"
	"strings"
	"testing"

	"github.com/go-redis/redis"
	"github.com/orlangure/gnomock"
	"gopkg.in/yaml.v2"
)

// Common values.
const (
	RefreshToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOiJrYXJhdmkiLCJleHAiOjE2MTU1ODU4ODMsImlzcyI6ImNvbS5kZWxsLmthcmF2aSIsInN1YiI6ImthcmF2aS10ZW5hbnQiLCJyb2xlIjoiQ0EtbWVkaXVtIiwiZ3JvdXAiOiJQYW5jYWtlR3JvdXAifQ.NIH-Y0xXudw57a8gITX19Ye1irgL1pyKsc1C0B1pbgA"
	AccessToken  = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOiJrYXJhdmkiLCJleHAiOjExMTQ0ODQ4ODMsImlzcyI6ImNvbS5kZWxsLmthcmF2aSIsInN1YiI6ImthcmF2aS10ZW5hbnQiLCJyb2xlIjoiQ0EtbWVkaXVtIiwiZ3JvdXAiOiJQYW5jYWtlR3JvdXAifQ.RC24I_DhWdRB73voxMApOTQzb0AaYtYvGhqeAJ0vmAM"
)

type AfterFunc func()

func TestTenantService(t *testing.T) {
	rdb := createRedisContainer(t)
	sut := tenantsvc.NewTenantService(
		tenantsvc.WithRedis(rdb),
		tenantsvc.WithJWTSigningSecret("secret"))

	afterFn := func() {
		if _, err := rdb.FlushDB().Result(); err != nil {
			t.Fatalf("error flushing db: %+v", err)
		}
	}

	t.Run("CreateTenant", testCreateTenant(sut, afterFn))
	t.Run("GetTenant", testGetTenant(sut, afterFn))
	t.Run("DeleteTenant", testDeleteTenant(sut, afterFn))
	t.Run("ListTenant", testListTenant(sut, rdb, afterFn))
	t.Run("BindRole", testBindRole(sut, rdb, afterFn))
	t.Run("UnbindRole", testUnbindRole(sut, rdb, afterFn))
	t.Run("GenerateToken", testGenerateToken(sut, rdb, afterFn))
	t.Run("RefreshToken", testRefreshToken(sut, rdb, afterFn))
	t.Run("RevokeTenant", testRevokeTenant(sut, rdb, afterFn))
	t.Run("CancelRevokeTenant", testCancelRevokeTenant(sut, rdb, afterFn))
}

func testCreateTenant(sut *tenantsvc.TenantService, afterFn AfterFunc) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("it creates a tenant entry", func(t *testing.T) {
			defer afterFn()

			wantName := "tenant"
			got, err := sut.CreateTenant(context.Background(), &pb.CreateTenantRequest{
				Tenant: &pb.Tenant{
					Name:  wantName,
					Roles: "role1,role2",
				},
			})
			checkError(t, err)

			if got.Name != wantName {
				t.Errorf("CreateTenant: got name = %q, want %q", got.Name, wantName)
			}
		})
		t.Run("it errors on a duplicate tenant", func(t *testing.T) {
			first, err := sut.CreateTenant(context.Background(), &pb.CreateTenantRequest{
				Tenant: &pb.Tenant{
					Name: "duplicateme",
				},
			})
			checkError(t, err)

			gotTenant, gotErr := sut.CreateTenant(context.Background(), &pb.CreateTenantRequest{
				Tenant: first,
			})

			wantErr := tenantsvc.ErrTenantAlreadyExists
			if gotErr != tenantsvc.ErrTenantAlreadyExists {
				t.Errorf("CreateTenant: got err = %v, want %v", gotErr, wantErr)
			}
			if gotTenant != nil {
				t.Error("CreateTenant: expected returned tenant to be nil")
			}
		})
	}
}

func testGetTenant(sut *tenantsvc.TenantService, afterFn AfterFunc) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("it gets a created tenant", func(t *testing.T) {
			defer afterFn()
			wantName := "tenant-1"
			_, err := sut.CreateTenant(context.Background(), &pb.CreateTenantRequest{
				Tenant: &pb.Tenant{
					Name: wantName,
				},
			})
			if err != nil {
				t.Fatal(err)
			}

			got, err := sut.GetTenant(context.Background(), &pb.GetTenantRequest{
				Name: wantName,
			})

			if got.Name != wantName {
				t.Errorf("GetTenant: got name = %q, want %q", got.Name, wantName)
			}
		})
		t.Run("it shows any bound roles", func(t *testing.T) {
			defer afterFn()
			tenantName := "tenant-1"
			roleName := "role-1"
			createTenant(t, sut, tenantConfig{Name: tenantName, Roles: roleName})

			got, err := sut.GetTenant(context.Background(), &pb.GetTenantRequest{
				Name: tenantName,
			})
			checkError(t, err)
			wantName := tenantName
			if got.Name != wantName {
				t.Errorf("got name = %q, want %q", got.Name, wantName)
			}
			wantRoles := roleName
			if got.Roles != wantRoles {
				t.Errorf("got roles = %v, want %v", got.Roles, wantRoles)
			}
		})
		t.Run("it returns redis errors", func(t *testing.T) {
			defer afterFn()
			_, err := sut.GetTenant(context.Background(), &pb.GetTenantRequest{
				Name: "tenant",
			})

			if err == nil {
				t.Error("expected non-nil error")
			}
		})
	}
}

func testDeleteTenant(sut *tenantsvc.TenantService, afterFn AfterFunc) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("it deletes an existing tenant", func(t *testing.T) {
			defer afterFn()
			name := "testname"
			createTenant(t, sut, tenantConfig{Name: name})

			_, err := sut.DeleteTenant(context.Background(), &pb.DeleteTenantRequest{
				Name: name,
			})
			checkError(t, err)

			_, gotErr := sut.GetTenant(context.Background(), &pb.GetTenantRequest{
				Name: name,
			})
			if gotErr == nil {
				t.Error("DeleteTenant: expected non-nil error")
			}
		})
		t.Run("it errors on a non-existent tenant", func(t *testing.T) {
			defer afterFn()
			_, gotErr := sut.DeleteTenant(context.Background(), &pb.DeleteTenantRequest{
				Name: "doesnotexist",
			})

			wantErr := tenantsvc.ErrTenantNotFound
			if gotErr != wantErr {
				t.Errorf("DeleteTenant: got err %v, want %v", gotErr, wantErr)
			}
		})
	}
}

func testBindRole(sut *tenantsvc.TenantService, rdb *redis.Client, afterFn AfterFunc) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("it creates a role binding", func(t *testing.T) {
			defer afterFn()
			tenantName := "testname"
			roleName := "role-1"
			createTenant(t, sut, tenantConfig{Name: tenantName})

			_, err := sut.BindRole(context.Background(), &pb.BindRoleRequest{
				TenantName: tenantName,
				RoleName:   roleName,
			})
			checkError(t, err)

			got, err := sut.GetTenant(context.Background(), &pb.GetTenantRequest{
				Name: tenantName,
			})
			checkError(t, err)
			if got.Roles != roleName {
				t.Errorf("got roles %q, want %q", got.Roles, roleName)
			}
		})
	}
}

func testUnbindRole(sut *tenantsvc.TenantService, rdb *redis.Client, afterFn AfterFunc) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("it deletes a role binding", func(t *testing.T) {
			defer afterFn()
			tenantName := "tenant-1"
			roleName := "role-1"
			createTenant(t, sut, tenantConfig{Name: tenantName, Roles: roleName})

			_, err := sut.UnbindRole(context.Background(), &pb.UnbindRoleRequest{
				TenantName: tenantName,
				RoleName:   roleName,
			})
			checkError(t, err)

			got, err := sut.GetTenant(context.Background(), &pb.GetTenantRequest{
				Name: tenantName,
			})
			checkError(t, err)
			if got.Roles != "" {
				t.Errorf("got roles %q, want %q", got.Roles, "")
			}
		})
	}
}

func testListTenant(sut *tenantsvc.TenantService, rdb *redis.Client, afterFn AfterFunc) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("it lists existing tenants", func(t *testing.T) {
			defer afterFn()
			for i := 0; i < 5; i++ {
				createTenant(t, sut, tenantConfig{Name: fmt.Sprintf("tenant-%d", i)})
			}

			res, err := sut.ListTenant(context.Background(), &pb.ListTenantRequest{})
			checkError(t, err)

			wantLen := 5
			if gotLen := len(res.Tenants); gotLen != wantLen {
				t.Errorf("got len = %d, want %d", gotLen, wantLen)
			}
		})
	}
}

func testGenerateToken(sut *tenantsvc.TenantService, rdb *redis.Client, afterFn AfterFunc) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("it generates a token", func(t *testing.T) {
			defer afterFn()
			name := "tenant"
			createTenant(t, sut, tenantConfig{Name: name, Roles: "role-1"})

			got, err := sut.GenerateToken(context.Background(), &pb.GenerateTokenRequest{
				TenantName: name,
			})
			checkError(t, err)

			if got.Token == "" {
				t.Errorf("got %q, want non-empty", got.Token)
			}
		})
		t.Run("it handles an unknown tenant", func(t *testing.T) {
			defer afterFn()

			_, err := sut.GenerateToken(context.Background(), &pb.GenerateTokenRequest{
				TenantName: "unknown",
			})

			want := tenantsvc.ErrTenantNotFound
			if got := err; got != want {
				t.Errorf("got err = %+v, want %+v", got, want)
			}
		})
		t.Run("it handles a tenant with zero roles", func(t *testing.T) {
			defer afterFn()
			name := "tenant"
			createTenant(t, sut, tenantConfig{Name: name})

			_, err := sut.GenerateToken(context.Background(), &pb.GenerateTokenRequest{
				TenantName: name,
			})

			want := tenantsvc.ErrNoRolesForTenant
			if got := err; got != want {
				t.Errorf("got err = %+v, want %+v", got, want)
			}
		})
	}
}

func testRefreshToken(sut *tenantsvc.TenantService, rdb *redis.Client, afterFn AfterFunc) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("it refreshes a token", func(t *testing.T) {
			defer afterFn()

			got, err := sut.RefreshToken(context.Background(), &pb.RefreshTokenRequest{
				RefreshToken:     RefreshToken,
				AccessToken:      AccessToken,
				JWTSigningSecret: "secret",
			})
			checkError(t, err)

			if got.AccessToken == "" {
				t.Errorf("expected non-empty access token, but it was empty")
			}
		})
		t.Run("it handles an invalid refresh token", func(t *testing.T) {
			defer afterFn()

			_, err := sut.RefreshToken(context.Background(), &pb.RefreshTokenRequest{
				RefreshToken:     "invalid",
				AccessToken:      AccessToken,
				JWTSigningSecret: "secret",
			})

			if err == nil {
				t.Errorf("expected a non-nil error, but got nil")
			}
		})
		t.Run("it returns an error for a revoked tenant", func(t *testing.T) {
			defer afterFn()
			name := "tenant-1"
			createTenant(t, sut, tenantConfig{Name: name, Roles: "role-1"})
			tkn, err := sut.GenerateToken(context.Background(), &pb.GenerateTokenRequest{
				TenantName: name,
			})
			checkError(t, err)
			_, err = sut.RevokeTenant(context.Background(), &pb.RevokeTenantRequest{
				TenantName: name,
			})
			checkError(t, err)
			tknData := tkn.Token
			var tokenData struct {
				Data struct {
					Refresh string `yaml:"refresh"`
					Access  string `yaml:"access"`
				} `yaml:"data"`
			}
			err = yaml.Unmarshal([]byte(tknData), &tokenData)
			checkError(t, err)
			decRefTkn, err := base64.StdEncoding.DecodeString(tokenData.Data.Refresh)
			checkError(t, err)
			decAccTkn, err := base64.StdEncoding.DecodeString(tokenData.Data.Access)
			checkError(t, err)

			_, err = sut.RefreshToken(context.Background(), &pb.RefreshTokenRequest{
				RefreshToken:     string(decRefTkn),
				AccessToken:      string(decAccTkn),
				JWTSigningSecret: "secret",
			})

			want := tenantsvc.ErrTenantIsRevoked
			if got := err; got != want {
				t.Errorf("got err = %+v, want %+v", got, want)
			}
		})
	}
}

func testRevokeTenant(sut *tenantsvc.TenantService, rdb *redis.Client, afterFn AfterFunc) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("it revokes access to a tenant", func(t *testing.T) {
			defer afterFn()
			name := "tenant"
			createTenant(t, sut, tenantConfig{Name: name})

			_, err := sut.RevokeTenant(context.Background(), &pb.RevokeTenantRequest{
				TenantName: name,
			})
			checkError(t, err)

			b, err := sut.CheckRevoked(context.Background(), name)
			checkError(t, err)
			if got, want := b, true; got != want {
				t.Errorf("CheckRevoked: got %v, want %v", got, want)
			}
		})
	}
}

func testCancelRevokeTenant(sut *tenantsvc.TenantService, rdb *redis.Client, afterFn AfterFunc) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("it cancels a revocation operation on a tenant", func(t *testing.T) {
			defer afterFn()
			name := "tenant"
			createTenant(t, sut, tenantConfig{Name: name, Revoked: true})

			_, err := sut.CancelRevokeTenant(context.Background(), &pb.CancelRevokeTenantRequest{
				TenantName: name,
			})
			checkError(t, err)

			b, err := sut.CheckRevoked(context.Background(), name)
			checkError(t, err)
			if got, want := b, false; got != want {
				t.Errorf("CheckRevoked: got %v, want %v", got, want)
			}
		})
	}
}

func checkError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

type tenantConfig struct {
	Name    string
	Roles   string
	Revoked bool
}

func createTenant(t *testing.T, svc *tenantsvc.TenantService, cfg tenantConfig) {
	t.Helper()

	tnt, err := svc.CreateTenant(context.Background(), &pb.CreateTenantRequest{
		Tenant: &pb.Tenant{
			Name: cfg.Name,
		},
	})
	checkError(t, err)

	if cfg.Roles != "" {
		split := strings.Split(cfg.Roles, ",")
		for _, rn := range split {
			_, err := svc.BindRole(context.Background(), &pb.BindRoleRequest{
				TenantName: tnt.Name,
				RoleName:   strings.TrimSpace(rn),
			})
			checkError(t, err)
		}
	}

	if cfg.Revoked {
		_, err := svc.RevokeTenant(context.Background(), &pb.RevokeTenantRequest{
			TenantName: tnt.Name,
		})
		checkError(t, err)
	}
}

func getTenant(t *testing.T, svc *tenantsvc.TenantService, name string) *pb.Tenant {
	res, err := svc.GetTenant(context.Background(), &pb.GetTenantRequest{
		Name: name,
	})
	checkError(t, err)
	return res
}

func createRedisContainer(t *testing.T) *redis.Client {
	redisContainer, err := gnomock.StartCustom(
		"docker.io/library/redis:latest",
		gnomock.NamedPorts{"db": gnomock.TCP(6379)},
		gnomock.WithDisableAutoCleanup())
	if err != nil {
		t.Fatalf("failed to start redis container: %+v", err)
	}
	rdb := redis.NewClient(&redis.Options{
		Addr: redisContainer.Address("db"),
	})
	t.Cleanup(func() {
		if err := rdb.Close(); err != nil {
			log.Printf("closing redis: %+v", err)
		}
		if err := gnomock.Stop(redisContainer); err != nil {
			log.Printf("stopping redis container: %+v", err)
		}
	})

	return rdb
}
