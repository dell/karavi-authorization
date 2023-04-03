// Copyright ©2023 Dell Inc. or its subsidiaries. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance wishthe License.
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

// Storage Handler is the proxy handler for karavictl storage requests
type StorageHandler struct {
	mux    *http.ServeMux
	client pb.StorageServiceClient
	log    *logrus.Entry
}

type createStorageBody struct {
	StorageType string `json:"StorageType"`
	Endpoint    string `json:"Endpoint"`
	SystemId    string `json:"SystemId"`
	UserName    string `json:"Username"`
	Password    string `json:"Password"`
	Insecure    bool   `json:"Insecure"`
}

// NewStorageHandler returns a StorageHandler
func NewStorageHandler(log *logrus.Entry, client pb.StorageServiceClient) *StorageHandler {
	sh := &StorageHandler{
		client: client,
		log:    log,
	}

	mux := http.NewServeMux()
	mux.Handle(web.ProxyStoragePath, web.Adapt(web.HandlerWithError(sh.storageHandler), web.TelemetryMW("storageHandler", log)))
	sh.mux = mux

	return sh
}

func (th *StorageHandler) storageHandler(w http.ResponseWriter, r *http.Request) error {
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

// ServeHTTP implements the http.Handler interface
func (sh *StorageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sh.mux.ServeHTTP(w, r)
}

func (sh *StorageHandler) createHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// read request body
	var body createStorageBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		handleJSONErrorResponse(sh.log, w, http.StatusBadRequest, err)
	}

	setAttributes(span, map[string]interface{}{
		"StorageType": body.StorageType,
		"Endpoint":    body.Endpoint,
		"SystemId":    body.SystemId,
		"UserName":    body.UserName,
		"Password":    body.Password,
		"Insecure":    body.Insecure,
	})

	sh.log.WithFields(logrus.Fields{
		"StorageType": body.StorageType,
		"Endpoint":    body.Endpoint,
		"SystemId":    body.SystemId,
		"UserName":    body.UserName,
		"Password":    body.Password,
		"Insecure":    body.Insecure,
	}).Info("Requesting storage creation")

	// call storage service
	_, err = sh.client.Create(ctx, &pb.StorageCreateRequest{
		StorageType: body.StorageType,
		Endpoint:    body.Endpoint,
		SystemId:    body.SystemId,
		UserName:    body.UserName,
		Password:    body.Password,
		Insecure:    body.Insecure,
	})
	if err != nil {
		sh.log.WithError(err).Errorf("creating storage: %v", err)
		handleJSONErrorResponse(sh.log, w, http.StatusInternalServerError, err)
		return err
	}

	w.WriteHeader(http.StatusCreated)
	return nil
}

func (sh *StorageHandler) updateHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// read request body
	var body createStorageBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		sh.log.WithError(err).Errorf("decoding request body: %v", err)
		handleJSONErrorResponse(sh.log, w, http.StatusBadRequest, err)
		return err
	}

	setAttributes(span, map[string]interface{}{
		"StorageType": body.StorageType,
		"Endpoint":    body.Endpoint,
		"SystemId":    body.SystemId,
		"UserName":    body.UserName,
		"Password":    body.Password,
		"Insecure":    body.Insecure,
	})

	sh.log.WithFields(logrus.Fields{
		"StorageType": body.StorageType,
		"Endpoint":    body.Endpoint,
		"SystemId":    body.SystemId,
		"UserName":    body.UserName,
		"Password":    body.Password,
		"Insecure":    body.Insecure,
	}).Info("Requesting storage update")

	// call storage service
	_, err = sh.client.Update(ctx, &pb.StorageUpdateRequest{
		StorageType: body.StorageType,
		Endpoint:    body.Endpoint,
		SystemId:    body.SystemId,
		UserName:    body.UserName,
		Password:    body.Password,
		Insecure:    body.Insecure,
	})
	if err != nil {
		sh.log.WithError(err).Errorf("updating storage: %v", err)
		handleJSONErrorResponse(sh.log, w, http.StatusInternalServerError, err)
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (sh *StorageHandler) getHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// parse storagetype from request parameters
	params := r.URL.Query()["StorageType"]
	if len(params) == 0 || params[0] == "" {
		sh.log.Info("Requesting storage list")

		// call storage service
		storages, err := sh.client.List(ctx, &pb.StorageListRequest{})
		if err != nil {
			err = fmt.Errorf("listing storages: %w", err)
			handleJSONErrorResponse(sh.log, w, http.StatusInternalServerError, err)
			return err
		}

		// write storage to client
		err = json.NewEncoder(w).Encode(&storages)
		if err != nil {
			err = fmt.Errorf("writing storage list response: %w", err)
			handleJSONErrorResponse(sh.log, w, http.StatusInternalServerError, err)
			return err
		}
		return nil
	}

	storType := params[0]

	// parse storage systemid from request parameters
	params = r.URL.Query()["SystemId"]
	if len(params) == 0 {
		err := fmt.Errorf("storage systemid not provided in query parameters")
		sh.log.WithError(err).Error()
		handleJSONErrorResponse(sh.log, w, http.StatusBadRequest, err)
		return err
	}

	sysID := params[0]

	setAttributes(span, map[string]interface{}{
		"storageType": storType,
		"systemID":    sysID,
	})

	sh.log.WithFields(logrus.Fields{
		"storageType": storType,
		"systemID":    sysID,
	}).Info("Requesting storage get")

	// call storage service
	storage, err := sh.client.Get(ctx, &pb.StorageGetRequest{
		StorageType: storType,
		SystemId:    sysID,
	})
	if err != nil {
		sh.log.WithError(err).Errorf("getting storage: %v", err)
		handleJSONErrorResponse(sh.log, w, http.StatusInternalServerError, err)
		return err
	}

	// return storage to client
	_, err = fmt.Fprint(w, protojson.MarshalOptions{Multiline: true, EmitUnpopulated: true, Indent: ""}.Format(storage))
	if err != nil {
		sh.log.WithError(err).Errorf("writing storage get response: %v", err)
		handleJSONErrorResponse(sh.log, w, http.StatusInternalServerError, err)
		return err
	}

	w.WriteHeader(http.StatusOK)
	return nil
}

func (sh *StorageHandler) deleteHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// parse storagetype from request parameters
	params := r.URL.Query()["StorageType"]
	if len(params) == 0 {
		err := fmt.Errorf("storage type not provided in query parameters")
		sh.log.WithError(err).Error()
		handleJSONErrorResponse(sh.log, w, http.StatusBadRequest, err)
		return err
	}

	storType := params[0]

	// parse storage systemid from request parameters
	params = r.URL.Query()["SystemId"]
	if len(params) == 0 {
		err := fmt.Errorf("storage systemid not provided in query parameters")
		sh.log.WithError(err).Error()
		handleJSONErrorResponse(sh.log, w, http.StatusBadRequest, err)
		return err
	}

	sysID := params[0]

	setAttributes(span, map[string]interface{}{
		"storageType": storType,
		"systemID":    sysID,
	})

	sh.log.WithFields(logrus.Fields{
		"storageType": storType,
		"systemID":    sysID,
	}).Info("Requesting storage delete")

	// call storage service
	_, err := sh.client.Delete(ctx, &pb.StorageDeleteRequest{
		StorageType: storType,
		SystemId:    sysID,
	})
	if err != nil {
		sh.log.WithError(err).Errorf("deleting storage: %v", err)
		handleJSONErrorResponse(sh.log, w, http.StatusInternalServerError, err)
		return err
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}
