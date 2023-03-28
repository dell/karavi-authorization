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
	"strings"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
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
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyStoragePath, "create"), web.Adapt(web.HandlerWithError(sh.createHandler), web.TelemetryMW("storageCreateHandler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyStoragePath, "update"), web.Adapt(web.HandlerWithError(sh.updateHandler), web.TelemetryMW("storageUpdateHandler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyStoragePath, "get"), web.Adapt(web.HandlerWithError(sh.getHandler), web.TelemetryMW("storageGetHandler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyStoragePath, "delete"), web.Adapt(web.HandlerWithError(sh.deleteHandler), web.TelemetryMW("storageDeleteHandler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyStoragePath, "list"), web.Adapt(web.HandlerWithError(sh.listHandler), web.TelemetryMW("storageListHandler", log)))
	mux.Handle(fmt.Sprintf("%s%s/", web.ProxyStoragePath, "volumes"), web.Adapt(web.HandlerWithError(sh.getPowerflexVolumesHandler), web.TelemetryMW("getPowerflexVolumesHandler", log)))

	return sh
}

// ServeHTTP implements the http.Handler interface
func (sh *StorageHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sh.mux.ServeHTTP(w, r)
}

func (sh *StorageHandler) createHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// only allow POST requests
	if r.Method != http.MethodPost {
		err := fmt.Errorf("method %s not allowed", r.Method)
		sh.log.WithError(err).Error()
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return err
	}

	// read request body
	var body createStorageBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		sh.log.WithError(err).Errorf("decoding request body")
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, fmt.Errorf("decoding request body: %v", err)); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("decoding request body: %v", err)
	}

	span.SetAttributes(attribute.KeyValue{Key: "StorageType", Value: attribute.StringValue(body.StorageType)},
		attribute.KeyValue{Key: "Endpoint", Value: attribute.StringValue(body.Endpoint)},
		attribute.KeyValue{Key: "SystemId", Value: attribute.StringValue(body.SystemId)},
		attribute.KeyValue{Key: "UserName", Value: attribute.StringValue(body.UserName)},
		attribute.KeyValue{Key: "Password", Value: attribute.StringValue(body.Password)},
		attribute.KeyValue{Key: "Insecure", Value: attribute.BoolValue(body.Insecure)})

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
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("creating storage: %v", err)); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("creating storage: %v", err)
	}

	w.WriteHeader(http.StatusCreated)
	return nil
}

func (sh *StorageHandler) updateHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// only allow PATCH requests
	if r.Method != http.MethodPatch {
		err := fmt.Errorf("method %s not allowed", r.Method)
		sh.log.WithError(err).Error()
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return err
	}

	// read request body
	var body createStorageBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		sh.log.WithError(err).Errorf("decoding request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, fmt.Errorf("decoding request body: %v", err)); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("decoding request body: %v", err)
	}

	span.SetAttributes(attribute.KeyValue{Key: "StorageType", Value: attribute.StringValue(body.StorageType)},
		attribute.KeyValue{Key: "Endpoint", Value: attribute.StringValue(body.Endpoint)},
		attribute.KeyValue{Key: "SystemId", Value: attribute.StringValue(body.SystemId)},
		attribute.KeyValue{Key: "UserName", Value: attribute.StringValue(body.UserName)},
		attribute.KeyValue{Key: "Password", Value: attribute.StringValue(body.Password)},
		attribute.KeyValue{Key: "Insecure", Value: attribute.BoolValue(body.Insecure)})

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
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("updating storage: %v", err)); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("updating storage: %v", err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (sh *StorageHandler) getHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// only allow GET requests
	if r.Method != http.MethodGet {
		err := fmt.Errorf("method %s not allowed", r.Method)
		sh.log.WithError(err).Error()
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return err
	}

	// parse storagetype from request parameters
	params := r.URL.Query()["StorageType"]
	if len(params) == 0 {
		err := fmt.Errorf("storage type not provided in query parameters")
		sh.log.WithError(err).Error()
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, err); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return err
	}

	storType := params[0]

	// parse storage systemid from request parameters
	params = r.URL.Query()["SystemId"]
	if len(params) == 0 {
		err := fmt.Errorf("storage systemid not provided in query parameters")
		sh.log.WithError(err).Error()
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, err); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return err
	}

	sysID := params[0]

	span.SetAttributes(attribute.KeyValue{Key: "storageType", Value: attribute.StringValue(storType)})
	span.SetAttributes(attribute.KeyValue{Key: "systemID", Value: attribute.StringValue(sysID)})

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
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("getting storage: %v", err)); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("getting storage: %v", err)
	}

	// return storage to client
	_, err = fmt.Fprint(w, protojson.MarshalOptions{Multiline: true, EmitUnpopulated: true, Indent: ""}.Format(storage))
	if err != nil {
		sh.log.WithError(err).Errorf("writing storage get response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("writing storage get response: %v", err)); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("writing storage get response: %v", err)
	}

	w.WriteHeader(http.StatusOK)
	return nil
}

func (sh *StorageHandler) deleteHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// only allow DELETE requests
	if r.Method != http.MethodDelete {
		err := fmt.Errorf("method %s not allowed", r.Method)
		sh.log.WithError(err).Error()
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return err
	}

	// parse storagetype from request parameters
	params := r.URL.Query()["StorageType"]
	if len(params) == 0 {
		err := fmt.Errorf("storage type not provided in query parameters")
		sh.log.WithError(err).Error()
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, err); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return err
	}

	storType := params[0]

	// parse storage systemid from request parameters
	params = r.URL.Query()["SystemId"]
	if len(params) == 0 {
		err := fmt.Errorf("storage systemid not provided in query parameters")
		sh.log.WithError(err).Error()
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, err); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return err
	}

	sysID := params[0]

	span.SetAttributes(attribute.KeyValue{Key: "storageType", Value: attribute.StringValue(storType)})
	span.SetAttributes(attribute.KeyValue{Key: "systemID", Value: attribute.StringValue(sysID)})

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
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("deleting storage: %v", err)); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("deleting storage: %v", err)
	}

	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (sh *StorageHandler) listHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	// only allow GET requests
	if r.Method != http.MethodGet {
		err := fmt.Errorf("method %s not allowed", r.Method)
		sh.log.WithError(err).Error()
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return err
	}

	sh.log.Info("Requesting storage list")

	// call storage service
	storages, err := sh.client.List(ctx, &pb.StorageListRequest{})
	if err != nil {
		sh.log.WithError(err).Errorf("listing storages: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("listing storages: %v", err)); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("listing storages: %v", err)
	}

	// write Storage to client
	_, err = fmt.Fprint(w, protojson.MarshalOptions{Multiline: true, EmitUnpopulated: true, Indent: ""}.Format(storages))
	if err != nil {
		sh.log.WithError(err).Errorf("writing storage list response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("writing storage list response: %v", err)); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("writing storage list response: %v", err)
	}

	w.WriteHeader(http.StatusOK)
	return nil
}
func (sh *StorageHandler) getPowerflexVolumesHandler(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	span := trace.SpanFromContext(ctx)

	// only allow GET requests
	if r.Method != http.MethodGet {
		err := fmt.Errorf("method %s not allowed", r.Method)
		sh.log.WithError(err).Error()
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return err
	}

	// parse storagetype from request parameters
	params := r.URL.Query()["VolumeName"]
	if len(params) == 0 {
		err := fmt.Errorf("VolumeName not provided in query parameters")
		sh.log.WithError(err).Error()
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, err); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return err
	}

	volName := params[0]

	// parse storage systemid from request parameters
	params = r.URL.Query()["SystemId"]
	if len(params) == 0 {
		err := fmt.Errorf("storage systemid not provided in query parameters")
		sh.log.WithError(err).Error()
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, err); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return err
	}

	sysID := params[0]

	span.SetAttributes(attribute.KeyValue{Key: "VolumeName", Value: attribute.StringValue(volName)})
	span.SetAttributes(attribute.KeyValue{Key: "systemID", Value: attribute.StringValue(sysID)})

	sh.log.WithFields(logrus.Fields{
		"VolumeName": volName,
		"systemID":   sysID,
	}).Info("Requesting storage get")

	//change volumeName into a slice stiring value
	volNameSlice := strings.Split(volName, ",")

	// call get powerflex volumes service
	storage, err := sh.client.GetPowerflexVolumes(ctx, &pb.GetPowerflexVolumesRequest{
		VolumeName: volNameSlice,
		SystemId:   sysID,
	})
	if err != nil {
		sh.log.WithError(err).Errorf("getting powerflex volumes: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("getting powerflex volumes: %v", err)); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("getting powerflex volumes: %v", err)
	}

	// return powerflex volumes to client
	_, err = fmt.Fprint(w, protojson.MarshalOptions{Multiline: true, EmitUnpopulated: true, Indent: ""}.Format(storage))
	if err != nil {
		sh.log.WithError(err).Errorf("writing powerflex volumes get response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("writing powerflex volumes get response: %v", err)); err != nil {
			sh.log.WithError(err).Error("creating json response")
		}
		return fmt.Errorf("writing powerflex volumes get response: %v", err)
	}

	w.WriteHeader(http.StatusOK)
	return nil
}
