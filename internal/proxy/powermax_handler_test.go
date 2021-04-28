package proxy

import (
	"context"
	"fmt"
	"io"
	"karavi-authorization/internal/quota"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
)

func TestPowerMaxHandler(t *testing.T) {
	t.Run("UpdateSystems", testPowerMaxUpdateSystems)
	t.Run("ServeHTTP", testPowerMaxServeHTTP)
}

func testPowerMaxServeHTTP(t *testing.T) {
	t.Run("it proxies requests", func(t *testing.T) {
		var gotRequestedPolicyPath string
		done := make(chan struct{})
		sut := buildPowerMaxHandler(t,
			withUnisphereServer(func(w http.ResponseWriter, r *http.Request) {
				done <- struct{}{}
			}),
			withOPAServer(func(w http.ResponseWriter, r *http.Request) {
				gotRequestedPolicyPath = r.URL.Path
				fmt.Fprintf(w, `{ "result": { "allow": true } }`)
			}),
		)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Forwarded", "for=https://10.0.0.1;000197900714")
		w := httptest.NewRecorder()

		go sut.ServeHTTP(w, r)

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("timed out waiting for proxied request")
		}
		wantRequestedPolicyPath := "/v1/data/karavi/authz/powermax/url"
		if gotRequestedPolicyPath != wantRequestedPolicyPath {
			t.Errorf("OPAPolicyPath: got %q, want %q",
				gotRequestedPolicyPath, wantRequestedPolicyPath)
		}
	})
	t.Run("it returns 502 Bad Gateway on unknown system", func(t *testing.T) {
		sut := buildPowerMaxHandler(t)
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("Forwarded", "for=https://10.0.0.1;0000000000") // pass unknown system ID
		w := httptest.NewRecorder()

		sut.ServeHTTP(w, r)

		want := http.StatusBadGateway
		if got := w.Result().StatusCode; got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	})
}

func testPowerMaxUpdateSystems(t *testing.T) {
	var tests = []struct {
		name                string
		given               io.Reader
		expectedErr         error
		expectedSystemCount int
	}{
		{"invalid json", strings.NewReader(""), io.EOF, 0},
		{"remove system", strings.NewReader("{}"), nil, 0},
		{"add system", strings.NewReader(systemJSON("test")), nil, 1},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			sut := buildPowerMaxHandler(t)

			err := sut.UpdateSystems(context.Background(), tt.given)

			if tt.expectedErr != nil {
				if err != tt.expectedErr {
					t.Fatalf("UpdateSystems: got err = %v, want = %v", err, tt.expectedErr)
				}
				return
			}
			want := tt.expectedSystemCount
			if got := len(sut.systems); got != want {
				t.Errorf("%s: got system count %d, want %d", tt.name, got, want)
			}
		})
	}
}

type powermaxHandlerOption func(*testing.T, *PowerMaxHandler)

func withUnisphereServer(h http.HandlerFunc) powermaxHandlerOption {
	return func(t *testing.T, pmh *PowerMaxHandler) {
		fakeUnisphere := fakeServer(t, h)
		err := pmh.UpdateSystems(context.Background(), strings.NewReader(systemJSON(fakeUnisphere.URL)))
		if err != nil {
			t.Fatal(err)
		}
	}
}

func withOPAServer(h http.HandlerFunc) powermaxHandlerOption {
	return func(t *testing.T, pmh *PowerMaxHandler) {
		fakeOPA := fakeServer(t, h)
		pmh.opaHost = hostPortFromFakeServer(t, fakeOPA)
	}
}

func withEnforcer(v *quota.RedisEnforcement) powermaxHandlerOption {
	return func(t *testing.T, pmh *PowerMaxHandler) {
		pmh.enforcer = v
	}
}

func withLogger(logger *logrus.Entry) powermaxHandlerOption {
	return func(t *testing.T, pmh *PowerMaxHandler) {
		pmh.log = logger
	}
}

func buildPowerMaxHandler(t *testing.T, opts ...powermaxHandlerOption) *PowerMaxHandler {
	defaultOptions := []powermaxHandlerOption{
		withLogger(testLogger()), // order matters for this one.
		withUnisphereServer(func(w http.ResponseWriter, r *http.Request) {}),
		withOPAServer(func(w http.ResponseWriter, r *http.Request) {}),
	}

	ret := PowerMaxHandler{}

	for _, opt := range defaultOptions {
		opt(t, &ret)
	}
	for _, opt := range opts {
		opt(t, &ret)
	}

	return &ret
}

func testLogger() *logrus.Entry {
	logger := logrus.New().WithContext(context.Background())
	logger.Logger.SetOutput(os.Stdout)
	return logger
}

func fakeEnforcer() *quota.RedisEnforcement {
	return quota.NewRedisEnforcement(context.Background())
}

func hostPortFromFakeServer(t *testing.T, testServer *httptest.Server) string {
	parsedURL, err := url.Parse(testServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	return parsedURL.Host
}

func fakeServer(t *testing.T, h http.Handler) *httptest.Server {
	s := httptest.NewServer(h)
	t.Cleanup(func() {
		s.Close()
	})
	return s
}

func systemJSON(endpoint string) string {
	return fmt.Sprintf(`{
	  "powermax": {
	    "000197900714": {
	      "endpoint": "%s",
	      "user": "smc",
	      "pass": "smc",
	      "insecure": true
	    }
	  }
	}
	`, endpoint)
}

func systemObject(endpoint string) SystemConfig {
	return SystemConfig{
		"powermax": Family{
			"000197900714": SystemEntry{
				Endpoint: endpoint,
				User:     "smc",
				Password: "smc",
				Insecure: true,
			},
		},
	}
}
