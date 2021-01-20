package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

// Common constants.
const (
	HeaderAuthz = "Authorization"
	Bearer      = "Bearer "
	ContentType = "application/json"
)

// Hooks that may be overridden for testing.
var (
	jsonMarshal = json.Marshal
	jsonDecode  = defaultJSONDecode
	urlParse    = url.Parse
	httpPost    = defaultHTTPPost
)

func main() {
	log.Println("Getting listen addr")
	listenAddr, ok := os.LookupEnv("LISTEN_ADDR")
	if !ok {
		listenAddr = ":8443"
	}

	proxyAddr, ok := os.LookupEnv("PROXY_ADDR")
	if !ok {
		log.Fatal("missing proxy addr")
	}
	proxyURL := url.URL{
		Scheme: "https",
		Host:   proxyAddr,
	}

	refresh, ok := os.LookupEnv("REFRESH_TOKEN")
	if !ok {
		log.Fatal("missing refresh token")
	}
	access, ok := os.LookupEnv("ACCESS_TOKEN")
	if !ok {
		log.Fatal("missing access token")
	}

	kp, err := tls.LoadX509KeyPair("/etc/app/cert.pem", "/etc/app/key.pem")
	if err != nil {
		panic(err)
	}

	tlsConfig := &tls.Config{
		Certificates:       []tls.Certificate{kp},
		InsecureSkipVerify: true,
	}
	l, err := tls.Listen("tcp", listenAddr, tlsConfig)
	if err != nil {
		panic(err)
	}

	p := httputil.NewSingleHostReverseProxy(&proxyURL)
	p.Transport = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	log.Printf("Listening on %s", listenAddr)
	log.Fatal(http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", access))
		log.Printf("Path: %s, Headers: %#v", r.URL.Path, r.Header)

		sw := &statusWriter{
			ResponseWriter: w,
		}
		p.ServeHTTP(sw, r)

		if sw.status == http.StatusUnauthorized {
			log.Println("Refreshing tokens!")
			refreshTokens(refresh, &access)
			log.Println(refresh)
			log.Println(access)
		}
	})))
}

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

func refreshTokens(refreshToken string, accessToken *string) error {
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

	base, err := urlParse("https://10.247.98.130")
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
