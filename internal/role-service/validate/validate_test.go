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

package validate_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	storage "karavi-authorization/cmd/karavictl/cmd"
	"karavi-authorization/internal/k8s"
	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/internal/role-service/validate"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestValidatePowerFlex(t *testing.T) {
	// Happy paths
	t.Run("Success", func(t *testing.T) {
		// create mock backend pwoerflex
		goodBackendPowerFlex := httptest.NewTLSServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/login":
					fmt.Fprintf(w, `"token"`)
				case "/api/version":
					fmt.Fprintf(w, "3.5")
				case "/api/types/System/instances":
					write(t, w, "powerflex_api_types_System_instances_542a2d5f5122210f.json")
				case "/api/instances/System::542a2d5f5122210f/relationships/ProtectionDomain":
					write(t, w, "protection_domains.json")
				case "/api/instances/ProtectionDomain::0000000000000001/relationships/StoragePool":
					write(t, w, "storage_pools.json")
				case "/api/instances/StoragePool::7000000000000000/relationships/Statistics":
					write(t, w, "storage_pool_statistics.json")
				default:
					t.Errorf("unhandled request path: %s", r.URL.Path)
				}
			}))
		defer goodBackendPowerFlex.Close()

		// define check functions to pass or fail tests
		type checkFn func(*testing.T, error)

		errIsNil := func(t *testing.T, err error) {
			if err != nil {
				t.Errorf("expected nil err, got %v", err)
			}
		}

		// temporarily set k8s.GetPowerFlexEndpoint to mock powerflex
		oldGetPowerFlexEndpoint := validate.GetPowerFlexEndpoint
		validate.GetPowerFlexEndpoint = func(_ storage.System) string {
			return goodBackendPowerFlex.URL
		}
		defer func() { validate.GetPowerFlexEndpoint = oldGetPowerFlexEndpoint }()

		// define the tests
		tests := map[string]func(t *testing.T) (validate.Kube, *roles.Instance, checkFn){
			"success": func(_ *testing.T) (validate.Kube, *roles.Instance, checkFn) {
				// configure fake k8s with storage secret
				data := []byte(fmt.Sprintf(`
storage:
  powerflex:
    542a2d5f5122210f:
      endpoint: %s
      insecure: true
      password: Password123
      user: admin`, goodBackendPowerFlex.URL))

				secret := &v1.Secret{
					ObjectMeta: meta.ObjectMeta{
						Name:      k8s.StorageSecret,
						Namespace: "test",
					},
					Data: map[string][]byte{
						k8s.StorageSecretDataKey: data,
					},
				}

				fakeClient := fake.NewSimpleClientset(secret)

				role := &roles.Instance{
					RoleKey: roles.RoleKey{
						Name:       "success",
						SystemType: "powerflex",
						SystemID:   "542a2d5f5122210f",
						Pool:       "bronze",
					},
					Quota: 1000,
				}

				api := &k8s.API{
					Client:    fakeClient,
					Namespace: "test",
					Lock:      sync.Mutex{},
					Log:       logrus.NewEntry(logrus.StandardLogger()),
				}

				return api, role, errIsNil
			},
		}

		// run the tests
		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				kube, role, checkFn := tc(t)
				rv := validate.NewRoleValidator(kube, logrus.NewEntry(logrus.StandardLogger()))
				err := rv.Validate(context.Background(), role)
				checkFn(t, err)
			})
		}
	})

	// Error cases
	t.Run("Error", func(t *testing.T) {
		// create mock backend powerflex
		goodBackendPowerFlex := httptest.NewTLSServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/login":
					fmt.Fprintf(w, `"token"`)
				case "/api/version":
					fmt.Fprintf(w, "3.5")
				case "/api/types/System/instances":
					write(t, w, "powerflex_api_types_System_instances_542a2d5f5122210f.json")
				case "/api/instances/System::542a2d5f5122210f/relationships/ProtectionDomain":
					write(t, w, "protection_domains.json")
				case "/api/instances/ProtectionDomain::0000000000000001/relationships/StoragePool":
					write(t, w, "storage_pools.json")
				case "/api/instances/StoragePool::7000000000000000/relationships/Statistics":
					write(t, w, "storage_pool_statistics.json")
				default:
					t.Errorf("unhandled request path: %s", r.URL.Path)
				}
			}))
		defer goodBackendPowerFlex.Close()

		// define check functions to pass or fail tests
		type checkFn func(*testing.T, error)

		errIsNotNil := func(t *testing.T, err error) {
			if err == nil {
				t.Errorf("expected an err, got nil")
			}
		}

		// define the tests
		tests := map[string]func(t *testing.T) (validate.Kube, *roles.Instance, checkFn){
			"negative quota": func(_ *testing.T) (validate.Kube, *roles.Instance, checkFn) {
				// configure fake k8s with storage secret
				data := []byte(fmt.Sprintf(`
			storage:
				powerflex:
				542a2d5f5122210f:
					Endpoint: %s
					Insecure: true
					Password: Password123
					User: admin`, goodBackendPowerFlex.URL))

				secret := &v1.Secret{
					ObjectMeta: meta.ObjectMeta{
						Name:      k8s.StorageSecret,
						Namespace: "test",
					},
					Data: map[string][]byte{
						k8s.StorageSecretDataKey: data,
					},
				}

				fakeClient := fake.NewSimpleClientset(secret)

				role := &roles.Instance{
					RoleKey: roles.RoleKey{
						Name:       "negative quota",
						SystemType: "powerflex",
						SystemID:   "542a2d5f5122210f",
						Pool:       "bronze",
					},
					Quota: -1,
				}

				api := &k8s.API{
					Client:    fakeClient,
					Namespace: "test",
					Lock:      sync.Mutex{},
					Log:       logrus.NewEntry(logrus.StandardLogger()),
				}

				return api, role, errIsNotNil
			},
		}

		// run the tests
		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				kube, role, checkFn := tc(t)
				rv := validate.NewRoleValidator(kube, logrus.NewEntry(logrus.StandardLogger()))
				err := rv.Validate(context.Background(), role)
				checkFn(t, err)
			})
		}
	})
}

func TestValidatePowerMax(t *testing.T) {
	// Happy pahts
	t.Run("Success", func(t *testing.T) {
		// Creates a fake powermax handler
		goodBackendPowerMax := httptest.NewTLSServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/univmax/restapi/version":
					fmt.Fprintf(w, `{ "version": "V10.0.0.1"}`)
				case "/univmax/restapi/100/sloprovisioning/symmetrix/000197900714/srp/bronze":
					w.WriteHeader(http.StatusOK)
				default:
					t.Errorf("unhandled unisphere request path: %s", r.URL.Path)
				}
			}))
		defer goodBackendPowerMax.Close()

		oldGetPowerMaxEndpoint := validate.GetPowerMaxEndpoint
		validate.GetPowerMaxEndpoint = func(_ storage.System) string {
			return goodBackendPowerMax.URL
		}
		defer func() { validate.GetPowerMaxEndpoint = oldGetPowerMaxEndpoint }()

		// define check functions to pass or fail tests
		type checkFn func(*testing.T, error)

		errIsNil := func(t *testing.T, err error) {
			if err != nil {
				t.Errorf("expected nil err, got %v", err)
			}
		}

		tests := map[string]func(t *testing.T) (validate.Kube, *roles.Instance, checkFn){
			"success": func(_ *testing.T) (validate.Kube, *roles.Instance, checkFn) {
				// configure fake k8s with storage secret
				data := []byte(fmt.Sprintf(`
storage:
  powermax:
    "000197900714":
      Endpoint: %s
      Insecure: true
      Password: Password123
      User: admin`, goodBackendPowerMax.URL))

				secret := &v1.Secret{
					ObjectMeta: meta.ObjectMeta{
						Name:      k8s.StorageSecret,
						Namespace: "test",
					},
					Data: map[string][]byte{
						k8s.StorageSecretDataKey: data,
					},
				}

				fakeClient := fake.NewSimpleClientset(secret)

				role := &roles.Instance{
					Quota: 1000,
					RoleKey: roles.RoleKey{
						Name:       "NewRole3",
						SystemType: "powermax",
						SystemID:   "000197900714",
						Pool:       "bronze",
					},
				}

				api := &k8s.API{
					Client:    fakeClient,
					Namespace: "test",
					Lock:      sync.Mutex{},
					Log:       logrus.NewEntry(logrus.StandardLogger()),
				}

				return api, role, errIsNil
			},
		}

		// run the tests
		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				kube, role, checkFn := tc(t)
				rv := validate.NewRoleValidator(kube, logrus.NewEntry(logrus.StandardLogger()))
				err := rv.Validate(context.Background(), role)
				checkFn(t, err)
			})
		}
	})

	// Error paths
	t.Run("Error", func(t *testing.T) {
		// Creates a fake powermax handler
		goodBackendPowerMax := httptest.NewTLSServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/univmax/restapi/version":
					fmt.Fprintf(w, `{ "version": "V10.0.0.1"}`)
				case "/univmax/restapi/100/sloprovisioning/symmetrix/000197900714/srp/bronze":
					w.WriteHeader(http.StatusOK)
				default:
					t.Errorf("unhandled unisphere request path: %s", r.URL.Path)
				}
			}))
		defer goodBackendPowerMax.Close()

		oldGetPowerMaxEndpoint := validate.GetPowerMaxEndpoint
		validate.GetPowerMaxEndpoint = func(_ storage.System) string {
			return goodBackendPowerMax.URL
		}
		defer func() { validate.GetPowerMaxEndpoint = oldGetPowerMaxEndpoint }()

		// define check functions to pass or fail tests
		type checkFn func(*testing.T, error)

		errIsNotNil := func(t *testing.T, err error) {
			if err == nil {
				t.Errorf("expected an err, got nil")
			}
		}

		tests := map[string]func(t *testing.T) (validate.Kube, *roles.Instance, checkFn){
			"negative quota": func(t *testing.T) (validate.Kube, *roles.Instance, checkFn) {
				// configure fake k8s with storage secret
				data := []byte(fmt.Sprintf(`
storage:
  powermax:
    "000197900714":
      Endpoint: %s
      Insecure: true
      Password: Password123
      User: admin`, goodBackendPowerMax.URL))

				secret := &v1.Secret{
					ObjectMeta: meta.ObjectMeta{
						Name:      k8s.StorageSecret,
						Namespace: "test",
					},
					Data: map[string][]byte{
						k8s.StorageSecretDataKey: data,
					},
				}

				fakeClient := fake.NewSimpleClientset()
				_, err := fakeClient.CoreV1().Secrets("test").Create(context.Background(), secret, meta.CreateOptions{})
				if err != nil {
					t.Fatal(err)
				}

				role := &roles.Instance{
					RoleKey: roles.RoleKey{
						Name:       "NewRole3",
						SystemType: "powermax",
						SystemID:   "000197900714",
						Pool:       "bronze",
					},
					Quota: -1,
				}

				api := &k8s.API{
					Client:    fakeClient,
					Namespace: "test",
					Lock:      sync.Mutex{},
					Log:       logrus.NewEntry(logrus.StandardLogger()),
				}

				return api, role, errIsNotNil
			},
		}

		// run the tests
		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				kube, role, checkFn := tc(t)
				rv := validate.NewRoleValidator(kube, logrus.NewEntry(logrus.StandardLogger()))
				err := rv.Validate(context.Background(), role)
				checkFn(t, err)
			})
		}
	})
}

func TestValidatePowerScale(t *testing.T) {
	// Happy paths
	t.Run("Success", func(t *testing.T) {
		// Creates a fake powerscale handler
		goodBackendPowerScale := httptest.NewTLSServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Log(r.URL.Path)
				switch r.URL.Path {
				case "/platform/latest/":
					fmt.Fprintf(w, `{ "latest": "6"}`)
				case "/namespace/bronze/":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintf(w, `{"attrs":[{"name":"is_hidden","value":false},{"name":"bronze","value":76}]}`)
				case "/session/1/session/":
					w.WriteHeader(http.StatusCreated)
				default:
					t.Errorf("unhandled powerscale request path: %s", r.URL.Path)
				}
			}))
		defer goodBackendPowerScale.Close()

		oldGetPowerScaleEndpoint := validate.GetPowerScaleEndpoint
		validate.GetPowerScaleEndpoint = func(_ storage.System) string {
			return goodBackendPowerScale.URL
		}
		defer func() { validate.GetPowerScaleEndpoint = oldGetPowerScaleEndpoint }()

		// define check functions to pass or fail tests
		type checkFn func(*testing.T, error)

		errIsNil := func(t *testing.T, err error) {
			if err != nil {
				t.Errorf("expected nil err, got %v", err)
			}
		}

		tests := map[string]func(t *testing.T) (validate.Kube, *roles.Instance, checkFn){
			"success": func(_ *testing.T) (validate.Kube, *roles.Instance, checkFn) {
				// configure fake k8s with storage secret
				data := []byte(fmt.Sprintf(`
storage:
  powerscale:
    myPowerScale:
      endpoint: %s
      insecure: true
      password: Password123
      user: admin`, goodBackendPowerScale.URL))

				secret := &v1.Secret{
					ObjectMeta: meta.ObjectMeta{
						Name:      k8s.StorageSecret,
						Namespace: "test",
					},
					Data: map[string][]byte{
						k8s.StorageSecretDataKey: data,
					},
				}

				fakeClient := fake.NewSimpleClientset(secret)

				role := &roles.Instance{
					RoleKey: roles.RoleKey{
						Name:       "NewRole3",
						SystemType: "powerscale",
						SystemID:   "myPowerScale",
						Pool:       "bronze",
					},
					Quota: 0,
				}

				api := &k8s.API{
					Client:    fakeClient,
					Namespace: "test",
					Lock:      sync.Mutex{},
					Log:       logrus.NewEntry(logrus.StandardLogger()),
				}

				return api, role, errIsNil
			},
		}

		// run the tests
		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				kube, role, checkFn := tc(t)
				rv := validate.NewRoleValidator(kube, logrus.NewEntry(logrus.StandardLogger()))
				err := rv.Validate(context.Background(), role)
				checkFn(t, err)
			})
		}
	})

	// Error paths
	t.Run("Error", func(t *testing.T) {
		// Creates a fake powerscale handler
		ts := httptest.NewTLSServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Log(r.URL.Path)
				switch r.URL.Path {
				case "/platform/latest/":
					fmt.Fprintf(w, `{ "latest": "6"}`)
				case "/namespace/bronze/":
					w.WriteHeader(http.StatusOK)
					fmt.Fprintf(w, `{"attrs":[{"name":"is_hidden","value":false},{"name":"bronze","value":76}]}`)
				case "/session/1/session/":
					w.WriteHeader(http.StatusCreated)
				default:
					t.Errorf("unhandled powerscale request path: %s", r.URL.Path)
				}
			}))
		defer ts.Close()

		oldGetPowerScaleEndpoint := validate.GetPowerScaleEndpoint
		validate.GetPowerScaleEndpoint = func(_ storage.System) string {
			return ts.URL
		}
		defer func() { validate.GetPowerScaleEndpoint = oldGetPowerScaleEndpoint }()

		// define check functions to pass or fail tests
		type checkFn func(*testing.T, error)

		errIsNotNil := func(t *testing.T, err error) {
			if err == nil {
				t.Errorf("expected an err, got nil")
			}
		}

		tests := map[string]func(t *testing.T) (validate.Kube, *roles.Instance, checkFn){
			"non-zero quota": func(t *testing.T) (validate.Kube, *roles.Instance, checkFn) {
				// configure fake k8s with storage secret
				data := []byte(fmt.Sprintf(`
storage:
  powerscale:
    myPowerScale:
      Endpoint: %s
      Insecure: true
      Password: Password123
      User: admin`, ts.URL))

				secret := &v1.Secret{
					ObjectMeta: meta.ObjectMeta{
						Name:      k8s.StorageSecret,
						Namespace: "test",
					},
					Data: map[string][]byte{
						k8s.StorageSecretDataKey: data,
					},
				}

				fakeClient := fake.NewSimpleClientset()
				_, err := fakeClient.CoreV1().Secrets("test").Create(context.Background(), secret, meta.CreateOptions{})
				if err != nil {
					t.Fatal(err)
				}

				role := &roles.Instance{
					RoleKey: roles.RoleKey{
						Name:       "NewRole3",
						SystemType: "powerscale",
						SystemID:   "myPowerScale",
						Pool:       "bronze",
					},
					Quota: 1000,
				}

				api := &k8s.API{
					Client:    fakeClient,
					Namespace: "test",
					Lock:      sync.Mutex{},
					Log:       logrus.NewEntry(logrus.StandardLogger()),
				}

				return api, role, errIsNotNil
			},
		}

		// run the tests
		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				kube, role, checkFn := tc(t)
				rv := validate.NewRoleValidator(kube, logrus.NewEntry(logrus.StandardLogger()))
				err := rv.Validate(context.Background(), role)
				checkFn(t, err)
			})
		}
	})
}

func TestValidateError(t *testing.T) {
	t.Run("invalid system", func(t *testing.T) {
		// define a role instance with an invalid system type
		roleInstance := &roles.Instance{
			RoleKey: roles.RoleKey{
				SystemType: "invalid",
			},
		}

		// define the validator
		rv := validate.NewRoleValidator(nil, logrus.NewEntry(logrus.StandardLogger()))

		// verifiy an error is returned
		err := rv.Validate(context.Background(), roleInstance)
		if err == nil {
			t.Errorf("expected an error, got nil")
		}
	})

	t.Run("getting storage system from storage secret", func(t *testing.T) {
		// define a role instance
		roleInstance := &roles.Instance{
			RoleKey: roles.RoleKey{
				SystemType: "powerflex",
			},
		}

		// define the validator with a k8s client that has no karavi-storage-secret configured
		fakeClient := fake.NewSimpleClientset()

		logger := logrus.NewEntry(logrus.StandardLogger())

		api := &k8s.API{
			Client:    fakeClient,
			Namespace: "test",
			Lock:      sync.Mutex{},
			Log:       logrus.NewEntry(logrus.StandardLogger()),
		}

		rv := validate.NewRoleValidator(api, logger)

		// verifiy an error is returned
		err := rv.Validate(context.Background(), roleInstance)
		if err == nil {
			t.Errorf("expected an error, got nil")
		}
	})

	t.Run("storage not configured", func(t *testing.T) {
		// define a role instance
		roleInstance := &roles.Instance{
			RoleKey: roles.RoleKey{
				SystemType: "powerflex",
				SystemID:   "542a2d5f5122210f",
			},
		}

		// configure fake k8s with storage secret
		data := []byte(fmt.Sprintf(`
storage:
  powerflex:
    notConfigured:
      Endpoint: %s
      Insecure: true
      Password: Password123
      User: admin`, ""))

		secret := &v1.Secret{
			ObjectMeta: meta.ObjectMeta{
				Name:      k8s.StorageSecret,
				Namespace: "test",
			},
			Data: map[string][]byte{
				k8s.StorageSecretDataKey: data,
			},
		}

		fakeClient := fake.NewSimpleClientset(secret)

		// define the validator

		logger := logrus.NewEntry(logrus.StandardLogger())

		api := &k8s.API{
			Client:    fakeClient,
			Namespace: "test",
			Lock:      sync.Mutex{},
			Log:       logger,
		}

		rv := validate.NewRoleValidator(api, logger)

		// verifiy an error is returned
		err := rv.Validate(context.Background(), roleInstance)
		if err == nil {
			t.Errorf("expected an error, got nil")
		}
	})

	t.Run("storage secret misconfigured", func(t *testing.T) {
		// define a role instance
		roleInstance := &roles.Instance{
			RoleKey: roles.RoleKey{
				SystemType: "powerflex",
			},
		}

		// configure fake k8s with storage secret with the wrong data key
		data := []byte(fmt.Sprintf(`
storage:
  powerflex:
    notConfigured:
      Endpoint: %s
      Insecure: true
      Password: Password123
      User: admin`, ""))

		secret := &v1.Secret{
			ObjectMeta: meta.ObjectMeta{
				Name:      k8s.StorageSecret,
				Namespace: "test",
			},
			Data: map[string][]byte{
				"wrong data key": data,
			},
		}

		fakeClient := fake.NewSimpleClientset(secret)

		// define the validator

		logger := logrus.NewEntry(logrus.StandardLogger())

		api := &k8s.API{
			Client:    fakeClient,
			Namespace: "test",
			Lock:      sync.Mutex{},
			Log:       logger,
		}

		rv := validate.NewRoleValidator(api, logger)

		// verifiy an error is returned
		err := rv.Validate(context.Background(), roleInstance)
		if err == nil {
			t.Errorf("expected an error, got nil")
		}
	})
}

func write(t *testing.T, w io.Writer, file string) {
	b, err := os.ReadFile(fmt.Sprintf("testdata/%s", file))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(w, bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
}
