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
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"karavi-authorization/internal/web"
	"math/big"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Common constants.
const (
	HeaderAuthz     = "Authorization"
	HeaderForwarded = "Forwarded"
	Bearer          = "Bearer "
	ContentType     = "application/json"
	csiLogLevel     = "CSI_LOG_LEVEL"
	csiLogFormat    = "CSI_LOG_FORMAT"
)

// Hooks that may be overridden for testing.
var (
	jsonMarshal            = json.Marshal
	jsonDecode             = defaultJSONDecode
	urlParse               = url.Parse
	httpPost               = defaultHTTPPost
	insecureProxy          = false
	driverConfigParamsFile *string // Set the location of the driver ConfigMap
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
	if insecureProxy {
		pi.rp.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	} else {
		pool, err := getRootCertificatePool(pi.log)
		if err != nil {
			return err
		}

		pi.rp.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:            pool,
				InsecureSkipVerify: false,
			},
		}
	}

	pi.log.Infof("Listening on %s", listenAddr)
	pi.svr = &http.Server{
		Addr:              listenAddr,
		Handler:           pi.Handler(proxyURL, access, refresh),
		TLSConfig:         pi.TLSConfig,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if err := pi.svr.ListenAndServeTLS("", ""); err != nil {
		fmt.Printf("error: %+v\n", err)
		return err
	}
	return nil
}

// Handler is the ProxyInstance http handler function
func (pi *ProxyInstance) Handler(proxyHost url.URL, access, refresh string) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Override the Authorization header with our Bearer token.
		r.Header.Set(HeaderAuthz, fmt.Sprintf("Bearer %s", access))

		// We must tell the Karavi-Authorization back-end proxy the originally
		// intended endpoint.
		// See https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Forwarded
		r.Host = proxyHost.Host
		r.Header.Add(HeaderForwarded, fmt.Sprintf("for=%s;%s", pi.IntendedEndpoint, pi.SystemID))
		r.Header.Add(HeaderForwarded, fmt.Sprintf("by=%s", pi.PluginID))
		pi.log.WithFields(logrus.Fields{
			"proxy_host": proxyHost.Host,
			"path":       r.URL.Path,
		}).Debug()

		sw := &web.StatusWriter{
			ResponseWriter: w,
		}
		pi.rp.ServeHTTP(sw, r)

		if sw.Status == http.StatusUnauthorized {
			pi.log.Debug("Refreshing tokens!")
			err := refreshTokens(proxyHost, refresh, &access, pi.log)
			if err != nil {
				pi.log.WithError(err).Error("refreshing tokens")
			}
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
	log.Infoln("main: starting sidecar-proxy")

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
	insecureProxyValue, _ := os.LookupEnv("INSECURE")
	if insecureProxyValue == "true" {
		insecureProxy = true
	}

	driverConfigParamsFile = flag.String("driver-config-params", "", "Full path to the YAML file containing the driver ConfigMap")
	flag.Parse()

	driverCfg := viper.New()
	driverCfg.SetConfigFile("/etc/karavi-authorization/driver-config-params.yaml")

	if err := driverCfg.ReadInConfig(); err != nil {
		log.WithError(err).Error("reading config file")
	}

	updateLoggingSettings := func(log *logrus.Entry) {
		logFormat := driverCfg.GetString(csiLogFormat)
		if strings.EqualFold(logFormat, "json") {
			log.Logger.SetFormatter(&logrus.JSONFormatter{})
		} else {
			// use text formatter by default
			log.Logger.SetFormatter(&logrus.TextFormatter{})
		}
		if logFormat != "" {
			log.WithField(csiLogFormat, logFormat).Info("configuration has been set")
		}

		logLevel := driverCfg.GetString(csiLogLevel)
		level, err := logrus.ParseLevel(logLevel)
		if err != nil {
			// use INFO level by default
			level = logrus.InfoLevel
		}
		log.WithField(csiLogLevel, level.String()).Info("configuration has been set")
		log.Logger.SetLevel(level)
		log.WithField(csiLogLevel, level.String()).Info("configuration has been set")
	}
	updateLoggingSettings(log)

	driverCfg.WatchConfig()
	driverCfg.OnConfigChange(func(e fsnotify.Event) {
		log.Infof("Configuration changed! %+v, %s", e.Op, e.Name)
		updateLoggingSettings(log)
	})

	cfgFile, err := os.Open("/etc/karavi-authorization/config/config")
	if err != nil {
		return err
	}
	var configs []SecretData
	err = json.NewDecoder(cfgFile).Decode(&configs)
	if err != nil {
		return err
	}

	// Generate a self-signed certificate for the CSI driver to trust,
	// since we will always be inside the same Pod talking over localhost.
	tlsCert, err := generateX509Certificate()
	if err != nil {
		return err
	}
	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{tlsCert},
		InsecureSkipVerify: true,
	}

	var proxyInstances []*ProxyInstance
	for _, v := range configs {
		fields := map[string]interface{}{
			"endpoint":         v.Endpoint,
			"username":         v.Username,
			"password":         "********",
			"intendedendpoint": v.IntendedEndpoint,
			"isDefault":        v.IsDefault,
			"systemID":         v.SystemID,
			"insecure":         v.Insecure,
		}

		log.WithFields(fields).Infof("main: config: ")

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

func refreshTokens(proxyHost url.URL, refreshToken string, accessToken *string, log *logrus.Entry) error {
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
		log.WithError(err).Error("decoding request body")
		return err
	}

	proxyRefresh, err := urlParse("/proxy/refresh-token")
	if err != nil {
		log.WithError(err).Error("parsing refresh url")
		return err
	}
	httpClient := &http.Client{}
	if insecureProxy {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	} else {
		pool, err := getRootCertificatePool(log)
		if err != nil {
			return err
		}
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:            pool,
				InsecureSkipVerify: false,
			},
		}
	}

	resp, err := httpPost(httpClient, proxyHost.ResolveReference(proxyRefresh).String(), ContentType, bytes.NewReader(reqBytes))
	if err != nil {
		log.WithError(err).Error("making http request")
		return err
	}
	defer resp.Body.Close()

	if sc := resp.StatusCode; sc != http.StatusOK {
		err := fmt.Errorf("status code was %d", sc)
		log.WithError(err).Error()
		return err
	}

	var respBody tokenPair
	if err := jsonDecode(resp.Body, &respBody); err != nil {
		log.WithError(err).Error("decoding response body")
		return fmt.Errorf("decoding proxy response body: %w", err)
	}

	log.Debug("access token was refreshed!")

	*accessToken = respBody.AccessToken
	return nil
}

func defaultHTTPPost(c *http.Client, url, contentType string, body io.Reader) (*http.Response, error) {
	return c.Post(url, contentType, body)
}

func defaultJSONDecode(body io.Reader, v interface{}) error {
	return json.NewDecoder(body).Decode(&v)
}

func generateX509Certificate() (tls.Certificate, error) {
	// Generate the private key.
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("generating private key: %w", err)
	}

	// Use the private key to generate a PEM block.
	keyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	})

	// Generate the certificate.
	serial, err := rand.Int(rand.Reader, big.NewInt(2048))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("getting random number: %w", err)
	}
	tml := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "KaraviAuthorization",
			Organization: []string{"Dell"},
		},
		BasicConstraintsValid: true,
	}
	cert, err := x509.CreateCertificate(rand.Reader, &tml, &tml, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("creating certificate: %w", err)
	}

	// Use the certificate to generate a PEM block.
	certPem := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert,
	})

	// Load the X509 key pair.
	tlsCert, err := tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("loading x509 key pair: %w", err)
	}

	return tlsCert, nil
}

func getRootCertificatePool(log *logrus.Entry) (*x509.CertPool, error) {
	pool := x509.NewCertPool()
	rootCAData, err := ioutil.ReadFile("/etc/karavi-authorization/root-certificates/rootCertificate.pem")
	if err != nil {
		return nil, fmt.Errorf("reading root certificate file: %w", err)
	}

	ok := pool.AppendCertsFromPEM([]byte(rootCAData))
	if !ok {
		log.Infof("unable to add root certificate")
	}
	return pool, nil
}
