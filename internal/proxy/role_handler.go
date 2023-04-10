// Copyright © 2023 Dell Inc. or its subsidiaries. All Rights Reserved.
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

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/encoding/protojson"
)

// RolesHandler is the proxy handler for karavictl role requests
type RoleHandler struct {
	mux    *http.ServeMux
	client pb.RoleServiceClient
	log    *logrus.Entry
}

// NewRoleHandler returns a RoleHandler
func NewRoleHandler(log *logrus.Entry, client pb.RoleServiceClient) *RoleHandler {
	th := &RoleHandler{
		client: client,
		log:    log,
	}

	mux := http.NewServeMux()
	mux.Handle(web.ProxyRolesPath, web.Adapt(web.HandlerWithError(th.roleHandler), web.TelemetryMW("role_handler", log)))
	th.mux = mux

	return th
}

// ServeHTTP implements the http.Handler interface
func (th *RoleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	th.mux.ServeHTTP(w, r)
}

func (th *RoleHandler) roleHandler(w http.ResponseWriter, r *http.Request) error {
	switch r.Method {
	case http.MethodPost:
		return th.createHandler(w, r)
	case http.MethodPatch:
		return th.updateHandler(w, r)
	case http.MethodGet:
		return th.getHandler(w, r)
	case http.MethodDelete:
		return th.deleteHandler(w, r)
	default:
		return nil
	}
}

// CreateRoleBody is the request body for tenant creation
type CreateRoleBody struct {
	Name        string `json:"name,omitempty"`
	StorageType string `json:"storageType,omitempty"`
	SystemId    string `json:"systemId,omitempty"`
	Pool        string `json:"pool,omitempty"`
	Quota       string `json:"quota,omitempty"`
}

func (th *RoleHandler) createHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// read request body
	var body CreateRoleBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		err = fmt.Errorf("decoding request body: %w", err)
		handleJSONErrorResponse(th.log, w, http.StatusBadRequest, err)
		return err
	}

	setAttributes(span, map[string]interface{}{
		"name":        body.Name,
		"storageType": body.StorageType,
		"systemId":    body.SystemId,
		"pool":        body.Pool,
		"quota":       body.Quota,
	})
	th.log.WithFields(logrus.Fields{
		"name":        body.Name,
		"storageType": body.StorageType,
		"systemId":    body.SystemId,
		"pool":        body.Pool,
		"quota":       body.Quota,
	}).Info("Requesting role creation")

	// call role service
	_, err = th.client.Create(ctx, &pb.RoleCreateRequest{
		Name:        body.Name,
		StorageType: body.StorageType,
		SystemId:    body.SystemId,
		Pool:        body.Pool,
		Quota:       body.Quota,
	})
	if err != nil {
		err = fmt.Errorf("creating role %s: %w", body, err)
		handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
		return err
	}
	w.WriteHeader(http.StatusCreated)
	return nil
}

func (th *RoleHandler) updateHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// read request body
	var body CreateRoleBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		err = fmt.Errorf("decoding request body: %w", err)
		handleJSONErrorResponse(th.log, w, http.StatusBadRequest, err)
		return err
	}

	setAttributes(span, map[string]interface{}{
		"name":        body.Name,
		"storageType": body.StorageType,
		"systemId":    body.SystemId,
		"pool":        body.Pool,
		"quota":       body.Quota,
	})
	th.log.WithFields(logrus.Fields{
		"name":        body.Name,
		"storageType": body.StorageType,
		"systemId":    body.SystemId,
		"pool":        body.Pool,
		"quota":       body.Quota,
	}).Info("Requesting role update")

	_, err = th.client.Update(ctx, &pb.RoleUpdateRequest{
		Name:        body.Name,
		StorageType: body.StorageType,
		SystemId:    body.SystemId,
		Pool:        body.Pool,
		Quota:       body.Quota,
	})

	if err != nil {
		err = fmt.Errorf("updating role %s: %w", body, err)
		handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (th *RoleHandler) getHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// parse role name from request parameters
	params := r.URL.Query()["name"]

	// not querying one role but list all
	if len(params) == 0 || params[0] == "" {
		th.log.Info("Requesting role list")

		// call role service
		roles, err := th.client.List(ctx, &pb.RoleListRequest{})
		if err != nil {
			err = fmt.Errorf("listing roles: %w", err)
			handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
			return err
		}

		// write roles to client
		err = json.NewEncoder(w).Encode(&roles)
		if err != nil {
			err = fmt.Errorf("writing role list response: %w", err)
			handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
			return err
		}
		return nil
	}
	// else, call role service to get one specific role
	name := params[0]

	setAttributes(span, map[string]interface{}{
		"name": name,
	})
	th.log.WithFields(logrus.Fields{
		"name": name,
	}).Info("Requesting role get")

	// call role service
	role, err := th.client.Get(ctx, &pb.RoleGetRequest{
		Name: name,
	})
	if err != nil {
		err = fmt.Errorf("getting role %s: %w", name, err)
		handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
		return err
	}

	// return role to client
	_, err = fmt.Fprint(w, protojson.MarshalOptions{Multiline: true, EmitUnpopulated: true, Indent: ""}.Format(role))
	if err != nil {
		err = fmt.Errorf("writing role get response: %w", err)
		handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
		return err
	}
	w.WriteHeader(http.StatusOK)
	return nil
}

func (th *RoleHandler) deleteHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)
	// read request body
	var body CreateRoleBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		err = fmt.Errorf("decoding request body: %w", err)
		handleJSONErrorResponse(th.log, w, http.StatusBadRequest, err)
		return err
	}

	setAttributes(span, map[string]interface{}{
		"name":        body.Name,
		"storageType": body.StorageType,
		"systemId":    body.SystemId,
		"pool":        body.Pool,
		"quota":       body.Quota,
	})
	th.log.WithFields(logrus.Fields{
		"name":        body.Name,
		"storageType": body.StorageType,
		"systemId":    body.SystemId,
		"pool":        body.Pool,
		"quota":       body.Quota,
	}).Info("Requesting role deletion")

	_, err = th.client.Delete(ctx, &pb.RoleDeleteRequest{
		Name:        body.Name,
		StorageType: body.StorageType,
		SystemId:    body.SystemId,
		Pool:        body.Pool,
		Quota:       body.Quota,
	})

	if err != nil {
		err = fmt.Errorf("deleting role %s: %w", body, err)
		handleJSONErrorResponse(th.log, w, http.StatusInternalServerError, err)
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
