package tenantsvc_test

import (
	"context"
	"errors"
	"fmt"
	"karavi-authorization/internal/tenantsvc"
	"karavi-authorization/pb"
	"log"
	"strings"
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
	t.Run("UpdateTenant", testUpdateTenant(sut, afterFn))
	t.Run("DeleteTenant", testDeleteTenant(sut, afterFn))
	t.Run("ListTenant", testListTenant(sut, afterFn))
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
	}
}

func testUpdateTenant(sut *tenantsvc.TenantService, afterFn AfterFunc) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("it updates an existing tenant", func(t *testing.T) {
			defer afterFn()
			tenantName := "testname"
			roles := []string{"role1"}
			_, err := sut.CreateTenant(context.Background(), &pb.CreateTenantRequest{
				Tenant: &pb.Tenant{
					Name:  tenantName,
					Roles: strings.Join(roles, ","),
				},
			})
			checkError(t, err)

			roles = append(roles, "role2")
			got, err := sut.UpdateTenant(context.Background(), &pb.UpdateTenantRequest{
				Tenant: &pb.Tenant{
					Name:  tenantName,
					Roles: strings.Join(roles, ","),
				},
			})
			checkError(t, err)

			if got == nil {
				t.Error("UpdateTenant: expected a non-nil tenant value")
			}

			got, err = sut.GetTenant(context.Background(), &pb.GetTenantRequest{
				Name: tenantName,
			})

			wantRoles := strings.Join(roles, ",")
			if got.Roles != wantRoles {
				t.Errorf("got roles %q, want %q", got.Roles, wantRoles)
			}
		})
		t.Run("it errors when updating a missing tenant", func(t *testing.T) {
			defer afterFn()

			gotTenant, gotErr := sut.UpdateTenant(context.Background(), &pb.UpdateTenantRequest{
				Tenant: &pb.Tenant{
					Name: "missingtenant",
				},
			})

			wantErr := tenantsvc.ErrTenantNotFound
			if gotErr == nil {
				t.Errorf("UpdateTenant: got err = %v, want %v", gotErr, wantErr)
			}
			if gotTenant != nil {
				t.Error("UpdateTenant: expected a nil tenant value")
			}
		})
		t.Run("it handles a nil tenant", func(t *testing.T) {
			defer afterFn()

			_, gotErr := sut.UpdateTenant(context.Background(), &pb.UpdateTenantRequest{})

			wantErr := tenantsvc.ErrNilTenant
			if !errors.Is(gotErr, wantErr) {
				t.Errorf("UpdateTenant: got err = %v, want %v", gotErr, wantErr)
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

func testListTenant(sut *tenantsvc.TenantService, afterFn AfterFunc) func(*testing.T) {
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
	}
}

func checkError(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func createRedisContainer(t *testing.T) *redis.Client {
	redisContainer, err := gnomock.StartCustom(
		"docker.io/library/redis:latest",
		gnomock.NamedPorts{"db": gnomock.TCP(6379)},
		gnomock.WithDisableAutoCleanup(),
		gnomock.WithContainerName("redis-test"))
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
