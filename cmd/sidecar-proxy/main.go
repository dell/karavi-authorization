// Copyright © 2021 Dell Inc., or its subsidiaries. All Rights Reserved.
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

package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
)

// Common constants.
const (
	HeaderAuthz     = "Authorization"
	HeaderForwarded = "Forwarded"
	Bearer          = "Bearer "
	ContentType     = "application/json"
)

// Hooks that may be overridden for testing.
var (
	jsonMarshal = json.Marshal
	jsonDecode  = defaultJSONDecode
	urlParse    = url.Parse
	httpPost    = defaultHTTPPost
)

// SecretData holds k8s secret data for a backend storage system
type SecretData struct {
	Username         string `json:"username"`
	Password         string `json:"password"`
	IntendedEndpoint string `json:"intendedEndpoint"`
	Endpoint         string `json:"endpoint"`
	SystemID         string `json:"systemID"`
	Insecure         bool   `json:"insecure"`
	IsDefault        bool   `json:"isDefault"`
}

// ProxyInstance is an instance of a proxy server to a backend storage system
type ProxyInstance struct {
	PluginID         string
	Endpoint         string
	IntendedEndpoint string
	SystemID         string
	TLSConfig        *tls.Config
	log              *logrus.Entry
	l                net.Listener
	rp               *httputil.ReverseProxy
	svr              *http.Server
}

// Start serves a ProxyInstance http server
func (pi *ProxyInstance) Start(proxyHost, access, refresh string) error {
	var err error

	ep, err := url.Parse(pi.Endpoint)
	if err != nil {
		return err
	}

	_, port, err := net.SplitHostPort(ep.Host)
	if err != nil {
		return err
	}

	listenAddr := fmt.Sprintf(":%v", port)
	proxyURL := url.URL{
		Scheme: "https",
		Host:   proxyHost,
	}
	pi.rp = httputil.NewSingleHostReverseProxy(&proxyURL)
	pi.rp.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	pi.log.Printf("Listening on %s", listenAddr)
	pi.svr = &http.Server{
		Addr:      listenAddr,
		Handler:   pi.Handler(proxyURL.String(), access, refresh),
		TLSConfig: pi.TLSConfig,
	}

	if err := pi.svr.ListenAndServeTLS("", ""); err != nil {
		fmt.Printf("error: %+v\n", err)
		return err
	}
	return nil
}

// Handler is the ProxyInstance http handler function
func (pi *ProxyInstance) Handler(proxyHost, access, refresh string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Override the Authorization header with our Bearer token.
		r.Header.Set(HeaderAuthz, fmt.Sprintf("Bearer %s", access))

		// We must tell the Karavi-Authorization back-end proxy the originally
		// intended endpoint.
		// See https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Forwarded
		r.Header.Add(HeaderForwarded, fmt.Sprintf("for=%s;%s", pi.IntendedEndpoint, pi.SystemID))
		r.Header.Add(HeaderForwarded, fmt.Sprintf("by=%s", pi.PluginID))
		log.Printf("Path: %s, Headers: %#v", r.URL.Path, r.Header)

		sw := &statusWriter{
			ResponseWriter: w,
		}
		pi.rp.ServeHTTP(sw, r)

		if sw.status == http.StatusUnauthorized {
			log.Println("Refreshing tokens!")
			refreshTokens(proxyHost, refresh, &access)
			log.Println(refresh)
			log.Println(access)
		}
	})
}

// Stop closes the ProxyInstance http server
func (pi *ProxyInstance) Stop() error {
	return pi.svr.Close()
}

func main() {
	log := logrus.New().WithContext(context.Background())
	if err := run(log); err != nil {
		log.Errorf("main: %+v", err)
		os.Exit(1)
	}
}

func run(log *logrus.Entry) error {
	log.Println("main: starting sidecar-proxy")

	proxyHost, ok := os.LookupEnv("PROXY_HOST")
	if !ok {
		return errors.New("missing proxy host")
	}
	pluginID, ok := os.LookupEnv("PLUGIN_IDENTIFIER")
	if !ok {
		return errors.New("missing proxy host")
	}
	refresh, ok := os.LookupEnv("REFRESH_TOKEN")
	if !ok {
		return errors.New("missing refresh token")
	}
	access, ok := os.LookupEnv("ACCESS_TOKEN")
	if !ok {
		return errors.New("missing access token")
	}

	cfgFile, err := os.Open("/etc/karavi-authorization/config/config")
	if err != nil {
		return err
	}
	var configs []SecretData
	err = json.NewDecoder(cfgFile).Decode(&configs)
	if err != nil {
		return err
	}
	log.Printf("main: config: %+v\n", configs)

	kp, err := tls.LoadX509KeyPair("/etc/karavi-authorization/tls/cert.pem", "/etc/karavi-authorization/tls/key.pem")
	if err != nil {
		return err
	}
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{kp},
		InsecureSkipVerify: true,
	}

	var proxyInstances []*ProxyInstance
	for _, v := range configs {
		pi := &ProxyInstance{
			log:              log,
			PluginID:         pluginID,
			Endpoint:         v.Endpoint,
			IntendedEndpoint: v.IntendedEndpoint,
			SystemID:         v.SystemID,
			TLSConfig:        tlsConfig,
		}
		proxyInstances = append(proxyInstances, pi)
	}
	var wg sync.WaitGroup
	for _, v := range proxyInstances {
		wg.Add(1)
		go func(pi *ProxyInstance) {
			defer wg.Done()
			defer pi.Stop()
			err := pi.Start(proxyHost, access, refresh)
			if err != nil {
				fmt.Printf("error: %+v\n", err)
				return
			}
		}(v)
	}
	// TODO(ian): Deal with context cancellation and graceful shutdown.
	wg.Wait()

	return nil
}

// TODO(ian): This should be a common struct in internal/web.
type statusWriter struct {
	http.ResponseWriter
	status int
	length int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.length += n
	return n, err
}

func refreshTokens(proxyHost, refreshToken string, accessToken *string) error {
	type tokenPair struct {
		RefreshToken string `json:"refreshToken"`
		AccessToken  string `json:"accessToken"`
	}
	reqBody := tokenPair{
		RefreshToken: refreshToken,
		AccessToken:  *accessToken,
	}

	reqBytes, err := jsonMarshal(&reqBody)
	if err != nil {
		log.Printf("%+v", err)
		return err
	}

	base, err := urlParse(proxyHost)
	if err != nil {
		log.Printf("%+v", err)
		return err
	}
	proxyRefresh, err := urlParse("/proxy/refresh-token")
	if err != nil {
		log.Printf("%+v", err)
		return err
	}

	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	resp, err := httpPost(httpClient, base.ResolveReference(proxyRefresh).String(), ContentType, bytes.NewReader(reqBytes))
	if err != nil {
		log.Printf("%+v", err)
		return err
	}
	defer resp.Body.Close()

	if sc := resp.StatusCode; sc != http.StatusOK {
		err := fmt.Errorf("status code was %d", sc)
		log.Printf("%+v", err)
		return err
	}

	var respBody tokenPair
	if err := jsonDecode(resp.Body, &respBody); err != nil {
		log.Printf("%+v", err)
		return fmt.Errorf("decoding proxy response body: %w", err)
	}

	log.Println("access token was refreshed!")

	*accessToken = respBody.AccessToken
	return nil
}

func defaultHTTPPost(c *http.Client, url, contentType string, body io.Reader) (*http.Response, error) {
	return c.Post(url, contentType, body)
}

func defaultJSONDecode(body io.Reader, v interface{}) error {
	return json.NewDecoder(body).Decode(&v)
}
