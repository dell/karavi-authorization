// Copyright © 2021 - 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//      http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"karavi-authorization/internal/web"
	"net/http"
	"net/url"
)

const (
	// HeaderKeyContentType is key for Content-Type
	HeaderKeyContentType = "Content-Type"
	// HeaderValContentTypeJSON is key for application/json
	HeaderValContentTypeJSON = "application/json"
	// headerValContentTypeBinaryOctetStream is key for binary/octet-stream
	headerValContentTypeBinaryOctetStream = "binary/octet-stream"
)

// Client is an API client.
type Client interface {
	// Get sends an HTTP request using the GET method to the proxy server.
	Get(
		ctx context.Context,
		path string,
		headers map[string]string,
		query url.Values,
		resp interface{}) error

	// Post sends an HTTP request using the POST method to the proxy server.
	Post(
		ctx context.Context,
		path string,
		headers map[string]string,
		query url.Values,
		body, resp interface{}) error

	// Put sends an HTTP request using the PATCH method to the proxy server.
	Patch(
		ctx context.Context,
		path string,
		headers map[string]string,
		query url.Values,
		body, resp interface{}) error

	// Delete sends an HTTP request using the DELETE method to the proxy server.
	Delete(
		ctx context.Context,
		path string,
		headers map[string]string,
		query url.Values,
		resp interface{}) error
}

type client struct {
	http *http.Client
	host string
}

// ClientOptions are options for the API client.
type ClientOptions struct {
	// Insecure is a flag that indicates whether or not to validate certificates.
	Insecure bool

	// HttpClient specifies a custom http client for this client.
	HttpClient *http.Client
}

// New returns a new API client.
func New(
	ctx context.Context,
	host string,
	opts ClientOptions) (Client, error) {

	if host == "" {
		return nil, fmt.Errorf("host must not be empty")
	}

	httpClient := http.DefaultClient
	if opts.HttpClient != nil {
		httpClient = opts.HttpClient
	}
	c := &client{
		http: httpClient,
		host: host,
	}

	if opts.Insecure {
		c.http.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}
	} else {
		pool, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}

		c.http.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs:            pool,
				InsecureSkipVerify: false,
			},
		}
	}

	return c, nil
}

// Get executes a GET request
func (c *client) Get(
	ctx context.Context,
	path string,
	headers map[string]string,
	query url.Values,
	resp interface{}) error {

	return c.DoWithHeaders(
		ctx, http.MethodGet, path, headers, query, nil, resp)
}

// Post executes a POST request
func (c *client) Post(
	ctx context.Context,
	path string,
	headers map[string]string,
	query url.Values,
	body, resp interface{}) error {

	return c.DoWithHeaders(
		ctx, http.MethodPost, path, headers, query, body, resp)
}

// Patch executes a PATCH request
func (c *client) Patch(
	ctx context.Context,
	path string,
	headers map[string]string,
	query url.Values,
	body, resp interface{}) error {

	return c.DoWithHeaders(
		ctx, http.MethodPatch, path, headers, query, body, resp)
}

// Delete executes a DELETE request
func (c *client) Delete(
	ctx context.Context,
	path string,
	headers map[string]string,
	query url.Values,
	resp interface{}) error {

	return c.DoWithHeaders(
		ctx, http.MethodDelete, path, headers, query, nil, resp)
}

func beginsWithSlash(s string) bool {
	return s[0] == '/'
}

func endsWithSlash(s string) bool {
	return s[len(s)-1] == '/'
}

// DoWithHeaders executes the request with the supplied headers
func (c *client) DoWithHeaders(
	ctx context.Context,
	method, uri string,
	headers map[string]string,
	query url.Values,
	body, resp interface{}) error {

	res, err := c.DoAndGetResponseBody(
		ctx, method, uri, headers, query, body)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// parse the response
	switch {
	case res == nil:
		return fmt.Errorf("no response")
	case res.StatusCode >= 200 && res.StatusCode <= 299:
		if resp == nil {
			return fmt.Errorf("no response")
		}

		err := json.NewDecoder(res.Body).Decode(resp)
		if err != nil {
			return err
		}
	default:
		return c.ParseJSONError(res)
	}
	return nil
}

// DoAndGetResponseBody executes the request and returns the response body
func (c *client) DoAndGetResponseBody(
	ctx context.Context,
	method, uri string,
	headers map[string]string,
	query url.Values,
	body interface{}) (*http.Response, error) {

	var (
		err                error
		req                *http.Request
		res                *http.Response
		ubf                = &bytes.Buffer{}
		luri               = len(uri)
		hostEndsWithSlash  = endsWithSlash(c.host)
		uriBeginsWithSlash = beginsWithSlash(uri)
	)

	ubf.WriteString(c.host)

	if !hostEndsWithSlash && (luri > 0) {
		ubf.WriteString("/")
	}

	if luri > 0 {
		if uriBeginsWithSlash {
			ubf.WriteString(uri[1:])
		} else {
			ubf.WriteString(uri)
		}
	}

	u, err := url.Parse(ubf.String())
	if err != nil {
		return nil, err
	}

	var isContentTypeSet bool

	// marshal the message body (assumes json format)
	if r, ok := body.(io.ReadCloser); ok {
		req, err = http.NewRequestWithContext(ctx, method, u.String(), r)
		defer r.Close()

		if v, ok := headers[HeaderKeyContentType]; ok {
			req.Header.Set(HeaderKeyContentType, v)
		} else {
			req.Header.Set(
				HeaderKeyContentType, headerValContentTypeBinaryOctetStream)
		}
		isContentTypeSet = true
	} else if body != nil {
		buf := &bytes.Buffer{}
		enc := json.NewEncoder(buf)
		if err = enc.Encode(body); err != nil {
			return nil, err
		}
		req, err = http.NewRequest(method, u.String(), buf)
		if v, ok := headers[HeaderKeyContentType]; ok {
			req.Header.Set(HeaderKeyContentType, v)
		} else {
			req.Header.Set(HeaderKeyContentType, HeaderValContentTypeJSON)
		}
		isContentTypeSet = true
	} else {
		req, err = http.NewRequest(method, u.String(), nil)
	}

	if err != nil {
		return nil, err
	}

	if !isContentTypeSet {
		isContentTypeSet = req.Header.Get(HeaderKeyContentType) != ""
	}

	// add headers to the request
	for header, value := range headers {
		if header == HeaderKeyContentType && isContentTypeSet {
			continue
		}
		req.Header.Add(header, value)
	}

	// add query values to the request
	if query != nil {
		req.URL.RawQuery = query.Encode()
	}

	// send the request
	req = req.WithContext(ctx)
	if res, err = c.http.Do(req); err != nil {
		return nil, err
	}

	return res, err
}

// ParseJSONError parses the error from the proxy server
func (c *client) ParseJSONError(r *http.Response) error {
	jsonError := web.JSONError{}
	if err := json.NewDecoder(r.Body).Decode(&jsonError); err != nil {
		return err
	}
	return jsonError
}
