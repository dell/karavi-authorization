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
	ctx, span := trace.SpanFromContext(r.Context()).TracerProvider().Tracer("").Start(r.Context(), "createTenantHandler")
	defer span.End()

	th.log.Info("Serving tenant create request")

	if r.Method != http.MethodPost {
		err := fmt.Errorf("method %s not allowed", r.Method)
		th.log.WithError(err).Printf(err.Error())
		w.WriteHeader(http.StatusMethodNotAllowed)
		if err := web.JSONErrorResponse(w, err); err != nil {
			th.log.WithError(err).Println("error creating json response")
		}
		return
	}

	var body createTenantBody
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		th.log.WithError(err).Printf("error decoding request body: %v", err)
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
	}).Info("Calling tenant service")

	_, err = th.client.CreateTenant(ctx, &pb.CreateTenantRequest{
		Tenant: &pb.Tenant{
			Name:       body.Name,
			Approvesdc: body.ApproveSdc,
		},
	})
	if err != nil {
		th.log.WithError(err).Printf("error creating tenant: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		if err := web.JSONErrorResponse(w, fmt.Errorf("creating tenant: %v", err)); err != nil {
			th.log.WithError(err).Println("error creating json response")
		}
	}
}
