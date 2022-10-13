/*
 Copyright © 2022 Dell Inc. or its subsidiaries. All Rights Reserved.

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at
      http://www.apache.org/licenses/LICENSE-2.0
 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/
package k8s

import (
	"bytes"
	"context"
	"errors"
	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/internal/types"
	"reflect"
	"testing"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

type connectFn func(*API) error
type configFn func() (*rest.Config, error)

func TestGetConfiguredRoles(t *testing.T) {
	// define check functions to pass or fail tests
	type checkFn func(*testing.T, *roles.JSON, error)

	checkExpectedOutput := func(key roles.RoleKey, expectedRolesJSON *roles.JSON) func(*testing.T, *roles.JSON, error) {
		return func(t *testing.T, rolesJSON *roles.JSON, err error) {
			if err != nil {
				t.Fatal(err)
			}

			want := expectedRolesJSON.Get(key)
			got := rolesJSON.Get(key)

			if !reflect.DeepEqual(want, got) {
				t.Errorf("want %+v, got %+v", want, got)
			}
		}
	}

	hasErr := func() func(*testing.T, *roles.JSON, error) {
		return func(t *testing.T, j *roles.JSON, err error) {
			if err == nil {
				t.Errorf("expected nil err, got %+v", err)
			}
		}
	}

	// define test input

	tests := map[string]func(t *testing.T) (connectFn, configFn, checkFn){
		"success": func(*testing.T) (connectFn, configFn, checkFn) {
			data := `
package karavi.common
default roles = {}
roles = {
	"test": {
		"system_types": {
			"powerflex": {
				"system_ids": {
					"11e4e7d35817bd0f": {
						"pool_quotas": {
							"bronze": 100
						}
					}
				}
			}
		}
	}
}`

			configMap := &v1.ConfigMap{
				ObjectMeta: meta.ObjectMeta{
					Name:      "common",
					Namespace: "test",
				},
				Data: map[string]string{
					RolesConfigMapDataKey: data,
				},
			}

			key := roles.RoleKey{
				Name:       "test",
				SystemType: "powerflex",
				SystemID:   "11e4e7d35817bd0f",
				Pool:       "bronze",
			}

			expectedRoles := roles.NewJSON()
			expectedRoles.Add(&roles.Instance{
				Quota:   100,
				RoleKey: key,
			})

			connect := func(api *API) error {
				api.Client = fake.NewSimpleClientset(configMap)
				return nil
			}
			return connect, nil, checkExpectedOutput(key, &expectedRoles)
		},
		"error connecting": func(*testing.T) (connectFn, configFn, checkFn) {
			connect := func(api *API) error {
				return errors.New("error")
			}
			return connect, nil, hasErr()
		},
		"error getting a valid config": func(*testing.T) (connectFn, configFn, checkFn) {
			inClusterConfig := func() (*rest.Config, error) {
				return nil, errors.New("error")
			}
			return nil, inClusterConfig, hasErr()
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			connectFn, inClusterConfig, checkFn := tc(t)
			api := API{
				Namespace: "test",
				Log:       logrus.NewEntry(logrus.StandardLogger()),
			}

			if connectFn != nil {
				oldConnectFn := ConnectFn
				defer func() { ConnectFn = oldConnectFn }()
				ConnectFn = connectFn
			}

			if inClusterConfig != nil {
				oldInClusterConfig := InClusterConfigFn
				defer func() { InClusterConfigFn = oldInClusterConfig }()
				InClusterConfigFn = inClusterConfig
			}

			roles, err := api.GetConfiguredRoles(context.Background())
			checkFn(t, roles, err)
		})
	}

}

func TestUpdateRoles(t *testing.T) {
	testGetApplyConfig(t)

	/*
		// define check functions to pass or fail tests
		type checkFn func(*testing.T, error)

		errIsNil := func(t *testing.T, err error) {
			if err != nil {
				t.Errorf("expected nil err, got %v", err)
			}
		}

		errIsNotNil := func(t *testing.T, err error) {
			if err == nil {
				t.Errorf("expected non-nil err")
			}
		}

		// define the tests
		tests := map[string]func(t *testing.T) (connectFn, configFn, *roles.JSON, checkFn){
			"success": func(*testing.T) (connectFn, configFn, *roles.JSON, checkFn) {
				connect := func(api *API) error {
					cm := &v1.ConfigMap{
						ObjectMeta: meta.ObjectMeta{
							Name:      ROLES_CONFIGMAP,
							Namespace: "test",
						},
						Data: map[string]string{
							ROLES_CONFIGMAP_DATA_KEY: "",
						},
					}
					c := fake.NewSimpleClientset()

					_, err := c.CoreV1().ConfigMaps("test").Create(context.Background(), cm, meta.CreateOptions{})
					if err != nil {
						t.Fatal(err)
					}

					c.PrependReactor("apply", "configmap", func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						obj := &v1.ConfigMap{
							ObjectMeta: meta.ObjectMeta{
								Name:      ROLES_CONFIGMAP,
								Namespace: "test",
							},
							Data: map[string]string{},
						}
						return false, obj, nil
					})
					api.Client = c
					return nil
				}

				r := roles.NewJSON()
				err := r.Add(&roles.Instance{
					Quota: 1000,
					RoleKey: roles.RoleKey{
						Name: "test",
					},
				})
				if err != nil {
					t.Fatal(err)
				}
				return connect, nil, &r, errIsNil
			},
			"error connecting": func(*testing.T) (connectFn, configFn, *roles.JSON, checkFn) {
				connect := func(api *API) error {
					return errors.New("error")
				}
				return connect, nil, nil, errIsNotNil
			},
			"error getting a valid config": func(*testing.T) (connectFn, configFn, *roles.JSON, checkFn) {
				inClusterConfig := func() (*rest.Config, error) {
					return nil, errors.New("error")
				}
				return nil, inClusterConfig, nil, errIsNotNil
			},
		}

		for name, tc := range tests {
			t.Run(name, func(t *testing.T) {
				connectFn, inClusterConfig, roles, checkFn := tc(t)

				if connectFn != nil {
					oldConnectFn := ConnectFn
					defer func() { ConnectFn = oldConnectFn }()
					ConnectFn = connectFn
				}

				if inClusterConfig != nil {
					oldInClusterConfig := InClusterConfigFn
					defer func() { InClusterConfigFn = oldInClusterConfig }()
					InClusterConfigFn = inClusterConfig
				}

				api := API{
					Namespace: "test",
				}

				err := api.UpdateRoles(context.Background(), roles)
				checkFn(t, err)
			})
		}*/
}

func TestGetConfiguredStorage(t *testing.T) {
	// define check functions to pass or fail tests
	type checkFn func(*testing.T, types.Storage, error)

	checkExpectedOutput := func(want types.Storage) func(*testing.T, types.Storage, error) {
		return func(t *testing.T, got types.Storage, err error) {
			if !reflect.DeepEqual(want, got) {
				t.Errorf("want %+v, got %+v", want, got)
			}
		}
	}

	hasErr := func() func(*testing.T, types.Storage, error) {
		return func(t *testing.T, got types.Storage, err error) {
			if err == nil {
				t.Errorf("expected nil err, got %+v", err)
			}
		}
	}

	// define the tests

	tests := map[string]func(t *testing.T) (connectFn, configFn, checkFn){
		"success": func(*testing.T) (connectFn, configFn, checkFn) {
			// configure fake k8s with storage secret
			data := []byte(`
storage:
  powerflex:
    542a2d5f5122210f:
      endpoint: https://10.0.0.1
      insecure: true
      password: password
      user: user`)

			secret := &v1.Secret{
				ObjectMeta: meta.ObjectMeta{
					Name:      StorageSecret,
					Namespace: "test",
				},
				Data: map[string][]byte{
					StorageSecretDataKey: data,
				},
			}

			expectedStorage := types.Storage{
				"powerflex": types.SystemType{
					"542a2d5f5122210f": types.System{
						User:     "user",
						Password: "password",
						Endpoint: "https://10.0.0.1",
						Insecure: true,
					},
				},
			}

			connect := func(api *API) error {
				api.Client = fake.NewSimpleClientset(secret)
				return nil
			}
			return connect, nil, checkExpectedOutput(expectedStorage)
		},
		"error connecting": func(*testing.T) (connectFn, configFn, checkFn) {
			connect := func(api *API) error {
				return errors.New("error")
			}
			return connect, nil, hasErr()
		},
		"error getting a valid config": func(*testing.T) (connectFn, configFn, checkFn) {
			inClusterConfig := func() (*rest.Config, error) {
				return nil, errors.New("error")
			}
			return nil, inClusterConfig, hasErr()
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			connectFn, inClusterConfig, checkFn := tc(t)
			api := API{
				Namespace: "test",
				Log:       logrus.NewEntry(logrus.StandardLogger()),
			}

			if connectFn != nil {
				oldConnectFn := ConnectFn
				defer func() { ConnectFn = oldConnectFn }()
				ConnectFn = connectFn
			}

			if inClusterConfig != nil {
				oldInClusterConfig := InClusterConfigFn
				defer func() { InClusterConfigFn = oldInClusterConfig }()
				InClusterConfigFn = inClusterConfig
			}

			storage, err := api.GetConfiguredStorage(context.Background())
			checkFn(t, storage, err)
		})
	}
}

func testGetApplyConfig(t *testing.T) {
	type checkFn func(*testing.T, string, error)

	checkExpectedOutput := func(want string) func(*testing.T, string, error) {
		return func(t *testing.T, got string, err error) {
			// remove spacing issues by removing white space and new line characters
			//want := strings.ReplaceAll(strings.ReplaceAll(expected, "\n", ""), " ", "")
			//got := strings.ReplaceAll(strings.ReplaceAll(result, "\n", ""), " ", "")

			if want != got {
				t.Errorf("want %s, got %s", want, got)
			}
		}
	}

	tests := map[string]func(t *testing.T) (*roles.JSON, checkFn){
		"success": func(*testing.T) (*roles.JSON, checkFn) {
			r := roles.NewJSON()

			r.Add(&roles.Instance{
				Quota: 100,
				RoleKey: roles.RoleKey{
					Name:       "test",
					SystemType: "powerflex",
					SystemID:   "542a2d5f5122210f",
					Pool:       "bronze",
				},
			})

			want := `package karavi.common
default roles = {}
roles = {
  "test": {
    "system_types": {
      "powerflex": {
        "system_ids": {
          "542a2d5f5122210f": {
            "pool_quotas": {
              "bronze": 100
            }
          }
        }
      }
    }
  }
}`
			return &r, checkExpectedOutput(want)
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			rolesJSON, checkFn := tc(t)
			api := API{
				Log: logrus.NewEntry(logrus.StandardLogger()),
			}

			conf, err := api.getApplyConfig(rolesJSON)
			checkFn(t, conf.Data[RolesConfigMapDataKey], err)
		})
	}
}

func TestUpdateStorages(t *testing.T) {
	testGetStorageSecret(t)
}

func testGetStorageSecret(t *testing.T) {
	type checkFn func(*testing.T, []byte, error)

	checkExpectedOutput := func(want []byte) func(*testing.T, []byte, error) {
		return func(t *testing.T, got []byte, err error) {
			if !bytes.Equal(want, got) {
				t.Errorf("want %s, got %s", string(want), string(got))
			}
		}
	}

	tests := map[string]func(t *testing.T) (types.Storage, checkFn){
		"success": func(*testing.T) (types.Storage, checkFn) {

			storage := types.Storage{
				"powerflex": types.SystemType{
					"542a2d5f5122210f": types.System{
						User:     "admin",
						Password: "Password123",
						Endpoint: "0.0.0.0:443",
						Insecure: true,
					},
				},
			}
			want := `storage:
  powerflex:
    542a2d5f5122210f:
      user: admin
      password: Password123
      endpoint: 0.0.0.0:443
      insecure: true
`

			b := []byte(want)
			return storage, checkExpectedOutput(b)
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			storage, checkFn := tc(t)
			api := API{
				Log: logrus.NewEntry(logrus.StandardLogger()),
			}

			secret, err := api.getStorageSecret(storage)
			checkFn(t, secret.Data[StorageSecretDataKey], err)
		})
	}
}
