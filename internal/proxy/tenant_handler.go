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
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyTenantPath, "create"), web.Adapt(web.HandlerWithError(th.createHandler), web.TelemetryMW("csm-authorization-proxy-server", "tenantCreateHandler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyTenantPath, "update"), web.Adapt(web.HandlerWithError(th.updateHandler), web.TelemetryMW("csm-authorization-proxy-server", "tenantUpdateHandler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyTenantPath, "get"), web.Adapt(web.HandlerWithError(th.getHandler), web.TelemetryMW("csm-authorization-proxy-server", "tenantGetHandler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyTenantPath, "delete"), web.Adapt(web.HandlerWithError(th.deleteHandler), web.TelemetryMW("csm-authorization-proxy-server", "tenantDeleteHandler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyTenantPath, "list"), web.Adapt(web.HandlerWithError(th.listHandler), web.TelemetryMW("csm-authorization-proxy-server", "tenantListHandler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyTenantPath, "bind"), web.Adapt(web.HandlerWithError(th.bindRoleHandler), web.TelemetryMW("csm-authorization-proxy-server", "tenantBindRoleHandler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyTenantPath, "unbind"), web.Adapt(web.HandlerWithError(th.unbindRoleHandler), web.TelemetryMW("csm-authorization-proxy-server", "tenantUnbindHandler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyTenantPath, "token"), web.Adapt(web.HandlerWithError(th.generateTokenHandler), web.TelemetryMW("csm-authorization-proxy-server", "tenantGenerateTokenHandler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyTenantPath, "revoke"), web.Adapt(web.HandlerWithError(th.revokeHandler), web.TelemetryMW("csm-authorization-proxy-server", "tenantRevokeHandler", log)))
	th.mux = mux

	return th
}

// ServeHTTP implements the http.Handler interface
func (th *TenantHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	th.mux.ServeHTTP(w, r)
}

type createTenantBody struct {
	Name       string `json:"name"`
	ApproveSdc bool   `json:"approveSdc"`
}

func (th *TenantHandler) createHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// only allow POST requests
	if r.Method != http.MethodPost {
		err := fmt.Errorf("method %s not allowed", r.Method)
		th.log.WithError(err).Error()
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return err
	}

	// read request body
	var body createTenantBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		th.log.WithError(err).Errorf("decoding request body")
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, fmt.Errorf("decoding request body: %v", err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("decoding request body: %v", err)
	}

	span.SetAttributes(attribute.KeyValue{Key: "name", Value: attribute.StringValue(body.Name)},
		attribute.KeyValue{Key: "name", Value: attribute.BoolValue(body.ApproveSdc)})

	th.log.WithFields(logrus.Fields{
		"name":       body.Name,
		"approveSdc": body.ApproveSdc,
	}).Info("Requesting tenant creation")

	// call tenant service
	_, err = th.client.CreateTenant(ctx, &pb.CreateTenantRequest{
		Tenant: &pb.Tenant{
			Name:       body.Name,
			Approvesdc: body.ApproveSdc,
		},
	})
	if err != nil {
		th.log.WithError(err).Errorf("creating tenant: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("creating tenant: %v", err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("creating tenant: %v", err)
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
		th.log.WithError(err).Error()
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return err
	}

	// read request body
	var body createTenantBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		th.log.WithError(err).Errorf("decoding request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, fmt.Errorf("decoding request body: %v", err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("decoding request body: %v", err)
	}

	span.SetAttributes(attribute.KeyValue{Key: "tenant", Value: attribute.StringValue(body.Name)},
		attribute.KeyValue{Key: "approveSdc", Value: attribute.BoolValue(body.ApproveSdc)})

	th.log.WithFields(logrus.Fields{
		"name":       body.Name,
		"approveSdc": body.ApproveSdc,
	}).Info("Requesting tenant update")

	// call tenant service
	_, err = th.client.UpdateTenant(ctx, &pb.UpdateTenantRequest{
		TenantName: body.Name,
		Approvesdc: body.ApproveSdc,
	})
	if err != nil {
		th.log.WithError(err).Errorf("updating tenant: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("updating tenant: %v", err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("updating tenant: %v", err)
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
		th.log.WithError(err).Error()
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return err
	}

	// parse tenant name from request parameters
	params := r.URL.Query()["name"]
	if len(params) == 0 {
		err := fmt.Errorf("tenant name not provided in query parameters")
		th.log.WithError(err).Error()
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, err); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return err
	}

	name := params[0]

	span.SetAttributes(attribute.KeyValue{Key: "tenant", Value: attribute.StringValue(name)})

	th.log.WithFields(logrus.Fields{
		"tenant": name,
	}).Info("Requesting tenant get")

	// call tenant service
	tenant, err := th.client.GetTenant(ctx, &pb.GetTenantRequest{
		Name: name,
	})
	if err != nil {
		th.log.WithError(err).Errorf("getting tenant: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("getting tenant: %v", err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("getting tenant: %v", err)
	}

	// return tenant to client
	_, err = fmt.Fprint(w, protojson.MarshalOptions{Multiline: true, EmitUnpopulated: true, Indent: ""}.Format(tenant))
	if err != nil {
		th.log.WithError(err).Errorf("writing tenant get response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("writing tenant get response: %v", err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("writing tenant get response: %v", err)
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
		th.log.WithError(err).Error()
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return err
	}

	// parse tenant name from request parameters
	params := r.URL.Query()["name"]
	if len(params) == 0 {
		err := fmt.Errorf("tenant name not provided in query parameters")
		th.log.WithError(err).Error()
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, err); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return err
	}

	name := params[0]

	span.SetAttributes(attribute.KeyValue{Key: "tenant", Value: attribute.StringValue(name)})

	th.log.WithFields(logrus.Fields{
		"name": name,
	}).Info("Requesting tenant delete")

	// call tenant service
	_, err := th.client.DeleteTenant(ctx, &pb.DeleteTenantRequest{
		Name: name,
	})
	if err != nil {
		th.log.WithError(err).Errorf("deleting tenant: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("deleting tenant: %v", err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("deleting tenant: %v", err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (th *TenantHandler) listHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// only allow GET requests
	if r.Method != http.MethodGet {
		err := fmt.Errorf("method %s not allowed", r.Method)
		th.log.WithError(err).Error()
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return err
	}

	th.log.Info("Requesting tenant list")

	// call tenant service
	tenants, err := th.client.ListTenant(ctx, &pb.ListTenantRequest{})
	if err != nil {
		th.log.WithError(err).Errorf("listing tenant: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("listing tenant: %v", err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("listing tenant: %v", err)
	}

	// write tenants to client
	err = json.NewEncoder(w).Encode(tenants)
	//_, err = fmt.Fprint(w, protojson.MarshalOptions{Multiline: true, EmitUnpopulated: true, Indent: ""}.Format(tenants))
	if err != nil {
		th.log.WithError(err).Errorf("writing tenant list response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("writing tenant list response: %v", err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("writing tenant list response: %v", err)
	}

	return nil
}

type bindRoleBody struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

func (th *TenantHandler) bindRoleHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// only allow POST requests
	if r.Method != http.MethodPost {
		err := fmt.Errorf("method %s not allowed", r.Method)
		th.log.WithError(err).Error()
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return err
	}

	// read request body
	var body bindRoleBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		th.log.WithError(err).Errorf("decoding request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, fmt.Errorf("decoding request body: %v", err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("decoding request body: %v", err)
	}

	span.SetAttributes(attribute.KeyValue{Key: "tenant", Value: attribute.StringValue(body.Name)},
		attribute.KeyValue{Key: "role", Value: attribute.StringValue(body.Role)})

	th.log.WithFields(logrus.Fields{
		"tenant": body.Name,
		"role":   body.Role,
	})

	// call tenant service
	_, err = th.client.BindRole(ctx, &pb.BindRoleRequest{
		TenantName: body.Name,
		RoleName:   body.Role,
	})
	if err != nil {
		th.log.WithError(err).Errorf("binding %s to %s: %v", body.Role, body.Name, err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("binding %s to %s: %v", body.Role, body.Name, err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("binding %s to %s: %v", body.Role, body.Name, err)
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
		th.log.WithError(err).Error()
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			th.log.WithError(err).Error("error creating json response")
		}
		return err
	}

	// read request body
	var body bindRoleBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		th.log.WithError(err).Errorf("decoding request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, fmt.Errorf("decoding request body: %v", err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("decoding request body: %v", err)
	}

	span.SetAttributes(attribute.KeyValue{Key: "tenant", Value: attribute.StringValue(body.Name)},
		attribute.KeyValue{Key: "role", Value: attribute.StringValue(body.Role)})

	th.log.WithFields(logrus.Fields{
		"tenant": body.Name,
		"role":   body.Role,
	}).Info("Requesting tenant unbind role")

	_, err = th.client.UnbindRole(ctx, &pb.UnbindRoleRequest{
		TenantName: body.Name,
		RoleName:   body.Role,
	})
	if err != nil {
		th.log.WithError(err).Errorf("unbinding %s to %s: %v", body.Role, body.Name, err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("unbinding %s to %s: %v", body.Role, body.Name, err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("unbinding %s to %s: %v", body.Role, body.Name, err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

type generateTokenBody struct {
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
		th.log.WithError(err).Error()
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			th.log.WithError(err).Error("error creating json response")
		}
		return err
	}

	// read request body
	var body generateTokenBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		th.log.WithError(err).Errorf("decoding request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, fmt.Errorf("decoding request body: %v", err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("decoding request body: %v", err)
	}

	// parse token expirations
	accessTokenDuration, err := time.ParseDuration(body.AccessTokenTTL)
	if err != nil {
		th.log.WithError(err).Errorf("parsing access token duration %s: %v", body.AccessTokenTTL, err)
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, fmt.Errorf("parsing access token duration %s: %v", body.AccessTokenTTL, err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("parsing access token duration %s: %v", body.AccessTokenTTL, err)
	}

	refreshTokenDuration, err := time.ParseDuration(body.RefreshTokenTTL)
	if err != nil {
		th.log.WithError(err).Errorf("parsing refresh token duration %s: %v", body.RefreshTokenTTL, err)
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, fmt.Errorf("parsing refresh token duration %s: %v", body.RefreshTokenTTL, err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("parsing refresh token duration %s: %v", body.RefreshTokenTTL, err)
	}

	span.SetAttributes(attribute.KeyValue{Key: "tenant", Value: attribute.StringValue(body.Tenant)},
		attribute.KeyValue{Key: "accessTokenTTL", Value: attribute.StringValue(body.AccessTokenTTL)},
		attribute.KeyValue{Key: "refreshTokenTTL", Value: attribute.StringValue(body.RefreshTokenTTL)})

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
		th.log.WithError(err).Errorf("generating token for %s: %v", body.Tenant, err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("generating token for %s: %v", body.Tenant, err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("generating token for %s: %v", body.Tenant, err)
	}

	// return token to client
	err = json.NewEncoder(w).Encode(token)
	if err != nil {
		th.log.WithError(err).Errorf("writing tenant token response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("writing tenant token response: %v", err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("writing tenant token response: %v", err)
	}

	return nil
}

type tenantRevokeBody struct {
	Tenant string `json:"name"`
	Cancel bool   `json:"cancel"`
}

func (th *TenantHandler) revokeHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// only allow PATCH requests
	if r.Method != http.MethodPatch {
		err := fmt.Errorf("method %s not allowed", r.Method)
		th.log.WithError(err).Error()
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return err
	}

	// read request body
	var body tenantRevokeBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		th.log.WithError(err).Errorf("decoding request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, fmt.Errorf("decoding request body: %v", err)); err != nil {
			th.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("decoding request body: %v", err)
	}

	span.SetAttributes(attribute.KeyValue{Key: "tenant", Value: attribute.StringValue(body.Tenant)},
		attribute.KeyValue{Key: "cancel", Value: attribute.BoolValue(body.Cancel)})

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
			th.log.WithError(err).Errorf("cancelling tenant %s revocation: %v", body.Tenant, err)
			w.WriteHeader(http.StatusInternalServerError)
			if err := web.JSONErrorResponse(w, fmt.Errorf("cancelling tenant %s revocation: %v", body.Tenant, err)); err != nil {
				th.log.WithError(err).Error("creating json response")
			}
			return fmt.Errorf("cancelling tenant %s revocation: %v", body.Tenant, err)
		}
	default:
		_, err = th.client.RevokeTenant(ctx, &pb.RevokeTenantRequest{
			TenantName: body.Tenant,
		})
		if err != nil {
			th.log.WithError(err).Errorf("revoking tenant %s: %v", body.Tenant, err)
			w.WriteHeader(http.StatusInternalServerError)
			if err := web.JSONErrorResponse(w, fmt.Errorf("revoking tenant %s: %v", body.Tenant, err)); err != nil {
				th.log.WithError(err).Error("creating json response")
			}
			return fmt.Errorf("revoking tenant %s: %v", body.Tenant, err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
