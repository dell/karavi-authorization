package tenantsvc_test

import (
	"context"
	"karavi-authorization/internal/tenantsvc"
	"karavi-authorization/pb"
	"log"
	"testing"

	"github.com/go-redis/redis"
	"github.com/orlangure/gnomock"
)

func TestTenantService(t *testing.T) {
	rdb := createRedisContainer(t)
	sut := tenantsvc.NewTenantService(tenantsvc.WithRedis(rdb))

	t.Run("CreateTenant", testCreateTenant(sut))
	t.Run("GetTenant", testGetTenant(sut))
}

func testCreateTenant(sut *tenantsvc.TenantService) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("it creates a tenant entry", func(t *testing.T) {
			wantName := "Avengers"
			got, err := sut.CreateTenant(context.Background(), &pb.CreateTenantRequest{
				Tenant: &pb.Tenant{
					Name: wantName,
				},
			})
			if err != nil {
				t.Fatal(err)
			}

			if got.Name != wantName {
				t.Errorf("CreateTenant: got name = %q, want %q", got.Name, wantName)
			}
		})
	}
}

func testGetTenant(sut *tenantsvc.TenantService) func(*testing.T) {
	return func(t *testing.T) {
		t.Run("created tenant can be gotten", func(t *testing.T) {
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
