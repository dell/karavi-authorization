// Copyright © 2021-2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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

package proxy

import (
	"encoding/json"
	"fmt"
	"karavi-authorization/internal/web"
	"karavi-authorization/pb"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/encoding/protojson"
)

// TenantHandler is the proxy handler for karavictl tenant requests
type TenantHandler struct {
	mux    *http.ServeMux
	client pb.TenantServiceClient
	log    *logrus.Entry
}

// NewTenantHandler returns a TenantHandler
func NewTenantHandler(log *logrus.Entry, client pb.TenantServiceClient) *TenantHandler {
	th := &TenantHandler{
		client: client,
		log:    log,
	}

	mux := http.NewServeMux()
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyTenantPath, "create"), web.Adapt(web.HandlerWithError(th.createHandler), web.TelemetryMW("tenant_create_handler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyTenantPath, "update"), web.Adapt(web.HandlerWithError(th.updateHandler), web.TelemetryMW("tenant_update_handler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyTenantPath, "get"), web.Adapt(web.HandlerWithError(th.getHandler), web.TelemetryMW("tenant_get_handler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyTenantPath, "delete"), web.Adapt(web.HandlerWithError(th.deleteHandler), web.TelemetryMW("tenant_delete_handler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyTenantPath, "list"), web.Adapt(web.HandlerWithError(th.listHandler), web.TelemetryMW("tenant_list_handler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyTenantPath, "bind"), web.Adapt(web.HandlerWithError(th.bindRoleHandler), web.TelemetryMW("tenant_bind_role_handler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyTenantPath, "unbind"), web.Adapt(web.HandlerWithError(th.unbindRoleHandler), web.TelemetryMW("tenant_unbind_role_handler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyTenantPath, "token"), web.Adapt(web.HandlerWithError(th.generateTokenHandler), web.TelemetryMW("tenant_generate_token_handler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyTenantPath, "revoke"), web.Adapt(web.HandlerWithError(th.revokeHandler), web.TelemetryMW("tenant_revoke_handler", log)))
	th.mux = mux

	return th
}

// ServeHTTP implements the http.Handler interface
func (th *TenantHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	th.mux.ServeHTTP(w, r)
}

// CreateTenantBody is the request body for tenant creation
type CreateTenantBody struct {
	Tenant     string `json:"tenant"`
	ApproveSdc bool   `json:"approve_sdc"`
}

func (th *TenantHandler) createHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// only allow POST requests
	if r.Method != http.MethodPost {
		err := fmt.Errorf("method %s not allowed", r.Method)
		handleJSONErrorResponse(th.log, w, http.StatusMethodNotAllowed, err)
		return err
	}

	// read request body
	var body CreateTenantBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		err = fmt.Errorf("decoding request body: %w", err)
		handleJSONErrorResponse(th.log, w, http.StatusBadRequest, err)
		return err
	}

	setAttributes(span, map[string]interface{}{
		"tenant":      body.Tenant,
		"approve_sdc": body.ApproveSdc,
	})
	th.log.WithFields(logrus.Fields{
		"tenant":      body.Tenant,
		"approve_sdc": body.ApproveSdc,
	}).Info("Requesting tenant creation")

	// call tenant service
	_, err = th.client.CreateTenant(ctx, &pb.CreateTenantRequest{
		Tenant: &pb.Tenant{
			Name:       body.Tenant,
			Approvesdc: body.ApproveSdc,
		},
	})
	if err != nil {
		err = fmt.Errorf("creating tenant %s: %w", body.Tenant, err)
		handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
		return err
	}

	w.WriteHeader(http.StatusCreated)
	return nil
}

func (th *TenantHandler) updateHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// only allow PATCH requests
	if r.Method != http.MethodPatch {
		err := fmt.Errorf("method %s not allowed", r.Method)
		handleJSONErrorResponse(th.log, w, http.StatusMethodNotAllowed, err)
		return err
	}

	// read request body
	var body CreateTenantBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		err = fmt.Errorf("decoding request body: %w", err)
		handleJSONErrorResponse(th.log, w, http.StatusBadRequest, err)
		return err
	}

	setAttributes(span, map[string]interface{}{
		"tenant":      body.Tenant,
		"approve_sdc": body.ApproveSdc,
	})
	th.log.WithFields(logrus.Fields{
		"tenant":      body.Tenant,
		"approve_sdc": body.ApproveSdc,
	}).Info("Requesting tenant update")

	// call tenant service
	_, err = th.client.UpdateTenant(ctx, &pb.UpdateTenantRequest{
		TenantName: body.Tenant,
		Approvesdc: body.ApproveSdc,
	})
	if err != nil {
		err = fmt.Errorf("updating tenant %s: %w", body.Tenant, err)
		handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (th *TenantHandler) getHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// only allow GET requests
	if r.Method != http.MethodGet {
		err := fmt.Errorf("method %s not allowed", r.Method)
		handleJSONErrorResponse(th.log, w, http.StatusMethodNotAllowed, err)
		return err
	}

	// parse tenant name from request parameters
	params := r.URL.Query()["name"]
	if len(params) == 0 {
		err := fmt.Errorf("tenant name not provided in query parameters")
		handleJSONErrorResponse(th.log, w, http.StatusBadRequest, err)
		return err
	}

	name := params[0]

	setAttributes(span, map[string]interface{}{
		"tenant": name,
	})
	th.log.WithFields(logrus.Fields{
		"tenant": name,
	}).Info("Requesting tenant get")

	// call tenant service
	tenant, err := th.client.GetTenant(ctx, &pb.GetTenantRequest{
		Name: name,
	})
	if err != nil {
		err = fmt.Errorf("getting tenant %s: %w", name, err)
		handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
		return err
	}

	// return tenant to client
	_, err = fmt.Fprint(w, protojson.MarshalOptions{Multiline: true, EmitUnpopulated: true, Indent: ""}.Format(tenant))
	if err != nil {
		err = fmt.Errorf("writing tenant get response: %w", err)
		handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
		return err
	}

	w.WriteHeader(http.StatusOK)
	return nil
}

func (th *TenantHandler) deleteHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// only allow DELETE requests
	if r.Method != http.MethodDelete {
		err := fmt.Errorf("method %s not allowed", r.Method)
		handleJSONErrorResponse(th.log, w, http.StatusMethodNotAllowed, err)
		return err
	}

	// parse tenant name from request parameters
	params := r.URL.Query()["name"]
	if len(params) == 0 {
		err := fmt.Errorf("tenant name not provided in query parameters")
		handleJSONErrorResponse(th.log, w, http.StatusBadRequest, err)
		return err
	}

	name := params[0]

	setAttributes(span, map[string]interface{}{
		"tenant": name,
	})
	th.log.WithFields(logrus.Fields{
		"tenant": name,
	}).Info("Requesting tenant delete")

	// call tenant service
	_, err := th.client.DeleteTenant(ctx, &pb.DeleteTenantRequest{
		Name: name,
	})
	if err != nil {
		err = fmt.Errorf("deleting tenant %s: %w", name, err)
		handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (th *TenantHandler) listHandler(w http.ResponseWriter, r *http.Request) error {
	// only allow GET requests
	if r.Method != http.MethodGet {
		err := fmt.Errorf("method %s not allowed", r.Method)
		handleJSONErrorResponse(th.log, w, http.StatusMethodNotAllowed, err)
		return err
	}

	th.log.Info("Requesting tenant list")

	// call tenant service
	tenants, err := th.client.ListTenant(r.Context(), &pb.ListTenantRequest{})
	if err != nil {
		err = fmt.Errorf("listing tenants: %w", err)
		handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
		return err
	}

	// write tenants to client
	err = json.NewEncoder(w).Encode(&tenants)
	if err != nil {
		err = fmt.Errorf("writing tenant list response: %w", err)
		handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
		return err
	}

	return nil
}

// BindRoleBody  is the request body for binding a tenant to a role
type BindRoleBody struct {
	Tenant string `json:"tenant"`
	Role   string `json:"role"`
}

func (th *TenantHandler) bindRoleHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// only allow POST requests
	if r.Method != http.MethodPost {
		err := fmt.Errorf("method %s not allowed", r.Method)
		handleJSONErrorResponse(th.log, w, http.StatusMethodNotAllowed, err)
		return err
	}

	// read request body
	var body BindRoleBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		err = fmt.Errorf("decoding request body: %w", err)
		handleJSONErrorResponse(th.log, w, http.StatusBadRequest, err)
		return err
	}

	setAttributes(span, map[string]interface{}{
		"tenant": body.Tenant,
		"role":   body.Role,
	})
	th.log.WithFields(logrus.Fields{
		"tenant": body.Tenant,
		"role":   body.Role,
	})

	// call tenant service
	_, err = th.client.BindRole(ctx, &pb.BindRoleRequest{
		TenantName: body.Tenant,
		RoleName:   body.Role,
	})
	if err != nil {
		err = fmt.Errorf("binding tenant %s to %s: %w", body.Tenant, body.Role, err)
		handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
		return err
	}

	w.WriteHeader(http.StatusCreated)
	return nil
}

func (th *TenantHandler) unbindRoleHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// only allow POST requests
	if r.Method != http.MethodPost {
		err := fmt.Errorf("method %s not allowed", r.Method)
		handleJSONErrorResponse(th.log, w, http.StatusMethodNotAllowed, err)
		return err
	}

	// read request body
	var body BindRoleBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		err = fmt.Errorf("decoding request body: %w", err)
		handleJSONErrorResponse(th.log, w, http.StatusBadRequest, err)
		return err
	}

	setAttributes(span, map[string]interface{}{
		"tenant": body.Tenant,
		"role":   body.Role,
	})
	th.log.WithFields(logrus.Fields{
		"tenant": body.Tenant,
		"role":   body.Role,
	}).Info("Requesting tenant unbind role")

	_, err = th.client.UnbindRole(ctx, &pb.UnbindRoleRequest{
		TenantName: body.Tenant,
		RoleName:   body.Role,
	})
	if err != nil {
		err = fmt.Errorf("unbinding tenant %s from %s: %w", body.Tenant, body.Role, err)
		handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

// GenerateTokenBody  is the request body for generating a tenant token
type GenerateTokenBody struct {
	Tenant          string `json:"tenant"`
	AccessTokenTTL  string `json:"accessTokenTTL"`
	RefreshTokenTTL string `json:"refreshTokenTTL"`
}

func (th *TenantHandler) generateTokenHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// only allow POST requests
	if r.Method != http.MethodPost {
		err := fmt.Errorf("method %s not allowed", r.Method)
		handleJSONErrorResponse(th.log, w, http.StatusMethodNotAllowed, err)
		return err
	}

	// read request body
	var body GenerateTokenBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		err = fmt.Errorf("decoding request body: %w", err)
		handleJSONErrorResponse(th.log, w, http.StatusBadRequest, err)
		return err
	}

	// parse token expirations
	accessTokenDuration, err := time.ParseDuration(body.AccessTokenTTL)
	if err != nil {
		err = fmt.Errorf("parsing access token duration %s: %w", body.AccessTokenTTL, err)
		handleJSONErrorResponse(th.log, w, http.StatusBadRequest, err)
		return err
	}

	refreshTokenDuration, err := time.ParseDuration(body.RefreshTokenTTL)
	if err != nil {
		err = fmt.Errorf("parsing refresh token duration %s: %w", body.RefreshTokenTTL, err)
		handleJSONErrorResponse(th.log, w, http.StatusBadRequest, err)
		return err
	}

	setAttributes(span, map[string]interface{}{
		"tenant":            body.Tenant,
		"access_token_TTL":  body.AccessTokenTTL,
		"refresh_token_TTL": body.RefreshTokenTTL,
	})
	th.log.WithFields(logrus.Fields{
		"tenant":          body.Tenant,
		"accessTokenTTL":  body.AccessTokenTTL,
		"refreshTokenTTL": body.RefreshTokenTTL,
	}).Info("Requesting token generation")

	// call tenant service
	token, err := th.client.GenerateToken(ctx, &pb.GenerateTokenRequest{
		TenantName:      body.Tenant,
		AccessTokenTTL:  int64(accessTokenDuration),
		RefreshTokenTTL: int64(refreshTokenDuration),
	})
	if err != nil {
		err = fmt.Errorf("generating token for %s: %w", body.Tenant, err)
		handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
		return err
	}

	// return token to client
	err = json.NewEncoder(w).Encode(token)
	if err != nil {
		err = fmt.Errorf("writing tenant token response: %w", err)
		handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
		return err
	}

	return nil
}

// TenantRevokeBody  is the request body for updating a tenant's revocation status
type TenantRevokeBody struct {
	Tenant string `json:"tenant"`
	Cancel bool   `json:"cancel"`
}

func (th *TenantHandler) revokeHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// only allow PATCH requests
	if r.Method != http.MethodPatch {
		err := fmt.Errorf("method %s not allowed", r.Method)
		handleJSONErrorResponse(th.log, w, http.StatusMethodNotAllowed, err)
		return err
	}

	// read request body
	var body TenantRevokeBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		err = fmt.Errorf("decoding request body: %w", err)
		handleJSONErrorResponse(th.log, w, http.StatusBadRequest, err)
		return err
	}

	setAttributes(span, map[string]interface{}{
		"tenant": body.Tenant,
		"cancel": body.Cancel,
	})
	th.log.WithFields(
		logrus.Fields{
			"tenant": body.Tenant,
			"cancel": body.Cancel,
		},
	).Info("Requesting tenant revoke")

	// call tenant service
	switch {
	case body.Cancel:
		_, err = th.client.CancelRevokeTenant(ctx, &pb.CancelRevokeTenantRequest{
			TenantName: body.Tenant,
		})
		if err != nil {
			err = fmt.Errorf("cancelling tenant %s revocation: %w", body.Tenant, err)
			handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
			return err
		}
	default:
		_, err = th.client.RevokeTenant(ctx, &pb.RevokeTenantRequest{
			TenantName: body.Tenant,
		})
		if err != nil {
			err = fmt.Errorf("revoking tenant %s: %w", body.Tenant, err)
			handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
			return err
		}
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func setAttributes(span trace.Span, data map[string]interface{}) {
	var attr []attribute.KeyValue
	for k, v := range data {
		switch d := v.(type) {
		case string:
			attr = append(attr, attribute.KeyValue{Key: attribute.Key(k), Value: attribute.StringValue(d)})
		case bool:
			attr = append(attr, attribute.KeyValue{Key: attribute.Key(k), Value: attribute.BoolValue(d)})
		}
	}
	span.SetAttributes(attr...)
}
