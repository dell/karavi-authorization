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

package proxy_test

import (
	"bytes"
	"context"
	"crypto/tls"

	//	"encoding/base64"
	"encoding/json"
	"fmt"
	"karavi-authorization/internal/proxy"
	"karavi-authorization/internal/quota"
	"karavi-authorization/internal/web"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dgrijalva/jwt-go"
	redisclient "github.com/go-redis/redis"
	"github.com/orlangure/gnomock"

	//"github.com/orlangure/gnomock/preset/redis"
	"github.com/sirupsen/logrus"
)

func init() {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
}

func TestPowerFlex(t *testing.T) {
	t.Run("it spoofs login requests from clients", func(t *testing.T) {
		// Logging.
		log := logrus.New().WithContext(context.Background())
		log.Logger.SetOutput(os.Stdout)
		// Prepare the login request.
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/login", nil)
		// Build a fake powerflex backend, since it will try to login for real.
		// We'll use the URL of this test server as part of the systems config.
		fakePowerFlex := buildTestTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		}))
		fakeOPA := buildTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"result": {"allow": true}}`))
		}))
		// Add headers that the sidecar-proxy would add, in order to identify
		// the request as intended for a PowerFlex with the given systemID.
		r.Header.Add("Forwarded", "by=csi-vxflexos")
		r.Header.Add("Forwarded", fmt.Sprintf("for=https://%s;542a2d5f5122210f", fakePowerFlex.URL))
		// Create the router and assign the appropriate handlers.
		rtr := newTestRouter()
		// Create the PowerFlex handler and configure it with a system
		// where the endpoint is our test server.
		powerFlexHandler := proxy.NewPowerFlexHandler(log, nil, hostPort(t, fakeOPA.URL))
		powerFlexHandler.UpdateSystems(strings.NewReader(fmt.Sprintf(`
{
  "powerflex": {
    "542a2d5f5122210f": {
      "endpoint": "%s",
      "user": "admin",
      "pass": "Password123",
      "insecure": true
    }
  }
}
`, fakePowerFlex.URL)))
		systemHandlers := map[string]http.Handler{
			"powerflex": web.Adapt(powerFlexHandler),
		}
		dh := proxy.NewDispatchHandler(log, systemHandlers)
		rtr.ProxyHandler = dh
		h := web.Adapt(rtr.Handler(), web.CleanMW())

		h.ServeHTTP(w, r)

		if got, want := w.Result().StatusCode, http.StatusOK; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
		// This response should come from our PowerFlex handler, NOT the (fake)
		// PowerFlex itself.
		got := string(w.Body.Bytes())
		want := "hellofromkaravi"
		if !strings.Contains(got, want) {
			t.Errorf("got %q, expected response body to contain %q", got, want)
		}
	})

	t.Run("it proxies immutable requests to the PowerFlex", func(t *testing.T) {
		// Logging.
		log := logrus.New().WithContext(context.Background())
		log.Logger.SetOutput(os.Stdout)
		// Prepare the login request.
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/version/", nil)
		// Build a fake powerflex backend, since it will try to login for real.
		// We'll use the URL of this test server as part of the systems config.
		done := make(chan struct{})
		fakePowerFlex := buildTestTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Logf("Test request path is %q", r.URL.Path)
			if r.URL.Path == "/api/version/" {
				done <- struct{}{}
			}
		}))
		fakeOPA := buildTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(`{"result": {"allow": true}}`))
		}))
		// Add headers that the sidecar-proxy would add, in order to identify
		// the request as intended for a PowerFlex with the given systemID.
		r.Header.Add("Forwarded", "by=csi-vxflexos")
		r.Header.Add("Forwarded", fmt.Sprintf("for=https://%s;542a2d5f5122210f", fakePowerFlex.URL))
		// Create the router and assign the appropriate handlers.
		rtr := newTestRouter()
		// Create the PowerFlex handler and configure it with a system
		// where the endpoint is our test server.
		powerFlexHandler := proxy.NewPowerFlexHandler(log, nil, hostPort(t, fakeOPA.URL))
		powerFlexHandler.UpdateSystems(strings.NewReader(fmt.Sprintf(`
{
  "powerflex": {
    "542a2d5f5122210f": {
      "endpoint": "%s",
      "user": "admin",
      "pass": "Password123",
      "insecure": true
    }
  }
}
`, fakePowerFlex.URL)))
		systemHandlers := map[string]http.Handler{
			"powerflex": web.Adapt(powerFlexHandler),
		}
		dh := proxy.NewDispatchHandler(log, systemHandlers)
		rtr.ProxyHandler = dh
		h := web.Adapt(rtr.Handler(), web.CleanMW())

		go func() {
			h.ServeHTTP(w, r)
			done <- struct{}{}
		}()
		<-done
		<-done // we also need to wait for the HTTP request to fully complete.

		if got, want := w.Result().StatusCode, http.StatusOK; got != want {
			t.Errorf("got %v, want %v", got, want)
		}
	})
	t.Run("it denies tenant request to remove volume that tenant does not own", func(t *testing.T) {
		t.Skip("TODO: determine why these tests are breaking")
		// Logging.
		log := logrus.New().WithContext(context.Background())
		log.Logger.SetOutput(os.Stdout)
		log.Logger.SetLevel(logrus.DebugLevel)

		// Shared secret for signing tokens
		sharedSecret := "secret"

		// Prepare tenant A's token
		// Create the claims
		claimsA := struct {
			jwt.StandardClaims
			Role  string `json:"role"`
			Group string `json:"group"`
		}{
			StandardClaims: jwt.StandardClaims{
				Issuer:    "com.dell.karavi",
				ExpiresAt: time.Now().Add(30 * time.Second).Unix(),
				Audience:  "karavi",
				Subject:   "Alice",
			},
			Role:  "DevTesting",
			Group: "TestingGroup",
		}
		// Sign for an access token
		tokenA := jwt.NewWithClaims(jwt.SigningMethodHS256, claimsA)
		accessTokenA, err := tokenA.SignedString([]byte(sharedSecret))
		if err != nil {
			t.Errorf("Could not sign access token")
		}
		tokenA.Raw = accessTokenA

		// Prepare tenant B's token
		// Create the claims
		claimsB := struct {
			jwt.StandardClaims
			Role  string `json:"role"`
			Group string `json:"group"`
		}{
			StandardClaims: jwt.StandardClaims{
				Issuer:    "com.dell.karavi",
				ExpiresAt: time.Now().Add(30 * time.Second).Unix(),
				Audience:  "karavi",
				Subject:   "Bob",
			},
			Role:  "DevTesting",
			Group: "TestingGroup",
		}
		// Sign for an access token
		tokenB := jwt.NewWithClaims(jwt.SigningMethodHS256, claimsB)
		accessTokenB, err := tokenB.SignedString([]byte(sharedSecret))
		if err != nil {
			t.Errorf("Could not sign access token")
		}
		tokenB.Raw = accessTokenB

		// Prepare the create volume request.
		createBody := struct {
			VolumeSize     int64
			VolumeSizeInKb string `json:"volumeSizeInKb"`
			StoragePoolID  string `json:"storagePoolId"`
			Name           string `json:"name"`
		}{
			VolumeSize:     10,
			VolumeSizeInKb: "10",
			StoragePoolID:  "3df6b86600000000",
			Name:           "TestVolume",
		}
		data, err := json.Marshal(createBody)
		if err != nil {
			t.Fatal(err)
		}
		payload := bytes.NewBuffer(data)

		wVolCreate := httptest.NewRecorder()
		rVolCreate := httptest.NewRequest(http.MethodPost, "/api/types/Volume/instances", payload)
		rVolCreateContext := context.WithValue(context.Background(), web.JWTKey, tokenA)
		rVolCreate = rVolCreate.WithContext(rVolCreateContext)

		// Prepare the remove volume request.
		removeBody := struct {
			RemoveMode string `json:"removeMode"`
		}{
			RemoveMode: "ONLY_ME",
		}
		data, err = json.Marshal(removeBody)
		if err != nil {
			t.Fatal(err)
		}
		payload = bytes.NewBuffer(data)
		wVolDel := httptest.NewRecorder()
		rVolDel := httptest.NewRequest(http.MethodPost, "/api/instances/Volume::000000000000001/action/removeVolume", payload)
		rVolDelContext := context.WithValue(context.Background(), web.JWTKey, tokenB)
		rVolDel = rVolDel.WithContext(rVolDelContext)

		// Build a fake powerflex backend, since it will try to create and delete volumes for real.
		// We'll use the URL of this test server as part of the systems config.
		fakePowerFlex := buildTestTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/types/Volume/instances/":
				w.Write([]byte(`{"id":"000000000000001", "name": "TestVolume"}`))
			case "/api/instances/Volume::000000000000001":
				w.Write([]byte(`{"sizeInKb":10, "storagePoolId":"3df6b86600000000", "name": "TestVolume"}`))
			case "/api/login":
				w.Write([]byte("token"))
			case "/api/version":
				w.Write([]byte("3.5"))
			case "/api/types/StoragePool/instances":
				w.Write([]byte(`[{"protectionDomainId": "75b661b400000000", "mediaType": "HDD", "id": "3df6b86600000000", "name": "TestPool"}]`))
			default:
				t.Errorf("Unexpected api call to fake PowerFlex: %v", r.URL.Path)
			}
		}))
		fakeOPA := buildTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Logf("Incoming OPA request: %v", r.URL.Path)
			switch r.URL.Path {
			case "/v1/data/karavi/authz/url":
				w.Write([]byte(`{"result": {"allow": true}}`))
			case "/v1/data/karavi/volumes/create":
				w.Write([]byte(`{"result": { "response": {"allowed": true, "status": {"reason": "ok"}}, "token": {"group": "TestingGroup"}, "quota": 99999}}`))
			case "/v1/data/karavi/volumes/delete":
				w.Write([]byte(`{"result": { "response": {"allowed": true, "status": {"reason": "ok"}}, "token": {"group": "TestingGroup"}, "quota": 99999}}`))
			}
		}))

		// Add headers that the sidecar-proxy would add, in order to identify
		// the request as intended for a PowerFlex with the given systemID.
		rVolCreate.Header.Add("Forwarded", "by=csi-vxflexos")
		rVolCreate.Header.Add("Forwarded", fmt.Sprintf("for=%s;542a2d5f5122210f", fakePowerFlex.URL))

		// Add headers that the sidecar-proxy would add, in order to identify
		// the request as intended for a PowerFlex with the given systemID.
		rVolDel.Header.Add("Forwarded", "by=csi-vxflexos")
		rVolDel.Header.Add("Forwarded", fmt.Sprintf("for=%s;542a2d5f5122210f", fakePowerFlex.URL))

		// Create the router and assign the appropriate handlers.
		rtr := newTestRouter()
		// Create a redis enforcer
		redisContainer, err := gnomock.StartCustom("docker.io/library/redis:latest", gnomock.NamedPorts{"db": gnomock.TCP(6379)}, gnomock.WithDisableAutoCleanup(), gnomock.WithContainerName("redis-test"))
		if err != nil {
			t.Errorf("failed to start redis container: %+v", err)
		}
		rdb := redisclient.NewClient(&redisclient.Options{
			Addr: redisContainer.Address("db"),
		})
		defer func() {
			if err := rdb.Close(); err != nil {
				log.Printf("closing redis: %+v", err)
			}
			if err := gnomock.Stop(redisContainer); err != nil {
				log.Printf("stopping redis container: %+v", err)
			}
		}()
		enf := quota.NewRedisEnforcement(context.Background(), rdb)

		// Create the PowerFlex handler and configure it with a system
		// where the endpoint is our test server.
		powerFlexHandler := proxy.NewPowerFlexHandler(log, enf, hostPort(t, fakeOPA.URL))
		powerFlexHandler.UpdateSystems(strings.NewReader(fmt.Sprintf(`
{
  "powerflex": {
    "542a2d5f5122210f": {
      "endpoint": "%s",
      "user": "admin",
      "pass": "Password123",
      "insecure": true
    }
  }
}
`, fakePowerFlex.URL)))
		systemHandlers := map[string]http.Handler{
			"powerflex": web.Adapt(powerFlexHandler),
		}
		dh := proxy.NewDispatchHandler(log, systemHandlers)
		rtr.ProxyHandler = dh
		h := web.Adapt(rtr.Handler(), web.CleanMW())

		h.ServeHTTP(wVolCreate, rVolCreate)
		h.ServeHTTP(wVolDel, rVolDel)

		if got, want := wVolCreate.Result().StatusCode, http.StatusOK; got != want {
			fmt.Printf("Create request: %v\n", *rVolCreate)
			fmt.Printf("Create response: %v\n", string(wVolCreate.Body.Bytes()))
			t.Errorf("got %v, want %v", got, want)
		}

		if got, want := wVolDel.Result().StatusCode, http.StatusForbidden; got != want {
			fmt.Printf("Remove request: %v\n", *rVolDel)
			fmt.Printf("Remove response: %v\n", string(wVolDel.Body.Bytes()))
			t.Errorf("got %v, want %v", got, want)
		}

		// This response should come from our PowerFlex handler, NOT the (fake)
		// PowerFlex itself.
		type DeleteRequestResponse struct {
			ErrorCode      int    `json:"errorCode"`
			HttpStatusCode int    `json:"httpStatusCode"`
			Message        string `json:"message"`
		}
		got := DeleteRequestResponse{}
		err = json.Unmarshal(wVolDel.Body.Bytes(), &got)
		if err != nil {
			t.Errorf("error demarshalling volume delete request response: %v", err)
		}
		want := DeleteRequestResponse{
			ErrorCode:      403,
			HttpStatusCode: 403,
			Message:        "request denied",
		}
		if !strings.Contains(got.Message, want.Message) || got.ErrorCode != want.ErrorCode || got.HttpStatusCode != want.HttpStatusCode {
			t.Errorf("got %q, expected response body to contain %q", got, want)
		}
	})
	t.Run("it denies tenant request to unmap volume that tenant does not own", func(t *testing.T) {
		t.Skip("TODO: determine why these tests are breaking")
		// Logging.
		log := logrus.New().WithContext(context.Background())
		log.Logger.SetOutput(os.Stdout)
		log.Logger.SetLevel(logrus.DebugLevel)

		// Shared secret for signing tokens
		sharedSecret := "secret"

		// Prepare tenant A's token
		// Create the claims
		claimsA := struct {
			jwt.StandardClaims
			Role  string `json:"role"`
			Group string `json:"group"`
		}{
			StandardClaims: jwt.StandardClaims{
				Issuer:    "com.dell.karavi",
				ExpiresAt: time.Now().Add(30 * time.Second).Unix(),
				Audience:  "karavi",
				Subject:   "Alice",
			},
			Role:  "DevTesting",
			Group: "TestingGroup",
		}
		// Sign for an access token
		tokenA := jwt.NewWithClaims(jwt.SigningMethodHS256, claimsA)
		accessTokenA, err := tokenA.SignedString([]byte(sharedSecret))
		if err != nil {
			t.Errorf("Could not sign access token")
		}
		tokenA.Raw = accessTokenA

		// Prepare tenant B's token
		// Create the claims
		claimsB := struct {
			jwt.StandardClaims
			Role  string `json:"role"`
			Group string `json:"group"`
		}{
			StandardClaims: jwt.StandardClaims{
				Issuer:    "com.dell.karavi",
				ExpiresAt: time.Now().Add(30 * time.Second).Unix(),
				Audience:  "karavi",
				Subject:   "Bob",
			},
			Role:  "DevTesting",
			Group: "TestingGroup",
		}
		// Sign for an access token
		tokenB := jwt.NewWithClaims(jwt.SigningMethodHS256, claimsB)
		accessTokenB, err := tokenB.SignedString([]byte(sharedSecret))
		if err != nil {
			t.Errorf("Could not sign access token")
		}
		tokenB.Raw = accessTokenB

		// Prepare the create volume request.
		createBody := struct {
			VolumeSize     int64
			VolumeSizeInKb string `json:"volumeSizeInKb"`
			StoragePoolID  string `json:"storagePoolId"`
			Name           string `json:"name"`
		}{
			VolumeSize:     10,
			VolumeSizeInKb: "10",
			StoragePoolID:  "3df6b86600000000",
			Name:           "TestVolume",
		}
		data, err := json.Marshal(createBody)
		if err != nil {
			t.Fatal(err)
		}
		payload := bytes.NewBuffer(data)

		wVolCreate := httptest.NewRecorder()
		rVolCreate := httptest.NewRequest(http.MethodPost, "/api/types/Volume/instances", payload)
		rVolCreateContext := context.WithValue(context.Background(), web.JWTKey, tokenA)
		rVolCreate = rVolCreate.WithContext(rVolCreateContext)

		// Prepare the map volume request.
		mapBody := struct {
			SdcID string `json:"sdcId"`
		}{
			SdcID: "21b4653900000000",
		}
		data, err = json.Marshal(mapBody)
		if err != nil {
			t.Fatal(err)
		}
		payload = bytes.NewBuffer(data)

		wVolMap := httptest.NewRecorder()
		rVolMap := httptest.NewRequest(http.MethodPost, "/api/instances/Volume::000000000000001/action/addMappedSdc", payload)
		rVolMapContext := context.WithValue(context.Background(), web.JWTKey, tokenA)
		rVolMap = rVolMap.WithContext(rVolMapContext)

		// Prepare the unmap volume request.
		unmapBody := struct {
			SdcID string `json:"sdcId"`
		}{
			SdcID: "21b4653900000000",
		}
		data, err = json.Marshal(unmapBody)
		if err != nil {
			t.Fatal(err)
		}
		payload = bytes.NewBuffer(data)
		wVolUnmap := httptest.NewRecorder()
		rVolUnmap := httptest.NewRequest(http.MethodPost, "/api/instances/Volume::000000000000001/action/removeMappedSdc", payload)
		rVolUnmapContext := context.WithValue(context.Background(), web.JWTKey, tokenB)
		rVolUnmap = rVolUnmap.WithContext(rVolUnmapContext)

		// Build a fake powerflex backend, since it will try to create and delete volumes for real.
		// We'll use the URL of this test server as part of the systems config.
		fakePowerFlex := buildTestTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/types/Volume/instances/":
				w.Write([]byte(`{"id":"000000000000001", "name": "TestVolume"}`))
			case "/api/instances/Volume::000000000000001":
				w.Write([]byte(`{"sizeInKb":10, "storagePoolId":"3df6b86600000000", "name": "TestVolume"}`))
			case "/api/login":
				w.Write([]byte("token"))
			case "/api/version":
				w.Write([]byte("3.5"))
			case "/api/types/StoragePool/instances":
				w.Write([]byte(`[{"protectionDomainId": "75b661b400000000", "mediaType": "HDD", "id": "3df6b86600000000", "name": "TestPool"}]`))
			case "/api/instances/Volume::000000000000001/action/addMappedSdc/":
				w.WriteHeader(http.StatusOK)
			case "/api/instances/Volume::000000000000001/action/removeMappedSdc/":
				w.WriteHeader(http.StatusOK)
			default:
				t.Errorf("Unexpected api call to fake PowerFlex: %v", r.URL.Path)
			}
		}))
		fakeOPA := buildTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Logf("Incoming OPA request: %v", r.URL.Path)
			switch r.URL.Path {
			case "/v1/data/karavi/authz/url":
				w.Write([]byte(`{"result": {"allow": true}}`))
			case "/v1/data/karavi/volumes/create":
				w.Write([]byte(`{"result": { "response": {"allowed": true, "status": {"reason": "ok"}}, "token": {"group": "TestingGroup"}, "quota": 99999}}`))
			case "/v1/data/karavi/volumes/unmap":
				w.Write([]byte(`{"result": { "response": {"allowed": true, "status": {"reason": "ok"}}, "token": {"group": "TestingGroup"}, "quota": 99999}}`))
			}
		}))

		// Add headers that the sidecar-proxy would add, in order to identify
		// the request as intended for a PowerFlex with the given systemID.
		rVolCreate.Header.Add("Forwarded", "by=csi-vxflexos")
		rVolCreate.Header.Add("Forwarded", fmt.Sprintf("for=%s;542a2d5f5122210f", fakePowerFlex.URL))

		rVolMap.Header.Add("Forwarded", "by=csi-vxflexos")
		rVolMap.Header.Add("Forwarded", fmt.Sprintf("for=%s;542a2d5f5122210f", fakePowerFlex.URL))

		rVolUnmap.Header.Add("Forwarded", "by=csi-vxflexos")
		rVolUnmap.Header.Add("Forwarded", fmt.Sprintf("for=%s;542a2d5f5122210f", fakePowerFlex.URL))

		// Create the router and assign the appropriate handlers.
		rtr := newTestRouter()
		// Create a redis enforcer
		redisContainer, err := gnomock.StartCustom("docker.io/library/redis:latest", gnomock.NamedPorts{"db": gnomock.TCP(6379)}, gnomock.WithDisableAutoCleanup(), gnomock.WithContainerName("redis-test"))
		if err != nil {
			t.Errorf("failed to start redis container: %+v", err)
		}
		rdb := redisclient.NewClient(&redisclient.Options{
			Addr: redisContainer.Address("db"),
		})
		defer func() {
			if err := rdb.Close(); err != nil {
				log.Printf("closing redis: %+v", err)
			}
			if err := gnomock.Stop(redisContainer); err != nil {
				log.Printf("stopping redis container: %+v", err)
			}
		}()
		enf := quota.NewRedisEnforcement(context.Background(), rdb)

		// Create the PowerFlex handler and configure it with a system
		// where the endpoint is our test server.
		powerFlexHandler := proxy.NewPowerFlexHandler(log, enf, hostPort(t, fakeOPA.URL))
		powerFlexHandler.UpdateSystems(strings.NewReader(fmt.Sprintf(`
{
  "powerflex": {
    "542a2d5f5122210f": {
      "endpoint": "%s",
      "user": "admin",
      "pass": "Password123",
      "insecure": true
    }
  }
}
`, fakePowerFlex.URL)))
		systemHandlers := map[string]http.Handler{
			"powerflex": web.Adapt(powerFlexHandler),
		}
		dh := proxy.NewDispatchHandler(log, systemHandlers)
		rtr.ProxyHandler = dh
		h := web.Adapt(rtr.Handler(), web.CleanMW())

		h.ServeHTTP(wVolCreate, rVolCreate)
		h.ServeHTTP(wVolMap, rVolMap)
		h.ServeHTTP(wVolUnmap, rVolUnmap)

		if got, want := wVolCreate.Result().StatusCode, http.StatusOK; got != want {
			fmt.Printf("Create request: %v\n", *rVolCreate)
			fmt.Printf("Create response: %v\n", string(wVolCreate.Body.Bytes()))
			t.Errorf("got %v, want %v", got, want)
		}

		if got, want := wVolMap.Result().StatusCode, http.StatusOK; got != want {
			fmt.Printf("Map request: %v\n", *rVolMap)
			fmt.Printf("Map response: %v\n", string(wVolMap.Body.Bytes()))
			t.Errorf("got %v, want %v", got, want)
		}

		if got, want := wVolUnmap.Result().StatusCode, http.StatusForbidden; got != want {
			fmt.Printf("Unmap request: %v\n", *rVolUnmap)
			fmt.Printf("Unmap response: %v\n", string(wVolUnmap.Body.Bytes()))
			t.Errorf("got %v, want %v", got, want)
		}

		// This response should come from our PowerFlex handler, NOT the (fake)
		// PowerFlex itself.
		type UnmapRequestResponse struct {
			ErrorCode      int    `json:"errorCode"`
			HttpStatusCode int    `json:"httpStatusCode"`
			Message        string `json:"message"`
		}
		got := UnmapRequestResponse{}
		err = json.Unmarshal(wVolUnmap.Body.Bytes(), &got)
		if err != nil {
			t.Errorf("error demarshalling volume delete request response: %v", err)
		}
		want := UnmapRequestResponse{
			ErrorCode:      403,
			HttpStatusCode: 403,
			Message:        "request denied",
		}
		if !strings.Contains(got.Message, want.Message) || got.ErrorCode != want.ErrorCode || got.HttpStatusCode != want.HttpStatusCode {
			t.Errorf("got %q, expected response body to contain %q", got, want)
		}
	})
}

func newTestRouter() *web.Router {
	noopHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	return &web.Router{
		ProxyHandler:               noopHandler,
		RolesHandler:               noopHandler,
		TokenHandler:               noopHandler,
		ClientInstallScriptHandler: noopHandler,
	}
}

func buildTestTLSServer(t *testing.T, h ...http.Handler) *httptest.Server {
	var handler http.Handler
	switch len(h) {
	case 0:
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	case 1:
		handler = h[0]
	}
	ts := httptest.NewTLSServer(handler)
	t.Cleanup(func() {
		ts.Close()
	})
	return ts
}

func buildTestServer(t *testing.T, h ...http.Handler) *httptest.Server {
	var handler http.Handler
	switch len(h) {
	case 0:
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	case 1:
		handler = h[0]
	}
	ts := httptest.NewServer(handler)
	t.Cleanup(func() {
		ts.Close()
	})
	return ts
}

func hostPort(t *testing.T, u string) string {
	t.Helper()
	p, err := url.Parse(u)
	if err != nil {
		t.Fatal(err)
	}
	return p.Host
}
