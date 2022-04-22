package validate_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/internal/role-service/validate"
	"karavi-authorization/internal/types"
	"net/http"
	"net/http/httptest"
	"testing"

	v1 "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
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

	// configure fake k8s with storage secret
	data := []byte(fmt.Sprintf(`
storage:
  powerflex:
    542a2d5f5122210f:
      Endpoint: %s
      Insecure: true
      Pass: Password123
      User: admin`, ts.URL))

	secret := &v1.Secret{
		ObjectMeta: meta.ObjectMeta{
			Name:      validate.STORAGE_SECRET,
			Namespace: "test",
		},
		Data: map[string][]byte{
			validate.STORAGE_SECRET_DATA_KEY: data,
		},
	}

	fakeClient := fake.NewSimpleClientset()
	_, err := fakeClient.CoreV1().Secrets("test").Create(context.Background(), secret, meta.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}

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
		name    string
		role    *roles.Instance
		checkFn checkFn
	}{
		{
			"success",
			&roles.Instance{
				RoleKey: roles.RoleKey{
					Name:       "success",
					SystemType: "powerflex",
					SystemID:   "542a2d5f5122210f",
					Pool:       "bronze",
				},
				Quota: 1000,
			},
			errIsNil,
		},
		{
			"negative quota",
			&roles.Instance{
				RoleKey: roles.RoleKey{
					Name:       "negative quota",
					SystemType: "powerflex",
					SystemID:   "542a2d5f5122210f",
					Pool:       "bronze",
				},
				Quota: -1,
			},
			errIsNotNil,
		},
	}

	rv := validate.NewRoleValidator(fakeClient, "test")

	// run the tests
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := rv.Validate(context.Background(), tc.role)
			tc.checkFn(t, err)
		})
	}
}

func TestValidatePowerMax(t *testing.T) {

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
