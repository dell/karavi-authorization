// Copyright © 2021-2022 Dell Inc., or its subsidiaries. All Rights Reserved.
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

package proxy_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"karavi-authorization/internal/proxy"
	"karavi-authorization/internal/quota"
	"karavi-authorization/internal/token"
	"karavi-authorization/internal/token/jwx"
	"karavi-authorization/internal/web"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redis"
	redisclient "github.com/go-redis/redis"
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
		r.Header.Add("Forwarded", fmt.Sprintf("for=%s;542a2d5f5122210f", fakePowerFlex.URL))
		// Create the router and assign the appropriate handlers.
		rtr := newTestRouter()
		// Create the PowerFlex handler and configure it with a system
		// where the endpoint is our test server.
		powerFlexHandler := proxy.NewPowerFlexHandler(log, nil, hostPort(t, fakeOPA.URL))
		powerFlexHandler.UpdateSystems(context.Background(), strings.NewReader(fmt.Sprintf(`
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
	`, fakePowerFlex.URL)), logrus.New().WithContext(context.Background()))
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
		powerFlexHandler.UpdateSystems(context.Background(), strings.NewReader(fmt.Sprintf(`
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
	`, fakePowerFlex.URL)), logrus.New().WithContext(context.Background()))
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
		// Logging.
		log := logrus.New().WithContext(context.Background())
		log.Logger.SetOutput(os.Stdout)

		// Token manager
		tm := jwx.NewTokenManager(jwx.HS256)

		// Prepare tenant A's token
		// Create the claims
		claimsA := token.Claims{
			Issuer:    "com.dell.karavi",
			ExpiresAt: time.Now().Add(30 * time.Second).Unix(),
			Audience:  "karavi",
			Subject:   "Alice",
			Roles:     "DevTesting",
			Group:     "TestingGroup",
		}

		tokenA, err := tm.NewWithClaims(claimsA)
		if err != nil {
			t.Fatal(err)
		}

		// Prepare tenant B's token
		// Create the claims
		claimsB := token.Claims{
			Issuer:    "com.dell.karavi",
			ExpiresAt: time.Now().Add(30 * time.Second).Unix(),
			Audience:  "karavi",
			Subject:   "Bob",
			Roles:     "DevTesting",
			Group:     "TestingGroup",
		}

		tokenB, err := tm.NewWithClaims(claimsB)
		if err != nil {
			t.Fatal(err)
		}

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
		rVolCreateContext = context.WithValue(rVolCreateContext, web.JWTTenantName, "TestingGroup")
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
		rVolDelContext = context.WithValue(rVolDelContext, web.JWTTenantName, "TestingGroup")
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
				w.Write([]byte(`{"result": {"allow": true, "permitted_roles": {"role": 9999999}}}`))
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
		rdb := testCreateRedisInstance(t)
		enf := quota.NewRedisEnforcement(context.Background(), quota.WithRedis(rdb))

		// Create the PowerFlex handler and configure it with a system
		// where the endpoint is our test server.
		powerFlexHandler := proxy.NewPowerFlexHandler(log, enf, hostPort(t, fakeOPA.URL))
		powerFlexHandler.UpdateSystems(context.Background(), strings.NewReader(fmt.Sprintf(`
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
	`, fakePowerFlex.URL)), logrus.New().WithContext(context.Background()))
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
	t.Run("it denies tenant request to map volume that tenant does not own", func(t *testing.T) {
		// Logging.
		log := logrus.New().WithContext(context.Background())
		log.Logger.SetOutput(os.Stdout)

		// Token manager
		tm := jwx.NewTokenManager(jwx.HS256)

		// Prepare tenant A's token
		// Create the claims
		claimsA := token.Claims{
			Issuer:    "com.dell.karavi",
			ExpiresAt: time.Now().Add(30 * time.Second).Unix(),
			Audience:  "karavi",
			Subject:   "Alice",
			Roles:     "DevTesting",
			Group:     "TestingGroup",
		}

		tokenA, err := tm.NewWithClaims(claimsA)
		if err != nil {
			t.Fatal(err)
		}

		// Prepare tenant B's token
		// Create the claims
		claimsB := token.Claims{
			Issuer:    "com.dell.karavi",
			ExpiresAt: time.Now().Add(30 * time.Second).Unix(),
			Audience:  "karavi",
			Subject:   "Bob",
			Roles:     "DevTesting",
			Group:     "TestingGroup2",
		}

		tokenB, err := tm.NewWithClaims(claimsB)
		if err != nil {
			t.Fatal(err)
		}

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
		rVolCreateContext = context.WithValue(rVolCreateContext, web.JWTTenantName, "TestingGroup")
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
		rVolMapContext := context.WithValue(context.Background(), web.JWTKey, tokenB)
		rVolMapContext = context.WithValue(rVolMapContext, web.JWTTenantName, "TestingGroup2")
		rVolMap = rVolMap.WithContext(rVolMapContext)

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
				w.Write([]byte(`{"result": {"allow": true, "permitted_roles": {"role": 9999999}}}`))
			case "/v1/data/karavi/volumes/map":
				w.Write([]byte(`{"result": {"claims": {"standardclaims": {"issuer":"com.dell.karavi","expiresat": "1614813072","Audience": "karavi", "Subject": "Bob"}, "role":  "DevTesting", "group": "TestingGroup2"},"deny": [],"response": {"allowed": true}}}`))
			}
		}))

		// Add headers that the sidecar-proxy would add, in order to identify
		// the request as intended for a PowerFlex with the given systemID.
		rVolCreate.Header.Add("Forwarded", "by=csi-vxflexos")
		rVolCreate.Header.Add("Forwarded", fmt.Sprintf("for=%s;542a2d5f5122210f", fakePowerFlex.URL))
		rVolCreate.Header.Add(proxy.HeaderPVName, createBody.Name)

		rVolMap.Header.Add("Forwarded", "by=csi-vxflexos")
		rVolMap.Header.Add("Forwarded", fmt.Sprintf("for=%s;542a2d5f5122210f", fakePowerFlex.URL))

		// Create the router and assign the appropriate handlers.
		rtr := newTestRouter()
		// Create a redis enforcer
		rdb := testCreateRedisInstance(t)
		enf := quota.NewRedisEnforcement(context.Background(), quota.WithRedis(rdb))

		// Create the PowerFlex handler and configure it with a system
		// where the endpoint is our test server.
		powerFlexHandler := proxy.NewPowerFlexHandler(log, enf, hostPort(t, fakeOPA.URL))
		powerFlexHandler.UpdateSystems(context.Background(), strings.NewReader(fmt.Sprintf(`
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
`, fakePowerFlex.URL)), logrus.New().WithContext(context.Background()))
		systemHandlers := map[string]http.Handler{
			"powerflex": web.Adapt(powerFlexHandler),
		}
		dh := proxy.NewDispatchHandler(log, systemHandlers)
		rtr.ProxyHandler = dh
		h := web.Adapt(rtr.Handler(), web.CleanMW())

		h.ServeHTTP(wVolCreate, rVolCreate)
		h.ServeHTTP(wVolMap, rVolMap)

		if got, want := wVolCreate.Result().StatusCode, http.StatusOK; got != want {
			fmt.Printf("Create request: %v\n", *rVolCreate)
			fmt.Printf("Create response: %v\n", string(wVolCreate.Body.Bytes()))
			t.Errorf("got %v, want %v", got, want)
		}

		if got, want := wVolMap.Result().StatusCode, http.StatusForbidden; got != want {
			fmt.Printf("Map request: %v\n", *rVolMap)
			fmt.Printf("Map response: %v\n", string(wVolMap.Body.Bytes()))
			t.Errorf("got %v, want %v", got, want)
		}

		// This response should come from our PowerFlex handler, NOT the (fake)
		// PowerFlex itself.
		type MapRequestResponse struct {
			ErrorCode      int    `json:"errorCode"`
			HttpStatusCode int    `json:"httpStatusCode"`
			Message        string `json:"message"`
		}
		got := MapRequestResponse{}
		err = json.Unmarshal(wVolMap.Body.Bytes(), &got)
		if err != nil {
			t.Errorf("error demarshalling volume map request response: %v", err)
		}
		want := MapRequestResponse{
			ErrorCode:      403,
			HttpStatusCode: 403,
			Message:        "map denied",
		}
		if !strings.Contains(got.Message, want.Message) || got.ErrorCode != want.ErrorCode || got.HttpStatusCode != want.HttpStatusCode {
			t.Errorf("got %q, expected response body to contain %q", got, want)
		}
	})
	t.Run("it denies tenant request to unmap volume that tenant does not own", func(t *testing.T) {
		// Logging.
		log := logrus.New().WithContext(context.Background())
		log.Logger.SetOutput(os.Stdout)

		// Token manager
		tm := jwx.NewTokenManager(jwx.HS256)

		// Prepare tenant A's token
		// Create the claims
		claimsA := token.Claims{
			Issuer:    "com.dell.karavi",
			ExpiresAt: time.Now().Add(30 * time.Second).Unix(),
			Audience:  "karavi",
			Subject:   "Alice",
			Roles:     "DevTesting",
			Group:     "TestingGroup",
		}

		tokenA, err := tm.NewWithClaims(claimsA)
		if err != nil {
			t.Fatal(err)
		}

		// Prepare tenant B's token
		// Create the claims
		claimsB := token.Claims{
			Issuer:    "com.dell.karavi",
			ExpiresAt: time.Now().Add(30 * time.Second).Unix(),
			Audience:  "karavi",
			Subject:   "Bob",
			Roles:     "DevTesting",
			Group:     "TestingGroup2",
		}

		tokenB, err := tm.NewWithClaims(claimsB)
		if err != nil {
			t.Fatal(err)
		}

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
		rVolCreateContext = context.WithValue(rVolCreateContext, web.JWTTenantName, "TestingGroup")
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
				w.Write([]byte(`{"result": {"allow": true, "permitted_roles": {"role": 9999999}}}`))
			case "/v1/data/karavi/volumes/map":
				w.Write([]byte(`{"result": {"claims": {"standardclaims": {"issuer":"com.dell.karavi","expiresat": "1614813072","Audience": "karavi", "Subject": "Alice"}, "role":  "DevTesting", "group": "TestingGroup"},"deny": [],"response": {"allowed": true}}}`))
			case "/v1/data/karavi/volumes/unmap":
				w.Write([]byte(`{"result": {"claims": {"standardclaims": {"issuer":"com.dell.karavi","expiresat": "1614813072","Audience": "karavi", "Subject": "Bob"}, "role":  "DevTesting", "group": "TestingGroup2"},"deny": [],"response": {"allowed": true}}}`))
			}
		}))

		// Add headers that the sidecar-proxy would add, in order to identify
		// the request as intended for a PowerFlex with the given systemID.
		rVolCreate.Header.Add("Forwarded", "by=csi-vxflexos")
		rVolCreate.Header.Add("Forwarded", fmt.Sprintf("for=%s;542a2d5f5122210f", fakePowerFlex.URL))
		rVolCreate.Header.Add(proxy.HeaderPVName, createBody.Name)

		rVolMap.Header.Add("Forwarded", "by=csi-vxflexos")
		rVolMap.Header.Add("Forwarded", fmt.Sprintf("for=%s;542a2d5f5122210f", fakePowerFlex.URL))

		rVolUnmap.Header.Add("Forwarded", "by=csi-vxflexos")
		rVolUnmap.Header.Add("Forwarded", fmt.Sprintf("for=%s;542a2d5f5122210f", fakePowerFlex.URL))

		// Create the router and assign the appropriate handlers.
		rtr := newTestRouter()
		// Create a redis enforcer
		rdb := testCreateRedisInstance(t)
		enf := quota.NewRedisEnforcement(context.Background(), quota.WithRedis(rdb))

		// Create the PowerFlex handler and configure it with a system
		// where the endpoint is our test server.
		powerFlexHandler := proxy.NewPowerFlexHandler(log, enf, hostPort(t, fakeOPA.URL))
		powerFlexHandler.UpdateSystems(context.Background(), strings.NewReader(fmt.Sprintf(`
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
`, fakePowerFlex.URL)), logrus.New().WithContext(context.Background()))
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
			Message:        "unmap denied",
		}
		if !strings.Contains(got.Message, want.Message) || got.ErrorCode != want.ErrorCode || got.HttpStatusCode != want.HttpStatusCode {
			t.Errorf("got %q, expected response body to contain %q", got, want)
		}
	})

	t.Run("provisioning request against a pool the tenant does not have permission to use", func(t *testing.T) {
		// Logging
		log := logrus.New().WithContext(context.Background())
		log.Logger.SetOutput(os.Stdout)

		// Prepare the create volume payload and request
		body := struct {
			VolumeSize     int64
			VolumeSizeInKb string `json:"volumeSizeInKb"`
			StoragePoolID  string `json:"storagePoolId"`
		}{
			VolumeSize:     10,
			VolumeSizeInKb: "10",
			StoragePoolID:  "3df6b86600000000",
		}
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		payload := bytes.NewBuffer(data)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/types/Volume/instances/", payload)

		// Add a jwt token to the request context
		// In production, the jwt token would have the role information for OPA to make a decision on
		// Since we are faking the OPA server, the jwt token doesn't require real info for the unit test
		reqCtx := context.WithValue(context.Background(), web.JWTKey, token.Token(&jwx.Token{}))
		reqCtx = context.WithValue(reqCtx, web.JWTTenantName, "TestingGroup")
		r = r.WithContext(reqCtx)

		// Build a httptest server to fake OPA
		fakeOPA := buildTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			// This path validates a supported request path, see policies/url.rego
			case "/v1/data/karavi/authz/url":
				w.Write([]byte(`{"result": {"allow": true}}`))
			// This path returns the OPA decision to not allow a create volume request
			case "/v1/data/karavi/volumes/create":
				w.Write([]byte(`{
					"result": {
						"allow": false,
						"deny": ["test not allow reason"]
					}
				}`))
			default:
				t.Fatalf("OPA path %s not supported", r.URL.Path)
			}
		}))

		// Build a httptest TLS server to fake PowerFlex
		fakePowerFlex := buildTestTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/login" {
				w.Write([]byte("token"))
			}
			if r.URL.Path == "/api/version" {
				w.Write([]byte("3.5"))
			}
			if r.URL.Path == "/api/types/StoragePool/instances" {
				data, err := ioutil.ReadFile("testdata/storage_pool_instances.json")
				if err != nil {
					t.Fatal(err)
				}
				w.Write(data)
			}
		}))

		// Add headers that the sidecar-proxy would add, in order to identify
		// the request as intended for a PowerFlex with the given systemID.
		r.Header.Add("Forwarded", "by=csi-vxflexos")
		r.Header.Add("Forwarded", fmt.Sprintf("for=https://%s;542a2d5f5122210f", fakePowerFlex.URL))
		rtr := newTestRouter()

		// Create a PowerFlexHandler and update it with the fake PowerFlex
		powerFlexHandler := proxy.NewPowerFlexHandler(log, nil, hostPort(t, fakeOPA.URL))

		// Cancel the powerflex token getter so we don't get any race conditions with the fakePowerFlex server
		systemCtx, cancel := context.WithCancel(context.Background())
		cancel()

		powerFlexHandler.UpdateSystems(systemCtx, strings.NewReader(fmt.Sprintf(`
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
		`, fakePowerFlex.URL)), logrus.New().WithContext(context.Background()))

		// Create a dispatch handler with the powerFlexHandler
		systemHandlers := map[string]http.Handler{
			"powerflex": web.Adapt(powerFlexHandler),
		}
		dh := proxy.NewDispatchHandler(log, systemHandlers)
		rtr.ProxyHandler = dh
		h := web.Adapt(rtr.Handler(), web.CleanMW())

		// Serve the request
		h.ServeHTTP(w, r)

		errBody := struct {
			Code       int    `json:"errorCode"`
			StatusCode int    `json:"httpStatusCode"`
			Message    string `json:"message"`
		}{}

		err = json.Unmarshal(w.Body.Bytes(), &errBody)
		if err != nil {
			t.Fatal(err)
		}

		want := http.StatusBadRequest
		if got := w.Code; got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	})

	// This test requires the "redis" docker image to be available locally
	t.Run("provisioning request against a pool that exceeds tenant's quota limit", func(t *testing.T) {
		// Logging
		log := logrus.New().WithContext(context.Background())
		log.Logger.SetOutput(os.Stdout)

		// Stand up a docker Redis instance and create a Redis Enforcement for quota validation
		rdb := testCreateRedisInstance(t)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		enf := quota.NewRedisEnforcement(ctx, quota.WithRedis(rdb))

		const tenantQuota = 100

		// Approve requests 0-9 to fill up the quota
		for i := 0; i < 10; i++ {
			r := quota.Request{
				StoragePoolID: "mypool",
				Group:         "mygroup",
				VolumeName:    fmt.Sprintf("k8s-%d", i),
				Capacity:      "10",
			}
			enf.ApproveRequest(ctx, r, tenantQuota)
		}

		// Prepare the create volume payload and request
		body := struct {
			VolumeSize     int64
			VolumeSizeInKb string `json:"volumeSizeInKb"`
			StoragePoolID  string `json:"storagePoolId"`
		}{
			VolumeSize:     10,
			VolumeSizeInKb: "10",
			StoragePoolID:  "3df6b86600000000",
		}
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		payload := bytes.NewBuffer(data)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/types/Volume/instances/", payload)

		// Add a jwt token to the request context
		// In production, the jwt token would have the role information for OPA to make a decision on
		// Since we are faking the OPA server, the jwt token doesn't require real info for the unit test
		reqCtx := context.WithValue(context.Background(), web.JWTKey, token.Token(&jwx.Token{}))
		reqCtx = context.WithValue(reqCtx, web.JWTTenantName, "mygroup")
		r = r.WithContext(reqCtx)

		// Build a httptest server to fake OPA
		fakeOPA := buildTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			// This path validates a supported request path (/api/types/Volume/instances/), see policies/url.rego
			case "/v1/data/karavi/authz/url":
				w.Write([]byte(`{"result": {"allow": true}}`))
			// This path returns the OPA decision to allow a create volume request in the requested storage pool
			// Note: this is not when the quota is validated, that happens with Redis
			case "/v1/data/karavi/volumes/create":
				w.Write([]byte(fmt.Sprintf(`{
					"result": {
						"allow": true,
						"permitted_roles": {
							"role": 1
						}
					}}`)))
			default:
				t.Fatalf("OPA path %s not supported", r.URL.Path)
			}
		}))

		// Build a httptest TLS server to fake PowerFlex
		fakePowerFlex := buildTestTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/login" {
				w.Write([]byte("token"))
			}
			if r.URL.Path == "/api/version" {
				w.Write([]byte("3.5"))
			}
			if r.URL.Path == "/api/types/StoragePool/instances" {
				data, err := ioutil.ReadFile("testdata/storage_pool_instances.json")
				if err != nil {
					t.Fatal(err)
				}
				w.Write(data)
			}
		}))

		// Add headers that the sidecar-proxy would add, in order to identify
		// the request as intended for a PowerFlex with the given systemID.
		r.Header.Add("Forwarded", "by=csi-vxflexos")
		r.Header.Add("Forwarded", fmt.Sprintf("for=https://%s;542a2d5f5122210f", fakePowerFlex.URL))
		rtr := newTestRouter()

		// Create a PowerFlexHandler and update it with the fake PowerFlex
		powerFlexHandler := proxy.NewPowerFlexHandler(log, enf, hostPort(t, fakeOPA.URL))

		// Cancel the powerflex token getter so we don't get any race conditions with the fakePowerFlex server
		systemCtx, cancel := context.WithCancel(context.Background())
		cancel()

		powerFlexHandler.UpdateSystems(systemCtx, strings.NewReader(fmt.Sprintf(`
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
		`, fakePowerFlex.URL)), logrus.New().WithContext(context.Background()))

		// Create a dispatch handler with the powerFlexHandler
		systemHandlers := map[string]http.Handler{
			"powerflex": web.Adapt(powerFlexHandler),
		}
		dh := proxy.NewDispatchHandler(log, systemHandlers)
		rtr.ProxyHandler = dh
		h := web.Adapt(rtr.Handler(), web.CleanMW())

		// Serve the request
		h.ServeHTTP(w, r)

		// Unmarshal the response and check for the exepcted http status code
		errBody := struct {
			Code       int    `json:"errorCode"`
			StatusCode int    `json:"httpStatusCode"`
			Message    string `json:"message"`
		}{}

		err = json.Unmarshal(w.Body.Bytes(), &errBody)
		if err != nil {
			t.Fatal(err)
		}

		if w.Code != http.StatusInsufficientStorage {
			t.Errorf("expected status %d, got %d", http.StatusInsufficientStorage, w.Code)
		}
	})

	// This is the happy path test scenario. A tenent makes a request against a pool within the set quota limit
	t.Run("provisioning request against a pool that is within tenant's quota limit", func(t *testing.T) {
		// Logging
		log := logrus.New().WithContext(context.Background())
		log.Logger.SetOutput(os.Stdout)

		body := struct {
			VolumeSize     int64
			VolumeSizeInKb string `json:"volumeSizeInKb"`
			StoragePoolID  string `json:"storagePoolId"`
		}{
			VolumeSize:     2000,
			VolumeSizeInKb: "2000",
			StoragePoolID:  "3df6df7600000001",
		}
		data, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		payload := bytes.NewBuffer(data)

		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/types/Volume/instances/", payload)

		// Add a jwt token to the request context
		// In production, the jwt token would have the role information for OPA to make a decision on
		// Since we are faking the OPA server, the jwt token doesn't require real info for the unit test
		reqCtx := context.WithValue(context.Background(), web.JWTKey, token.Token(&jwx.Token{}))
		reqCtx = context.WithValue(reqCtx, web.JWTTenantName, "mygroup")
		r = r.WithContext(reqCtx)

		// Build a httptest server to fake OPA
		fakeOPA := buildTestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			// This path validates a supported request path, see policies/url.rego
			case "/v1/data/karavi/authz/url":
				w.Write([]byte(`{"result": {"allow": true}}`))
			// This path returns the OPA decision to allow a create volume request
			case "/v1/data/karavi/volumes/create":
				w.Write([]byte(fmt.Sprintf(`{
					"result": {
						"allow": true,
						"permitted_roles": {
							"role": 2001
						}
				}}`)))
			default:
				t.Fatalf("OPA path %s not supported", r.URL.Path)
			}
		}))

		// Build a httptest TLS server to fake PowerFlex
		fakePowerFlex := buildTestTLSServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/login" {
				w.Write([]byte("token"))
			}
			if r.URL.Path == "/api/version" {
				w.Write([]byte("3.5"))
			}
			if r.URL.Path == "/api/types/StoragePool/instances" {
				data, err := ioutil.ReadFile("testdata/storage_pool_instances.json")
				if err != nil {
					t.Fatal(err)
				}
				w.Write(data)
			}
			if r.URL.Path == "/api/types/Volume/instances/" {
				type volumeCreate struct {
					VolumeSizeInKb string `json:"volumeSizeInKb"`
					StoragePoolID  string `json:"storagePoolId"`
				}
				body, err := ioutil.ReadAll(r.Body)
				if err != nil {
					t.Fatal(err)
				}
				log.Println(string(body))
				var v volumeCreate
				err = json.Unmarshal(body, &v)
				if err != nil {
					t.Fatal(err)
				}
				w.Write([]byte("{\"id\": \"847ce5f30000005a\"}"))
			}
		}))

		// Add headers that the sidecar-proxy would add, in order to identify
		// the request as intended for a PowerFlex with the given systemID.
		r.Header.Add("Forwarded", "by=csi-vxflexos")
		r.Header.Add("Forwarded", fmt.Sprintf("for=https://%s;542a2d5f5122210f", fakePowerFlex.URL))
		rtr := newTestRouter()

		rdb := testCreateRedisInstance(t)
		if rdb == nil {
			t.Fatal("expected non-nil return value for redis client")
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		sut := quota.NewRedisEnforcement(ctx, quota.WithRedis(rdb))
		req := quota.Request{
			StoragePoolID: "3df6df7600000001",
			Group:         "allowed",
			VolumeName:    "k8s-0",
			Capacity:      "1",
		}
		sut.ApproveRequest(ctx, req, 8000)
		t.Run("NewRedisEnforcer", func(t *testing.T) {
			if sut == nil {
				t.Fatal("expected non-nil return value for redis enforcemnt")
			}
		})

		// Create a PowerFlexHandler and update it with the fake PowerFlex
		powerFlexHandler := proxy.NewPowerFlexHandler(log, sut, hostPort(t, fakeOPA.URL))
		powerFlexHandler.UpdateSystems(context.Background(), strings.NewReader(fmt.Sprintf(`
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
			`, fakePowerFlex.URL)), logrus.New().WithContext(context.Background()))

		// Create a dispatch handler with the powerFlexHandler
		systemHandlers := map[string]http.Handler{
			"powerflex": web.Adapt(powerFlexHandler),
		}
		dh := proxy.NewDispatchHandler(log, systemHandlers)
		rtr.ProxyHandler = dh
		h := web.Adapt(rtr.Handler(), web.CleanMW())

		// Serve the request
		h.ServeHTTP(w, r)

		respBody := struct {
			StatusCode int `json:"httpStatusCode"`
			//	Message    string `json:"message"`
		}{}

		err = json.Unmarshal(w.Body.Bytes(), &respBody)
		if err != nil {
			t.Fatal(err)
		}

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
	})
}

func newTestRouter() *web.Router {
	noopHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	return &web.Router{
		ProxyHandler: noopHandler,
		RolesHandler: noopHandler,
		TokenHandler: noopHandler,
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

type tb interface {
	testing.TB
}

func testCreateRedisInstance(t tb) *redis.Client {
	var rdb *redisclient.Client

	redisHost := os.Getenv("REDIS_HOST")
	redistPort := os.Getenv("REDIS_PORT")

	if redisHost != "" && redistPort != "" {
		rdb = redis.NewClient(&redis.Options{
			Addr: fmt.Sprintf("%s:%s", redisHost, redistPort),
		})
	} else {
		var retries int
		for {
			cmd := exec.Command("docker", "run",
				"--rm",
				"--name", "test-redis",
				"--net", "host",
				"--detach",
				"redis")
			b, err := cmd.CombinedOutput()
			if err != nil {
				retries++
				if retries >= 3 {
					t.Fatalf("starting redis in docker: %s, %v", string(b), err)
				}
				time.Sleep(time.Second)
				continue
			}
			break
		}

		t.Cleanup(func() {
			err := exec.Command("docker", "stop", "test-redis").Start()
			if err != nil {
				t.Fatal(err)
			}
		})

		rdb = redis.NewClient(&redis.Options{
			Addr: "localhost:6379",
		})
	}

	// Wait for a PING before returning, or fail with timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		_, err := rdb.Ping().Result()
		if err != nil {
			select {
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			default:
				time.Sleep(time.Nanosecond)
				continue
			}
		}

		break
	}

	return rdb
}
