package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"karavi-authorization/internal/tenantsvc/mocks"
	"karavi-authorization/pb"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

func TestTenantHandler(t *testing.T) {
	t.Run("it handles tenant create", func(t *testing.T) {
		t.Run("successfully creates a tenant", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				CreateTenantFn: func(ctx context.Context, ctr *pb.CreateTenantRequest, co ...grpc.CallOption) (*pb.Tenant, error) {
					return &pb.Tenant{
						Name:       "test",
						Approvesdc: true,
					}, nil
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&createTenantBody{
				Name:       "test",
				ApproveSdc: true,
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPost, "/proxy/tenant/create", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusOK {
				t.Errorf("expected status code %d, got %d", http.StatusOK, code)
			}
		})
		t.Run("handles bad request", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				CreateTenantFn: func(ctx context.Context, ctr *pb.CreateTenantRequest, co ...grpc.CallOption) (*pb.Tenant, error) {
					return &pb.Tenant{
						Name:       "test",
						Approvesdc: true,
					}, nil
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodGet, "/proxy/tenant/create", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusMethodNotAllowed {
				t.Errorf("expected status code %d, got %d", http.StatusMethodNotAllowed, code)
			}
		})
		t.Run("handles malformed request body", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				CreateTenantFn: func(ctx context.Context, ctr *pb.CreateTenantRequest, co ...grpc.CallOption) (*pb.Tenant, error) {
					return &pb.Tenant{
						Name:       "test",
						Approvesdc: true,
					}, nil
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			r := httptest.NewRequest(http.MethodPost, "/proxy/tenant/create", nil)
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusBadRequest {
				t.Errorf("expected status code %d, got %d", http.StatusBadRequest, code)
			}
		})
		t.Run("handles error from tenant service", func(t *testing.T) {
			client := &mocks.FakeTenantServiceClient{
				CreateTenantFn: func(ctx context.Context, ctr *pb.CreateTenantRequest, co ...grpc.CallOption) (*pb.Tenant, error) {
					return nil, errors.New("error")
				},
			}

			sut := NewTenantHandler(logrus.NewEntry(logrus.New()), client)

			payload, err := json.Marshal(&createTenantBody{
				Name:       "test",
				ApproveSdc: true,
			})
			if err != nil {
				t.Fatal(err)
			}

			r := httptest.NewRequest(http.MethodPost, "/proxy/tenant/create", bytes.NewReader(payload))
			w := httptest.NewRecorder()

			sut.ServeHTTP(w, r)

			code := w.Result().StatusCode
			if code != http.StatusInternalServerError {
				t.Errorf("expected status code %d, got %d", http.StatusInternalServerError, code)
			}
		})
	})
}
