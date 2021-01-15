package powerflex_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"powerflex-reverse-proxy/hack/powerflex"
	"testing"

	"github.com/dell/goscaleio"
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
			case "/api/types/StoragePool/instances":
				switch powerFlexCallCount {
				case 0:
					powerFlexCallCount++
					data, err := ioutil.ReadFile("testdata/storage_pool_instances.json")
					if err != nil {
						panic(err)
					}
					w.Write(data)
				default:
					panic("unexpected call to PowerFlex server")
				}
			default:
				panic(fmt.Sprintf("path %s not supported", r.URL.String()))
			}
		})
		defer powerFlexSvr.Close()

		// Create a new storage pool cache pointing to the httptest server PowerFlex
		config := powerflex.StoragePoolCacheConfig{
			PowerFlexClient: newPowerFlexClient(powerFlexSvr.URL),
			Size:            2,
		}
		cache, err := powerflex.NewStoragePoolCache(config)
		if err != nil {
			t.Fatal(err)
		}

		// Act

		// Get storage pool name with ID 3df6b86600000000
		poolName, err := cache.GetStoragePoolNameByID("3df6b86600000000")

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
			case "/api/types/StoragePool/instances":
				switch powerFlexCallCount {
				case 0:
					powerFlexCallCount++
					data, err := ioutil.ReadFile("testdata/storage_pool_instances.json")
					if err != nil {
						panic(err)
					}
					w.Write(data)
				default:
					panic("unexpected call to PowerFlex server")
				}
			default:
				panic(fmt.Sprintf("path %s not supported", r.URL.String()))
			}

		})
		defer powerFlexSvr.Close()

		// Create a new storage pool cache pointing to the httptest server PowerFlex
		config := powerflex.StoragePoolCacheConfig{
			PowerFlexClient: newPowerFlexClient(powerFlexSvr.URL),
			Size:            2,
		}

		cache, err := powerflex.NewStoragePoolCache(config)
		if err != nil {
			t.Fatal(err)
		}

		// Update the cache with storage pool ID 3df6b86600000000
		_, err = cache.GetStoragePoolNameByID("3df6b86600000000")
		if err != nil {
			t.Fatal(err)
		}

		// Act

		// Get storage pool name with ID 3df6b86600000000
		poolName, err := cache.GetStoragePoolNameByID("3df6b86600000000")

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
			case "/api/types/StoragePool/instances":
				switch powerFlexCallCount {
				case 0:
					data, err := ioutil.ReadFile("testdata/storage_pool_instances.json")
					if err != nil {
						panic(err)
					}
					w.Write(data)
				default:
					panic("unexpected call to PowerFlex server")
				}
			default:
				panic(fmt.Sprintf("path %s not supported", r.URL.String()))
			}

		})
		defer powerFlexSvr.Close()

		// Create a new storage pool cache pointing to the httptest server PowerFlex
		config := powerflex.StoragePoolCacheConfig{
			PowerFlexClient: newPowerFlexClient(powerFlexSvr.URL),
			Size:            2,
		}

		cache, err := powerflex.NewStoragePoolCache(config)
		if err != nil {
			t.Fatal(err)
		}

		// Act

		// Get storage pool name with ID 3df6b86600000000
		poolName, err := cache.GetStoragePoolNameByID("0")

		// Assert

		// Assert that the token is nil value
		if poolName != "" {
			t.Errorf("expected nil pool name value, got %s", poolName)
		}

		// Asser that err is not nil
		if err == nil {
			t.Errorf("expected an error, got nil")
		}
	})
}

func newPowerFlexClient(addr string) *goscaleio.Client {
	client, err := goscaleio.NewClientWithArgs(addr, "", false, false)
	if err != nil {
		panic(err)
	}

	return client
}
