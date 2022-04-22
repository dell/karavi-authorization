package validate_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"karavi-authorization/internal/role-service/validate"
	"karavi-authorization/internal/types"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidatePowerFlex(t *testing.T) {
	// create mock backend pwoerflex
	ts := httptest.NewTLSServer(
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
	defer ts.Close()

	// temporarily set validate.GetPowerFlexEndpoint to mock powerflex
	oldGetPowerFlexEndpoint := validate.GetPowerFlexEndpoint
	validate.GetPowerFlexEndpoint = func(system types.System) string {
		return ts.URL
	}
	defer func() { validate.GetPowerFlexEndpoint = oldGetPowerFlexEndpoint }()

	/*tests := map[string]func(t *testing.T) (string, int){
		"success creating role with json file": func(*testing.T) (string, int) {
			return "--role=NewRole1=powerflex=542a2d5f5122210f=bronze=9GB", 0
		},
		"failure creating role with negative quota": func(*testing.T) (string, int) {
			return "--role=NewRole1=powerflex=542a2d5f5122210f=bronze=-2GB", 1
		},
	}*/

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
	tests := []struct {
		name     string
		system   types.System
		systemId string
		pool     string
		quota    int64
		checkFn  checkFn
	}{
		{
			"success",
			types.System{},
			"542a2d5f5122210f",
			"bronze",
			1000,
			errIsNil,
		},
		{
			"negative quota",
			types.System{},
			"542a2d5f5122210f",
			"bronze",
			-1,
			errIsNotNil,
		},
	}

	// run the tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validate.ValidatePowerFlex(context.Background(), tc.system, tc.systemId, tc.pool, tc.quota)
			tc.checkFn(t, err)
		})
	}
}

func write(t *testing.T, w io.Writer, file string) {
	b, err := ioutil.ReadFile(fmt.Sprintf("testdata/%s", file))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(w, bytes.NewReader(b)); err != nil {
		t.Fatal(err)
	}
}
