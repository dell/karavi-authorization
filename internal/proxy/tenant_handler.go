package proxy

import (
	"encoding/json"
	"fmt"
	"karavi-authorization/internal/web"
	"karavi-authorization/pb"
	"net/http"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/encoding/protojson"
)

type TenantHandler struct {
	mux    *http.ServeMux
	client pb.TenantServiceClient
	log    *logrus.Entry
}

func NewTenantHandler(log *logrus.Entry, client pb.TenantServiceClient) *TenantHandler {
	th := &TenantHandler{
		client: client,
		log:    log,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(fmt.Sprintf("%s%s", web.ProxyTenantPath, "create"), th.createHandler)
	mux.HandleFunc(fmt.Sprintf("%s%s", web.ProxyTenantPath, "update"), th.updateHandler)
	mux.HandleFunc(fmt.Sprintf("%s%s", web.ProxyTenantPath, "get"), th.getHandler)

	return &TenantHandler{
		mux:    mux,
		client: client,
	}
}

func (th *TenantHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	th.mux.ServeHTTP(w, r)
}

type createTenantBody struct {
	Name       string `json:"name"`
	ApproveSdc bool   `json:"approveSdc"`
}

func (th *TenantHandler) createHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.SpanFromContext(r.Context()).TracerProvider().Tracer("csm-authorization-proxy-server").Start(r.Context(), "tenantCreateHandler")
	defer span.End()

	if r.Method != http.MethodPost {
		err := fmt.Errorf("method %s not allowed", r.Method)
		th.log.WithError(err).Error()
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			th.log.WithError(err).Println("error creating json response")
		}
		return
	}

	var body createTenantBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		th.log.WithError(err).Errorf("error decoding request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, fmt.Errorf("decoding request body: %v", err)); err != nil {
			th.log.WithError(err).Println("error creating json response")
		}
		return
	}

	span.SetAttributes(attribute.KeyValue{Key: "name", Value: attribute.StringValue(body.Name)}, attribute.KeyValue{Key: "name", Value: attribute.BoolValue(body.ApproveSdc)})
	th.log.WithFields(logrus.Fields{
		"name":       body.Name,
		"approveSdc": body.ApproveSdc,
	}).Debug("Requesting tenant creation")

	_, err = th.client.CreateTenant(ctx, &pb.CreateTenantRequest{
		Tenant: &pb.Tenant{
			Name:       body.Name,
			Approvesdc: body.ApproveSdc,
		},
	})
	if err != nil {
		th.log.WithError(err).Errorf("error creating tenant: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("creating tenant: %v", err)); err != nil {
			th.log.WithError(err).Println("error creating json response")
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (th *TenantHandler) updateHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.SpanFromContext(r.Context()).TracerProvider().Tracer("csm-authorization-proxy-server").Start(r.Context(), "tenantUpdateHandler")
	defer span.End()

	if r.Method != http.MethodPatch {
		err := fmt.Errorf("method %s not allowed", r.Method)
		th.log.WithError(err).Error()
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			th.log.WithError(err).Println("error creating json response")
		}
		return
	}

	var body createTenantBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		th.log.WithError(err).Errorf("error decoding request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, fmt.Errorf("decoding request body: %v", err)); err != nil {
			th.log.WithError(err).Println("error creating json response")
		}
		return
	}

	span.SetAttributes(attribute.KeyValue{Key: "name", Value: attribute.StringValue(body.Name)}, attribute.KeyValue{Key: "name", Value: attribute.BoolValue(body.ApproveSdc)})
	th.log.WithFields(logrus.Fields{
		"name":       body.Name,
		"approveSdc": body.ApproveSdc,
	}).Debug("Requesting tenant update")

	_, err = th.client.UpdateTenant(ctx, &pb.UpdateTenantRequest{
		TenantName: body.Name,
		Approvesdc: body.ApproveSdc,
	})
	if err != nil {
		th.log.WithError(err).Errorf("error updating tenant: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("updating tenant: %v", err)); err != nil {
			th.log.WithError(err).Println("error creating json response")
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (th *TenantHandler) getHandler(w http.ResponseWriter, r *http.Request) {
	ctx, span := trace.SpanFromContext(r.Context()).TracerProvider().Tracer("csm-authorization-proxy-server").Start(r.Context(), "tenantGetHandler")
	defer span.End()

	if r.Method != http.MethodGet {
		err := fmt.Errorf("method %s not allowed", r.Method)
		th.log.WithError(err).Error()
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			th.log.WithError(err).Println("error creating json response")
		}
		return
	}

	params := r.URL.Query()["name"]
	if len(params) == 0 {
		err := fmt.Errorf("tenant name not provided in query parameters")
		th.log.WithError(err).Error()
		w.WriteHeader(http.StatusBadRequest)
		if err := web.JSONErrorResponse(w, err); err != nil {
			th.log.WithError(err).Println("error creating json response")
		}
		return
	}

	name := params[0]

	span.SetAttributes(attribute.KeyValue{Key: "name", Value: attribute.StringValue(name)})
	th.log.WithFields(logrus.Fields{
		"name": name,
	}).Debug("Requesting tenant get")

	tenant, err := th.client.GetTenant(ctx, &pb.GetTenantRequest{
		Name: name,
	})
	if err != nil {
		th.log.WithError(err).Errorf("error getting tenant: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("getting tenant: %v", err)); err != nil {
			th.log.WithError(err).Println("error creating json response")
		}
		return
	}

	_, err = fmt.Fprint(w, protojson.MarshalOptions{Multiline: true, EmitUnpopulated: true, Indent: ""}.Format(tenant))
	if err != nil {
		th.log.WithError(err).Errorf("error writing tenant get response: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("writing tenant get response: %v", err)); err != nil {
			th.log.WithError(err).Println("error creating json response")
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}
