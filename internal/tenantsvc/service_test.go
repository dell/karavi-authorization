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
	"fmt"
	"karavi-authorization/internal/tenantsvc"
	"karavi-authorization/pb"
	"log"
	"testing"

	"github.com/go-redis/redis"
	"github.com/orlangure/gnomock"
)

type AfterFunc func()

func TestTenantService(t *testing.T) {
	rdb := createRedisContainer(t)
	sut := tenantsvc.NewTenantService(tenantsvc.WithRedis(rdb))

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
}

func testCreateTenant(sut *tenantsvc.TenantService, afterFn AfterFunc) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("it creates a tenant entry", func(t *testing.T) {
			defer afterFn()

			wantName := "Avengers"
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

			wantName := "Avengers2"
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

			_, err := sut.CreateTenant(context.Background(), &pb.CreateTenantRequest{
				Tenant: &pb.Tenant{
					Name: "Avengers",
				},
			})
			checkError(t, err)

			_, err = sut.BindRole(context.Background(), &pb.BindRoleRequest{
				TenantName: "Avengers",
				RoleName:   "Role1",
			})

			got, err := sut.GetTenant(context.Background(), &pb.GetTenantRequest{
				Name: "Avengers",
			})

			wantName := "Avengers"
			if got.Name != wantName {
				t.Errorf("got name = %q, want %q", got.Name, wantName)
			}
			wantRoles := "Role1"
			if got.Roles != wantRoles {
				t.Errorf("got roles = %v, want %v", got.Roles, wantRoles)
			}
		})
	}
}

func testDeleteTenant(sut *tenantsvc.TenantService, afterFn AfterFunc) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("it deletes an existing tenant", func(t *testing.T) {
			defer afterFn()
			tenantName := "testname"
			_, err := sut.CreateTenant(context.Background(), &pb.CreateTenantRequest{
				Tenant: &pb.Tenant{
					Name: tenantName,
				},
			})
			checkError(t, err)

			_, err = sut.DeleteTenant(context.Background(), &pb.DeleteTenantRequest{
				Name: tenantName,
			})
			checkError(t, err)
			_, gotErr := sut.GetTenant(context.Background(), &pb.GetTenantRequest{
				Name: tenantName,
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
			_, err := sut.CreateTenant(context.Background(), &pb.CreateTenantRequest{
				Tenant: &pb.Tenant{
					Name: tenantName,
				},
			})
			checkError(t, err)

			_, err = sut.BindRole(context.Background(), &pb.BindRoleRequest{
				TenantName: tenantName,
				RoleName:   "Role1",
			})
			checkError(t, err)

			got, err := sut.GetTenant(context.Background(), &pb.GetTenantRequest{
				Name: tenantName,
			})
			if got.Roles != "Role1" {
				t.Errorf("got roles %q, want %q", got.Roles, "Role1")
			}
		})
	}
}

func testUnbindRole(sut *tenantsvc.TenantService, rdb *redis.Client, afterFn AfterFunc) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("it deletes a role binding", func(t *testing.T) {
			defer afterFn()
			tenantName := "testname"
			_, err := sut.CreateTenant(context.Background(), &pb.CreateTenantRequest{
				Tenant: &pb.Tenant{
					Name: tenantName,
				},
			})
			checkError(t, err)
			_, err = sut.BindRole(context.Background(), &pb.BindRoleRequest{
				TenantName: tenantName,
				RoleName:   "Role1",
			})
			checkError(t, err)
			got, err := sut.GetTenant(context.Background(), &pb.GetTenantRequest{
				Name: tenantName,
			})
			if got.Roles != "Role1" {
				t.Errorf("got roles %q, want %q", got.Roles, "Role1")
			}

			_, err = sut.UnbindRole(context.Background(), &pb.UnbindRoleRequest{
				TenantName: tenantName,
				RoleName:   "Role1",
			})
			checkError(t, err)
			got, err = sut.GetTenant(context.Background(), &pb.GetTenantRequest{
				Name: tenantName,
			})

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
				_, err := sut.CreateTenant(context.Background(), &pb.CreateTenantRequest{
					Tenant: &pb.Tenant{
						Name: fmt.Sprintf("tenant-%d", i),
					},
				})
				checkError(t, err)
			}

			res, err := sut.ListTenant(context.Background(), &pb.ListTenantRequest{})
			checkError(t, err)

			wantLen := 5
			if gotLen := len(res.Tenants); gotLen != wantLen {
				t.Errorf("got len = %d, want %d", gotLen, wantLen)
			}
		})
		t.Run("it ignores unintended tenant keys", func(t *testing.T) {
			defer afterFn()
			// In Redis, the keys for tenants are "tenant:name" and for listing
			// we scan through these keys, extract the name part and return that
			// as a list entry.  Redis may also have extra components, e.g.
			// "tenant:foo:bar", so we want to ignore the listing incorrectly
			// thinking that "foo" in this case is a tenant.
			_, err := sut.CreateTenant(context.Background(), &pb.CreateTenantRequest{
				Tenant: &pb.Tenant{
					Name: "testname",
				},
			})
			checkError(t, err)

			err = rdb.Set("tenant:foo:bar", "1", 0).Err()
			checkError(t, err)

			res, err := sut.ListTenant(context.Background(), &pb.ListTenantRequest{})
			checkError(t, err)

			wantLen := 1
			if gotLen := len(res.Tenants); gotLen != wantLen {
				t.Errorf("got len = %d, want %d", gotLen, wantLen)
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
