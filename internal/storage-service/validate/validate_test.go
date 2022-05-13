// Copyright © 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"context"
	"fmt"
	"karavi-authorization/internal/k8s"
	"karavi-authorization/internal/storage-service/validate"
	"karavi-authorization/internal/types"
	"net/http"
	"net/http/httptest"
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
		// create mock backend powerflex
		goodBackendPowerFlex := httptest.NewTLSServer(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/login":
					fmt.Fprintf(w, `"token"`)
				case "/api/version":
					fmt.Fprintf(w, "3.5")
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
		validate.GetPowerFlexEndpoint = func(system types.System) string {
			return goodBackendPowerFlex.URL
		}
		defer func() { validate.GetPowerFlexEndpoint = oldGetPowerFlexEndpoint }()

		// define the tests
		tests := map[string]func(t *testing.T) (validate.Kube, string, types.System, checkFn){
			"success": func(t *testing.T) (validate.Kube, string, types.System, checkFn) {
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

				newSystem := types.System{
					User:     "admin",
					Password: "Password123",
					Endpoint: goodBackendPowerFlex.URL,
					Insecure: true,
				}

				api := &k8s.API{
					Client:    fakeClient,
					Namespace: "test",
					Lock:      sync.Mutex{},
					Log:       logrus.NewEntry(logrus.StandardLogger()),
				}

				return api, "542a2d5f5122210f", newSystem, errIsNil
			},
		}

		// run the tests
		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				kube, systemID, system, checkFn := tc(t)
				rv := validate.NewStorageValidator(kube, logrus.NewEntry(logrus.StandardLogger()))
				err := rv.Validate(context.Background(), systemID, "powerflex", system)
				checkFn(t, err)
			})
		}
	})

	// Error cases
	t.Run("Error", func(t *testing.T) {

		// define check functions to pass or fail tests
		type checkFn func(*testing.T, error)

		errIsNotNil := func(t *testing.T, err error) {
			if err == nil {
				t.Errorf("expected an err, got nil")
			}
		}

		// define the tests
		tests := map[string]func(t *testing.T) (validate.Kube, string, types.System, checkFn){
			"fail to connect": func(t *testing.T) (validate.Kube, string, types.System, checkFn) {
				// configure fake k8s with storage secret
				fakeClient := fake.NewSimpleClientset()

				newSystem := types.System{
					User:     "admin",
					Password: "Password123",
					Endpoint: "0.0.0.0:443",
					Insecure: false,
				}

				api := &k8s.API{
					Client:    fakeClient,
					Namespace: "test",
					Lock:      sync.Mutex{},
					Log:       logrus.NewEntry(logrus.StandardLogger()),
				}

				return api, "542a2d5f5122210f", newSystem, errIsNotNil
			},
			"invalid endpoint": func(t *testing.T) (validate.Kube, string, types.System, checkFn) {
				// configure fake k8s with storage secret
				fakeClient := fake.NewSimpleClientset()

				newSystem := types.System{
					User:     "admin",
					Password: "Password123",
					Endpoint: "invalid-endpoint",
					Insecure: true,
				}

				api := &k8s.API{
					Client:    fakeClient,
					Namespace: "test",
					Lock:      sync.Mutex{},
					Log:       logrus.NewEntry(logrus.StandardLogger()),
				}

				return api, "542a2d5f5122210f", newSystem, errIsNotNil
			},
		}

		// run the tests
		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				kube, systemID, system, checkFn := tc(t)
				rv := validate.NewStorageValidator(kube, logrus.NewEntry(logrus.StandardLogger()))
				err := rv.Validate(context.Background(), systemID, "powerflex", system)
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
				case "/univmax/restapi/90/system/version":
					fmt.Fprintf(w, `{ "version": "V9.2.1.2"}`)
				default:
					t.Errorf("unhandled unisphere request path: %s", r.URL.Path)
				}
			}))
		defer goodBackendPowerMax.Close()

		oldGetPowerMaxEndpoint := validate.GetPowerMaxEndpoint
		validate.GetPowerMaxEndpoint = func(storageSystemDetails types.System) string {
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

		tests := map[string]func(t *testing.T) (validate.Kube, string, types.System, checkFn){
			"success": func(t *testing.T) (validate.Kube, string, types.System, checkFn) {
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

				newSystem := types.System{
					User:     "admin",
					Password: "Password123",
					Endpoint: goodBackendPowerMax.URL,
					Insecure: true,
				}

				api := &k8s.API{
					Client:    fakeClient,
					Namespace: "test",
					Lock:      sync.Mutex{},
					Log:       logrus.NewEntry(logrus.StandardLogger()),
				}

				return api, "000197900714", newSystem, errIsNil
			},
		}

		// run the tests
		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				kube, systemID, system, checkFn := tc(t)
				rv := validate.NewStorageValidator(kube, logrus.NewEntry(logrus.StandardLogger()))
				err := rv.Validate(context.Background(), systemID, "powermax", system)
				checkFn(t, err)
			})
		}
	})

	// Error paths
	t.Run("Error", func(t *testing.T) {

		// define check functions to pass or fail tests
		type checkFn func(*testing.T, error)

		errIsNotNil := func(t *testing.T, err error) {
			if err == nil {
				t.Errorf("expected an err, got nil")
			}
		}

		tests := map[string]func(t *testing.T) (validate.Kube, string, types.System, checkFn){
			"fail to connect": func(t *testing.T) (validate.Kube, string, types.System, checkFn) {
				// configure fake k8s with storage secret
				fakeClient := fake.NewSimpleClientset()

				newSystem := types.System{
					User:     "admin",
					Password: "Password123",
					Endpoint: "0.0.0.0:443",
					Insecure: false,
				}

				api := &k8s.API{
					Client:    fakeClient,
					Namespace: "test",
					Lock:      sync.Mutex{},
					Log:       logrus.NewEntry(logrus.StandardLogger()),
				}

				return api, "000197900714", newSystem, errIsNotNil
			},
			"invalid endpoint": func(t *testing.T) (validate.Kube, string, types.System, checkFn) {
				// configure fake k8s with storage secret
				fakeClient := fake.NewSimpleClientset()

				newSystem := types.System{
					User:     "admin",
					Password: "Password123",
					Endpoint: "invalid-endpoint",
					Insecure: true,
				}

				api := &k8s.API{
					Client:    fakeClient,
					Namespace: "test",
					Lock:      sync.Mutex{},
					Log:       logrus.NewEntry(logrus.StandardLogger()),
				}

				return api, "000197900714", newSystem, errIsNotNil
			},
		}

		// run the tests
		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				kube, systemID, system, checkFn := tc(t)
				rv := validate.NewStorageValidator(kube, logrus.NewEntry(logrus.StandardLogger()))
				err := rv.Validate(context.Background(), systemID, "powermax", system)
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
				case "/platform/3/cluster/config/":
					fmt.Fprintf(w, `{ "name": "myPowerScale"}`)
				default:
					t.Errorf("unhandled powerscale request path: %s", r.URL.Path)
				}
			}))
		defer goodBackendPowerScale.Close()

		oldGetPowerScaleEndpoint := validate.GetPowerScaleEndpoint
		validate.GetPowerScaleEndpoint = func(storageSystemDetails types.System) string {
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

		tests := map[string]func(t *testing.T) (validate.Kube, string, types.System, checkFn){
			"success": func(t *testing.T) (validate.Kube, string, types.System, checkFn) {
				// configure fake k8s with storage secret
				var data []byte

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

				newSystem := types.System{
					User:     "admin",
					Password: "Password123",
					Endpoint: goodBackendPowerScale.URL,
					Insecure: true,
				}

				api := &k8s.API{
					Client:    fakeClient,
					Namespace: "test",
					Lock:      sync.Mutex{},
					Log:       logrus.NewEntry(logrus.StandardLogger()),
				}

				return api, "myPowerScale", newSystem, errIsNil
			},
		}

		// run the tests
		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				kube, systemID, system, checkFn := tc(t)
				rv := validate.NewStorageValidator(kube, logrus.NewEntry(logrus.StandardLogger()))
				err := rv.Validate(context.Background(), systemID, "powerscale", system)
				checkFn(t, err)
			})
		}
	})

	// Error paths
	t.Run("Error", func(t *testing.T) {

		// define check functions to pass or fail tests
		type checkFn func(*testing.T, error)

		errIsNotNil := func(t *testing.T, err error) {
			if err == nil {
				t.Errorf("expected an err, got nil")
			}
		}

		tests := map[string]func(t *testing.T) (validate.Kube, string, types.System, checkFn){
			"fail to connect": func(t *testing.T) (validate.Kube, string, types.System, checkFn) {
				// configure fake k8s with storage secret
				fakeClient := fake.NewSimpleClientset()

				newSystem := types.System{
					User:     "admin",
					Password: "Password123",
					Endpoint: "0.0.0.0:443",
					Insecure: false,
				}

				api := &k8s.API{
					Client:    fakeClient,
					Namespace: "test",
					Lock:      sync.Mutex{},
					Log:       logrus.NewEntry(logrus.StandardLogger()),
				}

				return api, "myPowerScale", newSystem, errIsNotNil
			},
			"invalid endpoint": func(t *testing.T) (validate.Kube, string, types.System, checkFn) {
				// configure fake k8s with storage secret
				fakeClient := fake.NewSimpleClientset()

				newSystem := types.System{
					User:     "admin",
					Password: "Password123",
					Endpoint: "invalid-endpoint",
					Insecure: true,
				}

				api := &k8s.API{
					Client:    fakeClient,
					Namespace: "test",
					Lock:      sync.Mutex{},
					Log:       logrus.NewEntry(logrus.StandardLogger()),
				}

				return api, "myPowerScale", newSystem, errIsNotNil
			},
		}

		// run the tests
		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				kube, systemID, system, checkFn := tc(t)
				rv := validate.NewStorageValidator(kube, logrus.NewEntry(logrus.StandardLogger()))
				err := rv.Validate(context.Background(), systemID, "powerscale", system)
				checkFn(t, err)
			})
		}
	})
}

func TestValidateError(t *testing.T) {

	t.Run("invalid system type", func(t *testing.T) {

		// define the validator with a k8s client that has no karavi-storage-secret configured
		fakeClient := fake.NewSimpleClientset()

		logger := logrus.NewEntry(logrus.StandardLogger())

		api := &k8s.API{
			Client:    fakeClient,
			Namespace: "test",
			Lock:      sync.Mutex{},
			Log:       logrus.NewEntry(logrus.StandardLogger()),
		}

		rv := validate.NewStorageValidator(api, logger)

		// verifiy an error is returned
		err := rv.Validate(context.Background(), "542a2d5f5122210f", "invalid-system-type", types.System{})
		if err == nil {
			t.Errorf("expected an error, got nil")
		}
	})
}
