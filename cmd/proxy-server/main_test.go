// Copyright © 2021-2023 Dell Inc., or its subsidiaries. All Rights Reserved.
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

package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	cmd "karavi-authorization/cmd/karavictl/cmd"
	"karavi-authorization/internal/proxy"
	"karavi-authorization/internal/role-service"
	"karavi-authorization/internal/role-service/roles"
	mockStorage "karavi-authorization/internal/storage-service/mocks"
	"karavi-authorization/internal/tenantsvc"
	"karavi-authorization/internal/token/jwx"
	"karavi-authorization/pb"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/go-redis/redis"
	"github.com/orlangure/gnomock"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"sigs.k8s.io/yaml"
)

type tenantConfig struct {
	Name    string
	Roles   string
	Revoked bool
}

func TestProxy(t *testing.T) {
	t.Skip("TODO")
}

func TestUpdateConfiguration(t *testing.T) {
	v := viper.New()
	v.Set("certificate.crtfile", "testCrtFile")
	v.Set("certificate.keyfile", "testKeyFile")
	v.Set("certificate.rootcertificate", "testRootCertificate")
	v.Set("web.jwtsigningsecret", "testSecret")

	oldCfg := cfg
	cfg = Config{}

	oldJWTSigningSecret := JWTSigningSecret

	defer func() {
		cfg = oldCfg
		JWTSigningSecret = oldJWTSigningSecret
	}()

	updateConfiguration(v, logrus.NewEntry(logrus.StandardLogger()))

	if JWTSigningSecret != "testSecret" {
		t.Errorf("expeted web.jwtsigningsecret to be %v, got %v", "testSecret", JWTSigningSecret)
	}
}

func TestUpdateStorageSystems(t *testing.T) {
	// define the check function that will pass or fail tests
	type checkFn func(t *testing.T, err error,
		powerScaleSystems map[string]*proxy.PowerScaleSystem,
		powerFlexSystems map[string]*proxy.System,
		powerMaxSystems map[string]*proxy.PowerMaxSystem,
	)

	// define the tests
	tests := []struct {
		name               string
		storageSystemsFile string // file name in testdata folder
		checkFn            checkFn
	}{
		{
			"success",
			"storage-systems.yaml",
			func(t *testing.T, err error, powerScaleSystems map[string]*proxy.PowerScaleSystem, powerFlexSystems map[string]*proxy.System, powerMaxSystems map[string]*proxy.PowerMaxSystem) {
				if err != nil {
					t.Errorf("expected nil error, got %v", err)
				}

				if _, ok := powerScaleSystems["IsilonClusterName"]; !ok {
					t.Error("expected powerScale IsilonClusterName to be configured")
				}
				if _, ok := powerScaleSystems["isilonclustername"]; !ok {
					t.Error("expected powerScale isilonclustername to be configured")
				}

				if _, ok := powerFlexSystems["542a2d5f5122210f"]; !ok {
					t.Error("expected powerFlex 542a2d5f5122210f to be configured")
				}

				if _, ok := powerMaxSystems["1234567890"]; !ok {
					t.Error("expected powerMax 1234567890 to be configured")
				}
			},
		},
	}

	// run the tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Given
			logger := logrus.NewEntry(logrus.New())

			powerScaleHandler := proxy.NewPowerScaleHandler(logger, nil, "")
			powerFlexHandler := proxy.NewPowerFlexHandler(logger, nil, nil, "")
			powerMaxHandler := proxy.NewPowerMaxHandler(logger, nil, "")

			// When
			err := updateStorageSystems(logger, fmt.Sprintf("testdata/%s", tc.storageSystemsFile), powerFlexHandler, powerMaxHandler, powerScaleHandler)

			// Then
			tc.checkFn(t, err, powerScaleHandler.GetSystems(), powerFlexHandler.GetSystems(), powerMaxHandler.GetSystems())
		})
	}
}

func TestVolumesHandler(t *testing.T) {
	ctx := context.Background()
	log := logrus.New().WithContext(ctx)
	rdb := createRedisContainer(t)
	sut := tenantsvc.NewTenantService(
		tenantsvc.WithRedis(rdb),
		tenantsvc.WithJWTSigningSecret("secret"),
		tenantsvc.WithTokenManager(jwx.NewTokenManager(jwx.HS256)))

	tests := map[string]func(t *testing.T, ctx context.Context, rdb *redis.Client, log *logrus.Entry){
		"Successful run of One Role": func(t *testing.T, ctx context.Context, rdb *redis.Client, log *logrus.Entry) {
			// creates tenant and binds role by name
			name := "PancakeGroup-0"
			createTenant(t, sut, tenantConfig{Name: name, Roles: "CA-medium-0"})

			tkn, err := sut.GenerateToken(context.Background(), &pb.GenerateTokenRequest{
				TenantName: name,
			})

			tknData := tkn.Token
			var tokenData struct {
				Data struct {
					Access string `yaml:"access"`
				} `yaml:"data"`
			}
			err = yaml.Unmarshal([]byte(tknData), &tokenData)
			checkError(t, err)
			decAccTkn, err := base64.StdEncoding.DecodeString(tokenData.Data.Access)
			checkError(t, err)
			// Create role
			roleInstance, err := roles.NewInstance("CA-medium-0", "powerflex", "542a2d5f5122210f", "bronze", "9GB")
			checkError(t, err)

			rff := roles.NewJSON()
			err = rff.Add(roleInstance)
			checkError(t, err)

			getRolesFn := func(ctx context.Context) (*roles.JSON, error) {
				return &rff, nil
			}
			roleSvc := role.NewService(fakeRoleKube{GetConfiguredRolesFn: getRolesFn}, successfulRoleValidator{})

			// create storage client
			storageClient := &mockStorage.FakeStorageServiceClient{
				GetPowerflexVolumesFn: func(context.Context, *pb.GetPowerflexVolumesRequest, ...grpc.CallOption) (*pb.GetPowerflexVolumesResponse, error) {
					// Create fake volume details
					mockVolumeDetails := []*pb.Volume{
						{
							Name:     "k8s-6aac50817e",
							Size:     8,
							SystemId: "542a2d5f5122210f",
							Id:       "volumeId1",
							Pool:     "bronze",
						},
					}

					resp := pb.GetPowerflexVolumesResponse{
						Volume: mockVolumeDetails,
					}

					return &resp, nil
				},
			}

			// create volume
			rdb.HSetNX("quota:powerflex:542a2d5f5122210f:bronze:PancakeGroup-0:data", "vol:k8s-6aac50817e:capacity", 1)

			// list volumes test

			h := volumesHandler(&roleClientService{roleService: roleSvc}, &storageClientService{storageClient: storageClient}, rdb, jwx.NewTokenManager(jwx.HS256), log)
			w := httptest.NewRecorder()
			r, err := http.NewRequestWithContext(ctx, http.MethodGet, "/proxy/volumes/", nil)
			r.Header.Add("Authorization", "Bearer "+string(decAccTkn))

			checkError(t, err)

			h.ServeHTTP(w, r)

			// Check if volume details are returned
			var fakeVolumeList []*pb.Volume

			if err := json.Unmarshal(w.Body.Bytes(), &fakeVolumeList); err != nil {
				t.Fatal(err)
			}

			got := fakeVolumeList
			want := []*pb.Volume{
				{
					Name:     "k8s-6aac50817e",
					Size:     8,
					SystemId: "542a2d5f5122210f",
					Id:       "volumeId1",
					Pool:     "bronze",
				},
			}

			if !reflect.DeepEqual(got, want) {
				t.Errorf("got %+v, expected response body to contain %+v", got, want)
			}
		},
		"Successful run of Multiple Roles": func(t *testing.T, ctx context.Context, rdb *redis.Client, log *logrus.Entry) {
			// creates tenant and binds role by name
			name := "PancakeGroup-1"
			createTenant(t, sut, tenantConfig{Name: name, Roles: "CA-medium-1,CA-large-1"})

			tkn, err := sut.GenerateToken(context.Background(), &pb.GenerateTokenRequest{
				TenantName: name,
			})

			tknData := tkn.Token
			var tokenData struct {
				Data struct {
					Access string `yaml:"access"`
				} `yaml:"data"`
			}
			err = yaml.Unmarshal([]byte(tknData), &tokenData)
			checkError(t, err)
			decAccTkn, err := base64.StdEncoding.DecodeString(tokenData.Data.Access)
			checkError(t, err)

			// Create Roles
			roleInstance, err := roles.NewInstance("CA-medium-1", "powerflex", "542a2d5f5122210f", "bronze", "9GB")
			roleInstanceTwo, err := roles.NewInstance("CA-large-1", "powerflex", "542a2d5f5122210f", "bronze", "20GB")
			checkError(t, err)

			rff := roles.NewJSON()

			err = rff.Add(roleInstance)
			checkError(t, err)

			err = rff.Add(roleInstanceTwo)
			checkError(t, err)

			getRolesFn := func(ctx context.Context) (*roles.JSON, error) {
				return &rff, nil
			}
			rolesSvc := role.NewService(fakeRoleKube{GetConfiguredRolesFn: getRolesFn}, successfulRoleValidator{})

			// create storage client
			storageClient := &mockStorage.FakeStorageServiceClient{
				GetPowerflexVolumesFn: func(context.Context, *pb.GetPowerflexVolumesRequest, ...grpc.CallOption) (*pb.GetPowerflexVolumesResponse, error) {
					// Create fake volume details
					mockVolumeDetails := []*pb.Volume{
						{
							Name:     "k8s-6aac50817e",
							Size:     8,
							SystemId: "542a2d5f5122210f",
							Id:       "volumeId1",
							Pool:     "bronze",
						},
					}

					resp := pb.GetPowerflexVolumesResponse{
						Volume: mockVolumeDetails,
					}

					return &resp, nil
				},
			}
			// create volume
			rdb.HSetNX("quota:powerflex:542a2d5f5122210f:bronze:PancakeGroup-1:data", "vol:k8s-6aac50817e:capacity", 1)

			// list volumes test

			h := volumesHandler(&roleClientService{roleService: rolesSvc}, &storageClientService{storageClient: storageClient}, rdb, jwx.NewTokenManager(jwx.HS256), log)
			w := httptest.NewRecorder()
			r, err := http.NewRequestWithContext(ctx, http.MethodGet, "/proxy/volumes/", nil)
			r.Header.Add("Authorization", "Bearer "+string(decAccTkn))

			checkError(t, err)

			h.ServeHTTP(w, r)

			// Check if volume details are returned
			var fakeVolumeList []*pb.Volume

			if err := json.Unmarshal(w.Body.Bytes(), &fakeVolumeList); err != nil {
				t.Fatal(err)
			}

			got := fakeVolumeList
			want := []*pb.Volume{
				{
					Name:     "k8s-6aac50817e",
					Size:     8,
					SystemId: "542a2d5f5122210f",
					Id:       "volumeId1",
					Pool:     "bronze",
				},
			}

			if !reflect.DeepEqual(got, want) {
				t.Errorf("got %+v, expected response body to contain %+v", got, want)
			}
		},
		"Unsuccessfull run of HGET failing": func(t *testing.T, ctx context.Context, rdb *redis.Client, log *logrus.Entry) {
			// creates tenant and binds role by name
			name := "PancakeGroup-2"
			createTenant(t, sut, tenantConfig{Name: name, Roles: "CA-medium-2"})

			tkn, err := sut.GenerateToken(context.Background(), &pb.GenerateTokenRequest{
				TenantName: name,
			})

			tknData := tkn.Token
			var tokenData struct {
				Data struct {
					Access string `yaml:"access"`
				} `yaml:"data"`
			}
			err = yaml.Unmarshal([]byte(tknData), &tokenData)
			checkError(t, err)
			decAccTkn, err := base64.StdEncoding.DecodeString(tokenData.Data.Access)
			checkError(t, err)
			// create No Roles
			rff := roles.NewJSON()

			getRolesFn := func(ctx context.Context) (*roles.JSON, error) {
				return &rff, nil
			}
			roleSvc := role.NewService(fakeRoleKube{GetConfiguredRolesFn: getRolesFn}, successfulRoleValidator{})

			// create storage client
			storageClient := &mockStorage.FakeStorageServiceClient{
				GetPowerflexVolumesFn: func(context.Context, *pb.GetPowerflexVolumesRequest, ...grpc.CallOption) (*pb.GetPowerflexVolumesResponse, error) {
					// Create fake volume details
					mockVolumeDetails := []*pb.Volume{
						{
							Name:     "k8s-6aac50817e",
							Size:     8,
							SystemId: "542a2d5f5122210f",
							Id:       "volumeId1",
							Pool:     "bronze",
						},
					}

					resp := pb.GetPowerflexVolumesResponse{
						Volume: mockVolumeDetails,
					}

					return &resp, nil
				},
			}

			// create volume
			rdb.HSetNX("quota:powerflex:542a2d5f5122210f:bronze:PancakeGroup-2:data", "vol:k8s-6aac50817e:capacity", 1)

			// list volumes test

			h := volumesHandler(&roleClientService{roleService: roleSvc}, &storageClientService{storageClient: storageClient}, rdb, jwx.NewTokenManager(jwx.HS256), log)
			w := httptest.NewRecorder()
			r, err := http.NewRequestWithContext(ctx, http.MethodGet, "/proxy/volumes/", nil)
			r.Header.Add("Authorization", "Bearer "+string(decAccTkn))

			checkError(t, err)

			h.ServeHTTP(w, r)

			// check if endpoint returns internalErrorServer status
			if got := w.Result().StatusCode; got != http.StatusInternalServerError {
				t.Errorf("got %d, want %d", got, http.StatusInternalServerError)
			}
			return
		},
		"Successfull run of multiple pools": func(t *testing.T, ctx context.Context, rdb *redis.Client, log *logrus.Entry) {
			// creates tenant and binds role by name
			name := "PancakeGroup-3"
			createTenant(t, sut, tenantConfig{Name: name, Roles: "CA-medium-3,CA-large-3"})

			tkn, err := sut.GenerateToken(context.Background(), &pb.GenerateTokenRequest{
				TenantName: name,
			})

			tknData := tkn.Token
			var tokenData struct {
				Data struct {
					Access string `yaml:"access"`
				} `yaml:"data"`
			}
			err = yaml.Unmarshal([]byte(tknData), &tokenData)
			checkError(t, err)
			decAccTkn, err := base64.StdEncoding.DecodeString(tokenData.Data.Access)
			checkError(t, err)
			// create Roles
			roleInstance, err := roles.NewInstance("CA-medium-3", "powerflex", "542a2d5f5122210f", "bronze", "9GB")
			roleInstanceTwo, err := roles.NewInstance("CA-large-3", "powerflex", "542a2d5f5122210f", "steel", "20GB")
			checkError(t, err)

			rff := roles.NewJSON()

			err = rff.Add(roleInstance)
			checkError(t, err)

			err = rff.Add(roleInstanceTwo)
			checkError(t, err)

			getRolesFn := func(ctx context.Context) (*roles.JSON, error) {
				return &rff, nil
			}
			roleSvc := role.NewService(fakeRoleKube{GetConfiguredRolesFn: getRolesFn}, successfulRoleValidator{})

			// create storage client
			storageClient := &mockStorage.FakeStorageServiceClient{
				GetPowerflexVolumesFn: func(context.Context, *pb.GetPowerflexVolumesRequest, ...grpc.CallOption) (*pb.GetPowerflexVolumesResponse, error) {
					// Create fake volume details
					mockVolumeDetails := []*pb.Volume{
						{
							Name:     "k8s-6aac50817e",
							Size:     8,
							SystemId: "542a2d5f5122210f",
							Id:       "volumeId1",
							Pool:     "bronze",
						},
					}

					resp := pb.GetPowerflexVolumesResponse{
						Volume: mockVolumeDetails,
					}

					return &resp, nil
				},
			}

			// create volume
			rdb.HSetNX("quota:powerflex:542a2d5f5122210f:bronze:PancakeGroup-3:data", "vol:k8s-6aac50817e:capacity", 1)
			rdb.HSetNX("quota:powerflex:542a2d5f5122210f:steel:PancakeGroup-3:data", "vol:k8s-6aac50818e:capacity", 1)

			// list volumes test

			h := volumesHandler(&roleClientService{roleService: roleSvc}, &storageClientService{storageClient: storageClient}, rdb, jwx.NewTokenManager(jwx.HS256), log)
			w := httptest.NewRecorder()
			r, err := http.NewRequestWithContext(ctx, http.MethodGet, "/proxy/volumes/", nil)
			r.Header.Add("Authorization", "Bearer "+string(decAccTkn))

			checkError(t, err)

			h.ServeHTTP(w, r)

			// Check if volume details are returned
			var fakeVolumeList []*pb.Volume

			if err := json.Unmarshal(w.Body.Bytes(), &fakeVolumeList); err != nil {
				t.Fatal(err)
			}

			got := fakeVolumeList
			want := []*pb.Volume{
				{
					Name:     "k8s-6aac50817e",
					Size:     8,
					SystemId: "542a2d5f5122210f",
					Id:       "volumeId1",
					Pool:     "bronze",
				},
			}

			if !reflect.DeepEqual(got, want) {
				t.Errorf("got %+v, expected response body to contain %+v", got, want)
			}
		},
		"Successfull run of deleted Role": func(t *testing.T, ctx context.Context, rdb *redis.Client, log *logrus.Entry) {
			// creates tenant and binds role by name
			name := "PancakeGroup-4"
			createTenant(t, sut, tenantConfig{Name: name, Roles: "CA-medium-4,CA-large-4"})

			tkn, err := sut.GenerateToken(context.Background(), &pb.GenerateTokenRequest{
				TenantName: name,
			})

			tknData := tkn.Token
			var tokenData struct {
				Data struct {
					Access string `yaml:"access"`
				} `yaml:"data"`
			}
			err = yaml.Unmarshal([]byte(tknData), &tokenData)
			checkError(t, err)
			decAccTkn, err := base64.StdEncoding.DecodeString(tokenData.Data.Access)
			checkError(t, err)
			// create Roles
			roleInstance, err := roles.NewInstance("CA-medium-4", "powerflex", "542a2d5f5122210f", "bronze", "9GB")
			roleInstanceTwo, err := roles.NewInstance("CA-large-4", "powerflex", "542a2d5f5122210f", "bronze", "20GB")
			checkError(t, err)

			rff := roles.NewJSON()

			err = rff.Add(roleInstance)
			checkError(t, err)

			err = rff.Add(roleInstanceTwo)
			checkError(t, err)

			getRolesFn := func(ctx context.Context) (*roles.JSON, error) {
				return &rff, nil
			}
			roleSvc := role.NewService(fakeRoleKube{GetConfiguredRolesFn: getRolesFn}, successfulRoleValidator{})

			// create storage client
			storageClient := &mockStorage.FakeStorageServiceClient{
				GetPowerflexVolumesFn: func(context.Context, *pb.GetPowerflexVolumesRequest, ...grpc.CallOption) (*pb.GetPowerflexVolumesResponse, error) {
					// Create fake volume details
					mockVolumeDetails := []*pb.Volume{
						{
							Name:     "k8s-6aac50817e",
							Size:     8,
							SystemId: "542a2d5f5122210f",
							Id:       "volumeId1",
							Pool:     "bronze",
						},
					}

					resp := pb.GetPowerflexVolumesResponse{
						Volume: mockVolumeDetails,
					}

					return &resp, nil
				},
			}

			// create volume
			rdb.HSetNX("quota:powerflex:542a2d5f5122210f:bronze:PancakeGroup-4:data", "vol:k8s-6aac50817e:capacity", 1)
			rdb.HSetNX("quota:powerflex:542a2d5f5122210f:bronze:PancakeGroup-4:data", "vol:k8s-6aac50818e:deleted", 1)

			// list volumes test

			h := volumesHandler(&roleClientService{roleService: roleSvc}, &storageClientService{storageClient: storageClient}, rdb, jwx.NewTokenManager(jwx.HS256), log)
			w := httptest.NewRecorder()
			r, err := http.NewRequestWithContext(ctx, http.MethodGet, "/proxy/volumes/", nil)
			r.Header.Add("Authorization", "Bearer "+string(decAccTkn))

			checkError(t, err)

			h.ServeHTTP(w, r)

			// Check if volume details are returned
			var fakeVolumeList []*pb.Volume

			if err := json.Unmarshal(w.Body.Bytes(), &fakeVolumeList); err != nil {
				t.Fatal(err)
			}

			got := fakeVolumeList
			want := []*pb.Volume{
				{
					Name:     "k8s-6aac50817e",
					Size:     8,
					SystemId: "542a2d5f5122210f",
					Id:       "volumeId1",
					Pool:     "bronze",
				},
			}

			if !reflect.DeepEqual(got, want) {
				t.Errorf("got %+v, expected response body to contain %+v", got, want)
			}
		},
	}

	// run the tests
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			tc(t, ctx, rdb, log)
		})
	}
}

func checkError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
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

func createRedisContainer(t *testing.T) *redis.Client {
	var rdb *redis.Client

	redisHost := os.Getenv("REDIS_HOST")
	redistPort := os.Getenv("REDIS_PORT")

	if redisHost != "" && redistPort != "" {
		rdb = redis.NewClient(&redis.Options{
			Addr: fmt.Sprintf("%s:%s", redisHost, redistPort),
		})
	} else {
		redisContainer, err := gnomock.StartCustom(
			"docker.io/library/redis:latest",
			gnomock.NamedPorts{"db": gnomock.TCP(6379)},
			gnomock.WithDisableAutoCleanup())
		if err != nil {
			t.Fatalf("failed to start redis container: %+v", err)
		}
		rdb = redis.NewClient(&redis.Options{
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
	}
	return rdb
}

type fakeRoleKube struct {
	UpdateRolesRn        func(ctx context.Context, roles *roles.JSON) error
	GetConfiguredRolesFn func(ctx context.Context) (*roles.JSON, error)
}

func (k fakeRoleKube) UpdateRoles(ctx context.Context, roles *roles.JSON) error {
	if k.UpdateRolesRn != nil {
		return k.UpdateRolesRn(ctx, roles)
	}
	return nil
}

func (k fakeRoleKube) GetConfiguredRoles(ctx context.Context) (*roles.JSON, error) {
	if k.GetConfiguredRolesFn != nil {
		return k.GetConfiguredRolesFn(ctx)
	}
	r := roles.NewJSON()
	return &r, nil
}

type fakeStorageKube struct {
	UpdateStoragesFn       func(ctx context.Context, storages cmd.Storage) error
	GetConfiguredStorageFn func(ctx context.Context) (cmd.Storage, error)
}

func (k fakeStorageKube) UpdateStorages(ctx context.Context, storages cmd.Storage) error {
	if k.UpdateStoragesFn != nil {
		return k.UpdateStoragesFn(ctx, storages)
	}
	return nil
}

func (k fakeStorageKube) GetConfiguredStorage(ctx context.Context) (cmd.Storage, error) {
	if k.GetConfiguredStorageFn != nil {
		return k.GetConfiguredStorageFn(ctx)
	}
	var s cmd.Storage
	return s, nil
}

type successfulRoleValidator struct{}

func (v successfulRoleValidator) Validate(_ context.Context, _ *roles.Instance) error {
	return nil
}

type successfulStorageValidator struct{}

func (v successfulStorageValidator) Validate(_ context.Context, _ string, _ string, _ cmd.System) error {
	return nil
}
