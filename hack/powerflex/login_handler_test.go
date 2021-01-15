package powerflex_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"powerflex-reverse-proxy/hack/powerflex"
	"testing"
	"time"
)

var (
	firstToken  = "YWRtaW46MTYxMDUxNzk5NDQxODpjYzBkMGEwMmUwYzNiODUxOTM1NWMxZThkNTcwZWEwNA"
	secondToken = "YWRtaW46MTYxMDU3OTI1NjMyMjo2MGFiNTIyYTcxYjEwMGM3ZTdlYzRhMDU3MDA1MjNhMw"
)

func TestLogin_GetToken(t *testing.T) {
	t.Run("success getting a token", func(t *testing.T) {
		// Arrange

		// Setup httptest server to represent a PowerFlex
		powerFlexSvr := newPowerFlexTestServer(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.String() {
			case "/api/version":
				w.Write([]byte("3.5"))
			case "/api/login":
				w.Write([]byte(firstToken))
			default:
				panic(fmt.Sprintf("path %s not supported", r.URL.String()))
			}
		})
		defer powerFlexSvr.Close()

		// Create a new LoginHandler pointing to the httptest server PowerFlex
		// TokenRefreshInterval shouldn't be relevant in this test case
		config := powerflex.Config{
			PowerFlexClient:      newPowerFlexClient(powerFlexSvr.URL),
			TokenRefreshInterval: time.Minute,
		}
		lh := powerflex.NewLoginHandler(config)

		// Act

		// Get a token
		token, err := lh.GetToken(context.Background())

		// Assert

		// Assert that the token we got is the expected token from the httptest server PowerFlex
		if token != firstToken {
			t.Errorf("expected token %s, got %s", firstToken, token)
		}

		// Assert that err is nil
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("success getting a token during refresh", func(t *testing.T) {
		// Arrange

		// Variable to keep track of the /api/login call count so we can return different things in the following httptest server
		powerFlexCallCount := 0

		// Setup httptest server to represent a PowerFlex
		powerFlexSvr := newPowerFlexTestServer(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.String() {
			case "/api/version":
				w.Write([]byte("3.5"))
			case "/api/login":
				switch powerFlexCallCount {
				case 0:
					w.Write([]byte(firstToken))
					powerFlexCallCount++
				case 1:
					// Sleep to simulate this call taking longer
					time.Sleep(2 * time.Second)
					w.Write([]byte(secondToken))
				default:
					panic("unexpected call to httptest server")
				}
			default:
				panic(fmt.Sprintf("path %s not supported", r.URL.String()))
			}
		})
		defer powerFlexSvr.Close()

		// Create a new LoginHandler pointing to the httptest server PowerFlex
		config := powerflex.Config{
			PowerFlexClient:      newPowerFlexClient(powerFlexSvr.URL),
			TokenRefreshInterval: time.Second,
		}
		lh := powerflex.NewLoginHandler(config)

		// Act

		// Wait for refresh interval to start
		<-time.After(time.Second)

		// Get a token while LoginHandler is refreshing
		token, err := lh.GetToken(context.Background())

		// Assert

		// Assert that the token we got is the expected token from the httptest server PowerFlex
		if token != secondToken {
			t.Errorf("expected token %s, got %s", secondToken, token)
		}

		// Assert that err is nil
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	})

	t.Run("timeout getting a token during refresh", func(t *testing.T) {
		// Arrange

		// Variable to keep track of the /api/login call count so we can return different things in the following httptest server
		powerFlexCallCount := 0

		// Setup httptest server to represent a PowerFlex
		powerFlexSvr := newPowerFlexTestServer(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.String() {
			case "/api/version":
				w.Write([]byte("3.5"))
			case "/api/login":
				switch powerFlexCallCount {
				case 0:
					w.Write([]byte(firstToken))
					powerFlexCallCount++
				case 1:
					// Sleep to simulate this call taking longer
					time.Sleep(5 * time.Second)
				default:
					panic("unexpected call to httptest server")
				}
			default:
				panic(fmt.Sprintf("path %s not supported", r.URL.String()))
			}
		})
		defer powerFlexSvr.Close()

		// Create a new LoginHandler pointing to the httptest server PowerFlex
		config := powerflex.Config{
			PowerFlexClient:      newPowerFlexClient(powerFlexSvr.URL),
			TokenRefreshInterval: time.Second,
		}
		lh := powerflex.NewLoginHandler(config)

		// Act

		// Wait for refresh interval to start
		<-time.After(time.Second)

		// Create a timeout context
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancel()

		// Get a token while LoginHandler is refreshing
		token, err := lh.GetToken(ctx)

		// Assert

		// Assert that the token is nil value
		if token != "" {
			t.Errorf("expected nil token value, got %s", token)
		}

		// Asser that the errror is the context error
		if ctx.Err() != err {
			t.Errorf("expected context error %v to be equal to error returned from GetToken, got %v", ctx.Err(), err)
		}
	})
}

func newPowerFlexTestServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}
