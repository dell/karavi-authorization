// Copyright © 2021-2022 Dell Inc., or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0

package powerflex_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"karavi-authorization/internal/powerflex"
	"net/http"
	"testing"
	"time"

	"github.com/dell/goscaleio"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

var (
	token = "YWRtaW46MTYxMDUxNzk5NDQxODpjYzBkMGEwMmUwYzNiODUxOTM1NWMxZThkNTcwZWEwNA"
)

func TestStoragePoolCache_GetStoragePoolNameByID(t *testing.T) {
	t.Run("success getting a storage pool not in cache", func(t *testing.T) {
		// Arrange

		// Variable to keep track of the /api/types/StoragePool/instances calls initiated from the cache
		powerFlexCallCount := 0

		// Setup httptest server to represent a PowerFlex
		powerFlexSvr := newPowerFlexTestServer(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.String() {
			case "/api/version":
				w.Write([]byte("3.5"))
			case "/api/types/StoragePool/instances":
				switch powerFlexCallCount {
				case 0:
					powerFlexCallCount++
					data, err := ioutil.ReadFile("testdata/storage_pool_instances.json")
					if err != nil {
						t.Fatal(err)
					}
					w.Write(data)
				default:
					t.Fatal("unexpected call to PowerFlex server")
				}
			default:
				t.Fatalf("path %s not supported", r.URL.String())
			}
		})
		defer powerFlexSvr.Close()

		pfClient := newPowerFlexClient(t, powerFlexSvr.URL)
		client := powerflex.NewClient(pfClient)
		tk := newTokenGetter(t, client, powerFlexSvr.URL)

		// Create a new storage pool cache pointing to the httptest server PowerFlex
		cache, err := powerflex.NewStoragePoolCache(client, 2)
		if err != nil {
			t.Fatal(err)
		}

		// Act

		// Get storage pool name with ID 3df6b86600000000
		poolName, err := cache.GetStoragePoolNameByID(context.Background(), tk, "3df6b86600000000")

		// Assert
		expectedPoolName := "mypool"

		// Assert that err is nil
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}

		// Assert that the pool name we got is the one we expect
		if poolName != expectedPoolName {
			t.Errorf("expected pool name %s, got %s", expectedPoolName, poolName)
		}
	})

	t.Run("success getting a storage pool in cache", func(t *testing.T) {
		// Arrange

		// Variable to keep track of the /api/types/StoragePool/instances calls initiated from the cache
		powerFlexCallCount := 0

		// Setup httptest server to represent a PowerFlex
		powerFlexSvr := newPowerFlexTestServer(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.String() {
			case "/api/version":
				w.Write([]byte("3.5"))
			case "/api/types/StoragePool/instances":
				switch powerFlexCallCount {
				case 0:
					powerFlexCallCount++
					data, err := ioutil.ReadFile("testdata/storage_pool_instances.json")
					if err != nil {
						t.Fatal(err)
					}
					w.Write(data)
				default:
					t.Fatal("unexpected call to PowerFlex server")
				}
			default:
				t.Fatalf("path %s not supported", r.URL.String())
			}

		})
		defer powerFlexSvr.Close()

		pfClient := newPowerFlexClient(t, powerFlexSvr.URL)
		client := powerflex.NewClient(pfClient)
		tk := newTokenGetter(t, client, powerFlexSvr.URL)

		// Create a new storage pool cache pointing to the httptest server PowerFlex
		cache, err := powerflex.NewStoragePoolCache(client, 2)
		if err != nil {
			t.Fatal(err)
		}

		// Update the cache with storage pool ID 3df6b86600000000
		_, err = cache.GetStoragePoolNameByID(context.Background(), tk, "3df6b86600000000")
		if err != nil {
			t.Fatal(err)
		}

		// Act

		// Get storage pool name with ID 3df6b86600000000
		poolName, err := cache.GetStoragePoolNameByID(context.Background(), tk, "3df6b86600000000")

		// Assert

		expectedPoolName := "mypool"

		// Assert that err is nil
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}

		// Assert that the pool name we got is the one we expect
		if poolName != expectedPoolName {
			t.Errorf("expected pool name %s, got %s", expectedPoolName, poolName)
		}
	})

	t.Run("error finding storage pool from PowerFlex", func(t *testing.T) {
		// Arrange

		// Variable to keep track of the /api/types/StoragePool/instances calls initiated from the cache
		powerFlexCallCount := 0

		// Setup httptest server to represent a PowerFlex
		powerFlexSvr := newPowerFlexTestServer(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.String() {
			case "/api/version":
				w.Write([]byte("3.5"))
			case "/api/types/StoragePool/instances":
				switch powerFlexCallCount {
				case 0:
					powerFlexCallCount++
					w.WriteHeader(http.StatusInternalServerError)
				default:
					t.Fatal("unexpected call to PowerFlex server")
				}
			default:
				t.Fatalf("path %s not supported", r.URL.String())
			}

		})
		defer powerFlexSvr.Close()

		pfClient := newPowerFlexClient(t, powerFlexSvr.URL)
		client := powerflex.NewClient(pfClient)
		tk := newTokenGetter(t, client, powerFlexSvr.URL)

		// Create a new storage pool cache pointing to the httptest server PowerFlex
		cache, err := powerflex.NewStoragePoolCache(client, 2)
		if err != nil {
			t.Fatal(err)
		}

		// Act

		// Get storage pool name with ID 3df6b86600000000
		poolName, err := cache.GetStoragePoolNameByID(context.Background(), tk, "0")

		// Assert

		// Assert that the pool is nil value
		if poolName != "" {
			t.Errorf("expected nil pool name value, got %s", poolName)
		}

		// Asser that err is not nil
		if err == nil {
			t.Errorf("expected an error, got nil")
		}
	})

	t.Run("Nil client", func(t *testing.T) {
		// Arrange

		// Variable to keep track of the /api/types/StoragePool/instances calls initiated from the cache
		powerFlexCallCount := 0

		// Setup httptest server to represent a PowerFlex
		powerFlexSvr := newPowerFlexTestServer(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.String() {
			case "/api/version":
				w.Write([]byte("3.5"))
			case "/api/types/StoragePool/instances":
				switch powerFlexCallCount {
				case 0:
					powerFlexCallCount++
					w.WriteHeader(http.StatusInternalServerError)
				default:
					t.Fatal("unexpected call to PowerFlex server")
				}
			default:
				t.Fatalf("path %s not supported", r.URL.String())
			}

		})
		defer powerFlexSvr.Close()

		// Attempt to create new storage pool with invalid client
		_, gotErr := powerflex.NewStoragePoolCache(nil, 2)
		wantErr := fmt.Errorf("goscaleio client is required")
		if gotErr.Error() != wantErr.Error() {
			t.Errorf("New Storage Pool Cache: got err = %v, want: %v", gotErr, wantErr)
		}
	})

	t.Run("Cache size < 1", func(t *testing.T) {
		// Arrange

		// Variable to keep track of the /api/types/StoragePool/instances calls initiated from the cache
		powerFlexCallCount := 0

		// Setup httptest server to represent a PowerFlex
		powerFlexSvr := newPowerFlexTestServer(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.String() {
			case "/api/version":
				w.Write([]byte("3.5"))
			case "/api/types/StoragePool/instances":
				switch powerFlexCallCount {
				case 0:
					powerFlexCallCount++
					w.WriteHeader(http.StatusInternalServerError)
				default:
					t.Fatal("unexpected call to PowerFlex server")
				}
			default:
				t.Fatalf("path %s not supported", r.URL.String())
			}

		})
		defer powerFlexSvr.Close()

		pfClient := newPowerFlexClient(t, powerFlexSvr.URL)
		client := powerflex.NewClient(pfClient)

		// Attempt to create new storage pool with cache size
		_, gotErr := powerflex.NewStoragePoolCache(client, 0)
		wantErr := fmt.Errorf("cache size must be at least one")
		if gotErr.Error() != wantErr.Error() {
			t.Errorf("New Storage Pool Cache: got err = %v, want: %v", gotErr, wantErr)
		}
	})

	t.Run("success multiple go routines accessing same storage pool at same time", func(t *testing.T) {
		// Arrange

		// Variable to keep track of the /api/types/StoragePool/instances calls initiated from the cache
		powerFlexCallCount := 0

		// Setup httptest server to represent a PowerFlex
		powerFlexSvr := newPowerFlexTestServer(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.String() {
			case "/api/version":
				w.Write([]byte("3.5"))
			case "/api/types/StoragePool/instances":
				switch powerFlexCallCount {
				case 0:
					powerFlexCallCount++
					data, err := ioutil.ReadFile("testdata/storage_pool_instances.json")
					if err != nil {
						t.Fatal(err)
					}
					w.Write(data)
				default:
					t.Fatal("unexpected call to PowerFlex server")
				}
			default:
				t.Fatalf("path %s not supported", r.URL.String())
			}

		})
		defer powerFlexSvr.Close()

		pfClient := newPowerFlexClient(t, powerFlexSvr.URL)
		client := powerflex.NewClient(pfClient)
		tk := newTokenGetter(t, client, powerFlexSvr.URL)

		// Create a new storage pool cache pointing to the httptest server PowerFlex
		cache, err := powerflex.NewStoragePoolCache(client, 2)
		if err != nil {
			t.Fatal(err)
		}

		// Act

		// Variables to hold pool name results
		var poolNameOne string
		var poolNameTwo string

		var eg errgroup.Group

		// Get storage pool name with ID 3df6b86600000000 in one go routine
		eg.Go(func() error {
			var err error
			poolNameOne, err = cache.GetStoragePoolNameByID(context.Background(), tk, "3df6b86600000000")
			return err
		})

		// Get storage pool name with ID 3df6b86600000000 in another go routine
		eg.Go(func() error {
			var err error
			poolNameTwo, err = cache.GetStoragePoolNameByID(context.Background(), tk, "3df6b86600000000")
			return err
		})

		err = eg.Wait()

		// Assert

		expectedPoolName := "mypool"

		// Asser that err is nil
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}

		// Assert that the pool name we got is the one we expect
		if poolNameOne != expectedPoolName {
			t.Errorf("expected pool name one %s, got %s", expectedPoolName, poolNameOne)
		}

		// Assert that the pool name we got is the one we expect
		if poolNameTwo != expectedPoolName {
			t.Errorf("expected pool name two %s, got %s", expectedPoolName, poolNameTwo)
		}

		// Assert that the number of PowerFlex server calls is no greater than 1
		if powerFlexCallCount > 1 {
			t.Errorf("expected only one PowerFlex server call, got %d", powerFlexCallCount)
		}
	})
}

func newPowerFlexClient(t *testing.T, addr string) *goscaleio.Client {
	client, err := goscaleio.NewClientWithArgs(addr, "", false, false)
	if err != nil {
		t.Fatal(err)
	}

	return client
}

func newTokenGetter(t *testing.T, client powerflex.TokenClient, addr string) *powerflex.TokenGetter {
	return powerflex.NewTokenGetter(powerflex.Config{
		PowerFlexClient:      client,
		TokenRefreshInterval: 5 * time.Minute,
		ConfigConnect: &goscaleio.ConfigConnect{
			Endpoint: addr,
			Username: "",
			Password: "",
		},
		Logger: logrus.NewEntry(logrus.New()),
	})
}
