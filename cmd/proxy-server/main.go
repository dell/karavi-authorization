// Copyright © 2021-2023 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"crypto/tls"
	"encoding/json"
	"expvar"
	"flag"
	"fmt"
	"io"
	"karavi-authorization/internal/proxy"
	"karavi-authorization/internal/quota"
	"karavi-authorization/internal/role-service"
	"karavi-authorization/internal/role-service/roles"
	"karavi-authorization/internal/sdc"
	"karavi-authorization/internal/storage-service"
	"karavi-authorization/internal/token"
	"karavi-authorization/internal/token/jwx"
	"karavi-authorization/internal/web"
	"karavi-authorization/pb"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	stdLog "log"

	"github.com/fsnotify/fsnotify"
	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/export/metric/aggregation"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.7.0"
	"google.golang.org/grpc"
	"sigs.k8s.io/yaml"
)

const (
	configParamJWTSigningScrt = "web.jwtsigningsecret"
	configParamLogLevel       = "LOG_LEVEL"
	configParamLogFormat      = "LOG_FORMAT"
	storageSystemsPath        = "/etc/karavi-authorization/storage/storage-systems.yaml"
)

var (
	// build is to be set via build flags in the makefile.
	build = "develop"
	cfg   Config
	// JWTSigningSecret is the secret string used to sign JWT tokens
	JWTSigningSecret = "secret"
)

func init() {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
}

func main() {
	log := logrus.New()

	if err := run(log.WithContext(context.Background())); err != nil {
		log.Errorf("main: error: %+v", err)
		os.Exit(1)
	}
}

type roleClientService struct {
	roleService *role.Service
	roleClient  pb.RoleServiceClient
}

type storageClientService struct {
	storageService *storage.Service
	storageClient  pb.StorageServiceClient
}

// Config is the configuration details on the proxy-server
type Config struct {
	Version string
	Zipkin  struct {
		CollectorURI string
		ServiceName  string
		Probability  float64
	}
	Certificate struct {
		CrtFile         string
		KeyFile         string
		RootCertificate string
	}
	Proxy struct {
		Host         string
		ReadTimeout  time.Duration
		WriteTimeout time.Duration
	}
	Web struct {
		ShowDebugHTTP    bool
		DebugHost        string
		ShutdownTimeout  time.Duration
		JWTSigningSecret string
	}
	Database struct {
		Host     string
		Password string
	}
	OpenPolicyAgent struct {
		Host string
	}
}

func run(log *logrus.Entry) error {
	redisHost := flag.String("redis-host", "", "address of redis host")
	tenantService := flag.String("tenant-service", "", "address of tenant service")
	roleService := flag.String("role-service", "", "address of role service")
	storageService := flag.String("storage-service", "", "address of storage service")
	flag.Parse()

	cfgViper := viper.New()
	cfgViper.SetConfigName("config")
	cfgViper.AddConfigPath(".")
	cfgViper.AddConfigPath("/etc/karavi-authorization/config/")

	cfgViper.SetDefault("certificate.crtfile", "")
	cfgViper.SetDefault("certificate.keyfile", "")

	cfgViper.SetDefault("proxy.host", ":8080")
	cfgViper.SetDefault("proxy.readtimeout", 30*time.Second)
	cfgViper.SetDefault("proxy.writetimeout", 30*time.Second)

	cfgViper.SetDefault("web.debughost", ":9090")
	cfgViper.SetDefault("web.shutdowntimeout", 15*time.Second)
	cfgViper.SetDefault(configParamJWTSigningScrt, "secret")
	cfgViper.SetDefault("web.showdebughttp", false)

	cfgViper.SetDefault("zipkin.collectoruri", "")
	cfgViper.SetDefault("zipkin.servicename", "proxy-server")
	cfgViper.SetDefault("zipkin.probability", 0.8)

	cfgViper.SetDefault("database.host", "redis.karavi.svc.cluster.local:6379")
	cfgViper.SetDefault("database.password", "")

	cfgViper.SetDefault("openpolicyagent.host", "127.0.0.1:8181")

	if err := cfgViper.ReadInConfig(); err != nil {
		log.Fatalf("reading config file: %+v", err)
	}
	if err := cfgViper.Unmarshal(&cfg); err != nil {
		log.Fatalf("decoding config file: %+v", err)
	}

	web.JWTSigningSecret = cfg.Web.JWTSigningSecret
	JWTSigningSecret = cfg.Web.JWTSigningSecret

	cfgViper.WatchConfig()
	cfgViper.OnConfigChange(func(e fsnotify.Event) {
		updateConfiguration(cfgViper, log)
	})

	log.Infof("Config: %+v", cfg)

	csmViper := viper.New()
	csmViper.SetConfigName("csm-config-params")
	csmViper.AddConfigPath("/etc/karavi-authorization/csm-config-params/")

	if err := csmViper.ReadInConfig(); err != nil {
		log.Fatalf("reading csm-config-params file: %+v", err)
	}

	updateLoggingSettings := func(log *logrus.Entry) {
		logFormat := csmViper.GetString(configParamLogFormat)
		if strings.EqualFold(logFormat, "json") {
			log.Logger.SetFormatter(&logrus.JSONFormatter{})
		} else {
			// use text formatter by default
			log.Logger.SetFormatter(&logrus.TextFormatter{})
		}
		if logFormat != "" {
			log.WithField(configParamLogFormat, logFormat).Info("configuration has been set")
		}

		logLevel := csmViper.GetString(configParamLogLevel)
		level, err := logrus.ParseLevel(logLevel)
		if err != nil {
			// use INFO level by default
			level = logrus.InfoLevel
		}

		// There are two log statements to ensure that we capture all LOG_LEVEL changes
		log.WithField(configParamLogLevel, level.String()).Info("configuration has been set")
		log.Logger.SetLevel(level)
		log.WithField(configParamLogLevel, level.String()).Info("configuration has been set")
	}
	updateLoggingSettings(log)

	csmViper.WatchConfig()
	csmViper.OnConfigChange(func(e fsnotify.Event) {
		updateLoggingSettings(log)
	})

	// Initializing application

	cfg.Version = build
	expvar.NewString("build").Set(build)

	log.Infof("main: started application version %q", build)
	defer log.Info("main: stopped application")

	// Initialize authentication

	// Initialize OPA

	// Initialize database connections

	redisAddr := cfg.Database.Host
	if *redisHost != "" {
		redisAddr = *redisHost
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr, // "redis.karavi.svc.cluster.local:6379",
		Password: cfg.Database.Password,
		DB:       0,
	})
	defer func() {
		if err := rdb.Close(); err != nil {
			log.WithError(err).Warn("closing redis")
		}
	}()
	enf := quota.NewRedisEnforcement(context.Background(), quota.WithRedis(rdb))
	sdcapr := sdc.NewSdcApprover(context.Background(), sdc.WithRedis(rdb))

	// Start tracing support

	tp, err := initTracing(log,
		cfg.Zipkin.CollectorURI,
		"csm-authorization-proxy-server",
		cfg.Zipkin.Probability)
	if err != nil {
		return err
	}

	// Start debug service
	//
	// /debug/pprof - added to the default mux by importing the net/http/pprof package.
	// /debug/vars - added to the default mux by importing the expvar package.
	//
	log.Info("main: initializing debugging support")

	config := prometheus.Config{}
	c := controller.New(
		processor.NewFactory(
			selector.NewWithHistogramDistribution(
				histogram.WithExplicitBoundaries(config.DefaultHistogramBoundaries),
			),
			aggregation.CumulativeTemporalitySelector(),
			processor.WithMemory(true),
		),
	)

	metricsExp, err := prometheus.New(config, c)
	if err != nil {
		return err
	}
	http.HandleFunc("/metrics", metricsExp.ServeHTTP)

	go func() {
		expvar.Publish("goroutines", expvar.Func(func() interface{} {
			return fmt.Sprintf("%d", runtime.NumGoroutine())
		}))
		log.WithField("debug host", cfg.Web.DebugHost).Debug("main: debug listening")
		s := http.Server{
			Addr:              cfg.Web.DebugHost,
			Handler:           http.DefaultServeMux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		if err := s.ListenAndServe(); err != nil {
			log.WithError(err).Warn("main: debug listener closed")
		}
	}()

	// Start watching for config changes for storage systems

	sysViper := viper.New()
	sysViper.SetConfigName("storage-systems")
	sysViper.AddConfigPath(".")
	sysViper.AddConfigPath("/etc/karavi-authorization/storage/")
	sysViper.WatchConfig()

	// Create handlers for the supported storage arrays.
	powerFlexHandler := proxy.NewPowerFlexHandler(log, enf, sdcapr, cfg.OpenPolicyAgent.Host)
	powerMaxHandler := proxy.NewPowerMaxHandler(log, enf, cfg.OpenPolicyAgent.Host)
	powerScaleHandler := proxy.NewPowerScaleHandler(log, enf, cfg.OpenPolicyAgent.Host)

	updaterFn := func() {
		err := updateStorageSystems(log, storageSystemsPath, powerFlexHandler, powerMaxHandler, powerScaleHandler)
		if err != nil {
			log.WithError(err).Error("main: updating storage systems")
		}
	}

	// Update on config changes.
	sysViper.OnConfigChange(func(e fsnotify.Event) {
		log.Infof("Configuration changed! %+v, %s", e.Op, e.Name)
		updaterFn()
	})
	updaterFn()

	// Create the handlers

	systemHandlers := map[string]http.Handler{
		"powerflex":  web.Adapt(powerFlexHandler, web.OtelMW(tp, "powerflex")),
		"powermax":   web.Adapt(powerMaxHandler, web.OtelMW(tp, "powermax")),
		"powerscale": web.Adapt(powerScaleHandler, web.OtelMW(tp, "powerscale")),
	}
	dh := proxy.NewDispatchHandler(log, systemHandlers)

	tenantAddr := "tenant-service.karavi.svc.cluster.local:50051"
	roleAddr := "role-service.karavi.svc.cluster.local:50051"
	storageAddr := "storage-service.karavi.svc.cluster.local:50051"

	if *tenantService != "" {
		tenantAddr = *tenantService
	}
	if *roleService != "" {
		roleAddr = *roleService
	}
	if *storageService != "" {
		storageAddr = *storageService
	}

	tenantConn, err := grpc.Dial(tenantAddr,
		grpc.WithTimeout(10*time.Second),
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()))
	if err != nil {
		return err
	}
	defer tenantConn.Close()

	roleConn, err := grpc.Dial(roleAddr,
		grpc.WithTimeout(10*time.Second),
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()))
	if err != nil {
		return err
	}
	defer roleConn.Close()

	storageConn, err := grpc.Dial(storageAddr,
		grpc.WithTimeout(10*time.Second),
		grpc.WithInsecure(),
		grpc.WithUnaryInterceptor(otelgrpc.UnaryClientInterceptor()),
		grpc.WithStreamInterceptor(otelgrpc.StreamClientInterceptor()))
	if err != nil {
		return err
	}
	defer storageConn.Close()

	router := &web.Router{
		RolesHandler:      web.Adapt(rolesHandler(log, cfg.OpenPolicyAgent.Host), web.OtelMW(tp, "roles")),
		TokenHandler:      web.Adapt(refreshTokenHandler(pb.NewTenantServiceClient(tenantConn), log), web.OtelMW(tp, "tenant_refresh")),
		AdminTokenHandler: web.Adapt(refreshAdminTokenHandler(log), web.OtelMW(tp, "admin_refresh")),
		ProxyHandler:      web.Adapt(dh, web.OtelMW(tp, "dispatch")),
		VolumesHandler:    web.Adapt(volumesHandler(&roleClientService{roleClient: pb.NewRoleServiceClient(roleConn)}, &storageClientService{storageClient: pb.NewStorageServiceClient(storageConn)}, rdb, jwx.NewTokenManager(jwx.HS256), log), web.OtelMW(tp, "volumes")),
		TenantHandler:     web.Adapt(proxy.NewTenantHandler(log, pb.NewTenantServiceClient(tenantConn)), web.OtelMW(tp, "tenant_handler")),
		StorageHandler:    web.Adapt(proxy.NewStorageHandler(log, pb.NewStorageServiceClient(storageConn)), web.OtelMW(tp, "storage_handler")),
	}

	// Start the proxy service
	log.Info("main: initializing proxy service")

	svr := http.Server{
		Addr: cfg.Proxy.Host,
		Handler: web.Adapt(router.Handler(),
			web.LoggingMW(log, cfg.Web.ShowDebugHTTP), // log all requests
			web.CleanMW(), // clean paths
			web.OtelMW(tp, "", // format the span name
				otelhttp.WithSpanNameFormatter(func(s string, r *http.Request) string {
					return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
				})),
			web.AuthMW(log, jwx.NewTokenManager(jwx.HS256))),
		ReadTimeout:       cfg.Proxy.ReadTimeout,
		WriteTimeout:      cfg.Proxy.WriteTimeout,
		ReadHeaderTimeout: 5 * time.Second,
	}
	// Start listening for requests
	serverErrors := make(chan error, 1)
	go func() {
		log.WithField("proxy host", cfg.Proxy.Host).Info("main: proxy listening")
		serverErrors <- svr.ListenAndServe()
	}()

	// Handle graceful shutdown

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("main: server error: %w", err)
	case sig := <-shutdown:
		log.WithField("signal", sig).Info("main: starting shutdown")
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Web.ShutdownTimeout)
		defer cancel()

		// Ask the proxy to shutdown and shed load
		if err := svr.Shutdown(ctx); err != nil {
			closeErr := svr.Close()
			if closeErr != nil {
				return fmt.Errorf("main: failed to close server: %w", closeErr)
			}
			return fmt.Errorf("main: failed to gracefully shutdown server: %w", err)
		}
	}

	return nil
}

func updateConfiguration(vc *viper.Viper, log *logrus.Entry) {
	jss := cfg.Web.JWTSigningSecret
	if vc.IsSet(configParamJWTSigningScrt) {
		value := vc.GetString(configParamJWTSigningScrt)
		jss = value
		log.WithField(configParamJWTSigningScrt, "***").Info("configuration has been set")
	}
	web.JWTSigningSecret = jss
	JWTSigningSecret = jss
}

func updateStorageSystems(log *logrus.Entry, storageSystemsPath string, powerFlexHandler *proxy.PowerFlexHandler, powerMaxHandler *proxy.PowerMaxHandler, powerScaleHandler *proxy.PowerScaleHandler) error {
	// read the storage-systems file
	storageYamlBytes, err := os.ReadFile(filepath.Clean(storageSystemsPath))
	if err != nil {
		return fmt.Errorf("reading storage systems: %w", err)
	}

	// unmarshal the yaml data
	var v map[string]interface{}
	err = yaml.Unmarshal(storageYamlBytes, &v)
	if err != nil {
		return fmt.Errorf("unmarshaling storage systems: %w", err)
	}

	// extract the storage field
	storage, ok := v["storage"]
	if !ok {
		return fmt.Errorf("storage key not found in storage-systems data")
	}

	// marshal the storage data
	systemsYamlBytes, err := yaml.Marshal(storage)
	if err != nil {
		return fmt.Errorf("marshaling storage systems: %w", err)
	}

	// convert above storage data to json
	systemsJSONBytes, err := yaml.YAMLToJSON(systemsYamlBytes)
	if err != nil {
		return fmt.Errorf("converting yaml to json: %w", err)
	}

	// update the systems with the json data

	err = powerFlexHandler.UpdateSystems(context.Background(), bytes.NewReader(systemsJSONBytes), log)
	if err != nil {
		log.WithError(err).Error("main: updating powerflex systems")
	}

	err = powerMaxHandler.UpdateSystems(context.Background(), bytes.NewReader(systemsJSONBytes), log)
	if err != nil {
		log.WithError(err).Error("main: updating powermax systems")
	}

	err = powerScaleHandler.UpdateSystems(context.Background(), bytes.NewReader(systemsJSONBytes), log)
	if err != nil {
		log.WithError(err).Error("main: updating powerscale systems")
	}

	return nil
}

func initTracing(log *logrus.Entry, uri, name string, prob float64) (*trace.TracerProvider, error) {
	if len(strings.TrimSpace(uri)) == 0 {
		return nil, nil
	}

	log.Info("main: initializing otel/zipkin tracing support")

	exporter, err := zipkin.New(
		uri,
		zipkin.WithLogger(stdLog.New(io.Discard, "", stdLog.LstdFlags)),
	)
	if err != nil {
		return nil, fmt.Errorf("creating zipkin exporter: %w", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithSampler(trace.TraceIDRatioBased(prob)),
		trace.WithBatcher(
			exporter,
			trace.WithMaxExportBatchSize(trace.DefaultMaxExportBatchSize),
			trace.WithBatchTimeout(trace.DefaultBatchTimeout),
			trace.WithMaxExportBatchSize(trace.DefaultMaxExportBatchSize),
		),
		trace.WithResource(resource.NewWithAttributes(semconv.SchemaURL,
			attribute.KeyValue{Key: semconv.ServiceNameKey, Value: attribute.StringValue(name)})),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}))
	return tp, nil
}

func refreshTokenHandler(client pb.TenantServiceClient, log *logrus.Entry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info("Refreshing token!")
		type tokenPair struct {
			RefreshToken string `json:"refreshToken,omitempty"`
			AccessToken  string `json:"accessToken"`
		}
		var input tokenPair
		err := json.NewDecoder(r.Body).Decode(&input)
		if err != nil {
			log.WithError(err).Error("decoding token pair")
			http.Error(w, "decoding token pair", http.StatusInternalServerError)
			return
		}

		refreshResp, err := client.RefreshToken(r.Context(), &pb.RefreshTokenRequest{
			AccessToken:      input.AccessToken,
			RefreshToken:     input.RefreshToken,
			JWTSigningSecret: JWTSigningSecret,
		})
		if err != nil {
			log.WithError(err).Error("refreshing token")
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var output tokenPair
		output.AccessToken = refreshResp.AccessToken
		err = json.NewEncoder(w).Encode(&output)
		if err != nil {
			log.WithError(err).Error("encoding token pair")
			http.Error(w, "encoding token pair", http.StatusInternalServerError)
			return
		}
	})
}

// refreshAdminTokenHandler refreshes an admin token
func refreshAdminTokenHandler(log *logrus.Entry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Info("Refreshing admin token!")
		var input token.AdminToken
		err := json.NewDecoder(r.Body).Decode(&input)
		if err != nil {
			if err := web.JSONErrorResponse(w, http.StatusInternalServerError, fmt.Errorf("decoding admin token pair %v", err)); err != nil {
				log.WithError(err).Println("sending json response")
			}
			return
		}

		refreshResp, err := jwx.RefreshAdminToken(context.Background(), &pb.RefreshAdminTokenRequest{
			RefreshToken:     input.Refresh,
			AccessToken:      input.Access,
			JWTSigningSecret: JWTSigningSecret,
		})
		if err != nil {
			if err := web.JSONErrorResponse(w, http.StatusInternalServerError, fmt.Errorf("refreshing admin token %v", err)); err != nil {
				log.WithError(err).Println("sending json response")
			}
			return
		}

		var resp pb.RefreshAdminTokenResponse
		resp.AccessToken = refreshResp.AccessToken
		err = json.NewEncoder(w).Encode(&resp)
		if err != nil {
			if err := web.JSONErrorResponse(w, http.StatusInternalServerError, fmt.Errorf("encoding admin token pair %v", err)); err != nil {
				log.WithError(err).Println("sending json response")
			}
			return
		}
	})
}

func rolesHandler(log *logrus.Entry, opaHost string) http.Handler {
	url := fmt.Sprintf("http://%s/v1/data/karavi/common/roles", opaHost)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			log.WithError(err).Fatal()
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			log.WithError(err).Fatal()
		}
		_, err = io.Copy(w, res.Body)
		if err != nil {
			log.WithError(err).Fatal()
		}
		defer res.Body.Close()
	})
}

func volumesHandler(roleServ *roleClientService, storageServ *storageClientService, rdb *redis.Client, tm token.Manager, log *logrus.Entry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var sysID, sysType, storPool, tenant string
		var volumeMap = make(map[string]map[string]string)
		var volumeList []*pb.Volume
		var resp *pb.RoleListResponse
		keyTenantRevoked := "tenant:revoked"

		authz := r.Header.Get("Authorization")
		parts := strings.Split(authz, " ")
		if len(parts) != 2 {
			if err := web.JSONErrorResponse(w, http.StatusUnauthorized, fmt.Errorf("invalid authz header")); err != nil {
				log.WithError(err).Println("error creating json response")
			}
			log.Errorf("invalid authz header: %v", parts)
			return
		}
		scheme, tkn := parts[0], parts[1]

		switch scheme {
		case "Bearer":
			var claims token.Claims
			//check validity of token
			_, err := tm.ParseWithClaims(tkn, JWTSigningSecret, &claims)
			if err != nil {
				log.WithError(err).Printf("error parsing token: %v", err)
				if jsonErr := web.JSONErrorResponse(w, http.StatusUnauthorized, fmt.Errorf("validating token: %v", err)); jsonErr != nil {
					log.WithError(jsonErr).Println("error creating json response")
				}
				return
			}
			// Check if the tenant is being denied.
			ok, err := rdb.SIsMember(keyTenantRevoked, claims.Group).Result()
			if err != nil {
				log.WithError(err).Printf("error checking tenant revoked status: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				if jsonErr := web.JSONErrorResponse(w, http.StatusUnauthorized, fmt.Errorf("checking tenant revoked status: %v", err)); jsonErr != nil {
					log.WithError(jsonErr).Println("error creating json response")
				}
				return
			}
			if ok {
				w.WriteHeader(http.StatusUnauthorized)
				if err := web.JSONErrorResponse(w, http.StatusUnauthorized, fmt.Errorf("tenant is revoked")); err != nil {
					log.WithError(err).Println("error creating json response")
				}
				return
			}

			log.Debugf("Serving get volumes request for tenant %s", claims.Group)

			if roleServ.roleService == nil {
				resp, err = roleServ.roleClient.List(r.Context(), &pb.RoleListRequest{})
			} else {
				resp, err = roleServ.roleService.List(r.Context(), &pb.RoleListRequest{})
			}

			if err != nil {
				log.WithError(err).Printf("error listing roles: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				if jsonErr := web.JSONErrorResponse(w, http.StatusUnauthorized, fmt.Errorf("listing configured roles: %v", err)); jsonErr != nil {
					log.WithError(jsonErr).Println("error creating json response")
				}
				return
			}

			roleJSON := roles.NewJSON()
			err = roleJSON.UnmarshalJSON(resp.Roles)
			if err != nil {
				log.WithError(err).Printf("error unmarshalling role data: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				if jsonErr := web.JSONErrorResponse(w, http.StatusUnauthorized, fmt.Errorf("unmarhsalling role data: %v", err)); jsonErr != nil {
					log.WithError(jsonErr).Println("error creating json response")
				}
				return
			}

			rolesSplit := strings.Split(claims.Roles, ",")

			roleJSON.Select(func(rInst roles.Instance) {
				for _, role := range rolesSplit {
					if rInst.Name == role {
						sysID = rInst.SystemID
						storPool = rInst.Pool
						sysType = rInst.SystemType
						tenant = claims.Group
						volumeMap[sysID] = make(map[string]string)

						dataKey := fmt.Sprintf("quota:%s:%s:%s:%s:data", sysType, sysID, storPool, tenant)

						res, err := rdb.HGetAll(dataKey).Result()
						if err != nil {
							log.WithError(err).Printf("getting volume data for tenant %s, %v", tenant, err)
							w.WriteHeader(http.StatusInternalServerError)
							if jsonErr := web.JSONErrorResponse(w, http.StatusUnauthorized, fmt.Errorf("getting volume data: %v", err)); jsonErr != nil {
								log.WithError(jsonErr).Println("error creating json response")
							}
							return
						}

						if len(res) == 0 {
							log.Printf("no volumes found for tenant %s", tenant)
							w.WriteHeader(http.StatusInternalServerError)
							if err := web.JSONErrorResponse(w, http.StatusUnauthorized, fmt.Errorf("no volumes found")); err != nil {
								log.WithError(err).Println("error creating json response")
							}
							return
						}

						for volKey := range res {
							if strings.Contains(volKey, "capacity") {
								splitStr := strings.Split(volKey, ":")
								//example : vol:k8s-cb89d36285:capacity
								if len(splitStr) == 3 {
									volumeMap[sysID][splitStr[1]] = splitStr[1]
								}
							}
						}
						for volKey := range res {
							if strings.Contains(volKey, "deleted") {
								splitStr := strings.Split(volKey, ":")
								//example : vol:k8s-cb89d36285:deleted
								if len(splitStr) == 3 {
									delete(volumeMap[sysID], splitStr[1])
								}
							}
						}

						// If none found for sysId, delete in map so we can output later if there's none found for tenant
						if len(volumeMap[sysID]) == 0 {
							delete(volumeMap, sysID)
						}
					}
				}
			})

		case "Basic":
			log.Println("Basic authentication used")
			return
		}
		if len(volumeMap) == 0 {
			log.Errorf("no volumes found for tenant %s", tenant)
			w.WriteHeader(http.StatusInternalServerError)
			if err := web.JSONErrorResponse(w, http.StatusUnauthorized, fmt.Errorf("no volumes found")); err != nil {
				log.WithError(err).Println("error creating json response")
			}
		}

		for sysID, nameMap := range volumeMap {
			var currentVolumeNameList []string
			var storageResp *pb.GetPowerflexVolumesResponse
			var err error

			for _, v := range nameMap {
				currentVolumeNameList = append(currentVolumeNameList, v)
			}

			// grpc call to storage service to get volume details
			powerflexVolumesRequest := &pb.GetPowerflexVolumesRequest{
				SystemId:   sysID,
				VolumeName: currentVolumeNameList,
			}

			storageResp, err = storageServ.storageClient.GetPowerflexVolumes(r.Context(), powerflexVolumesRequest)
			if err != nil {
				log.WithError(err).Println("getting powerflex volumes")
				w.WriteHeader(http.StatusInternalServerError)
				if jsonErr := web.JSONErrorResponse(w, http.StatusUnauthorized, fmt.Errorf("getting powerflex volumes: %v", err)); jsonErr != nil {
					log.WithError(jsonErr).Println("error creating json response")
				}
				return
			}

			volumeList = append(volumeList, storageResp.Volume...)

			log.Printf("Volume Details for System ID: %s\n %v", sysID, storageResp.String())
		}

		w.WriteHeader(http.StatusOK)
		err := json.NewEncoder(w).Encode(&volumeList)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.WithError(err).Println("unable to encode body")
			return
		}
	})
}
