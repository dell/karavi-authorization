// Copyright © 2021 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"fmt"
	"io"
	"io/ioutil"
	"karavi-authorization/internal/proxy"
	"karavi-authorization/internal/quota"
	"karavi-authorization/internal/web"
	"karavi-authorization/pb"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	stdLog "log"

	"github.com/fsnotify/fsnotify"
	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/metric/prometheus"
	"go.opentelemetry.io/otel/exporters/trace/zipkin"
	"go.opentelemetry.io/otel/sdk/trace"
	"google.golang.org/grpc"
)

// build is to be set via build flags in the makefile.
var build = "develop"

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

func run(log *logrus.Entry) error {
	var cfg struct {
		Version string
		Zipkin  struct {
			CollectorURI string
			ServiceName  string
			Probability  float64
		}
		Certificate struct {
			CrtFile string
			KeyFile string
		}
		Proxy struct {
			Host         string
			ReadTimeout  time.Duration
			WriteTimeout time.Duration
		}
		Web struct {
			DebugHost        string
			ShutdownTimeout  time.Duration
			SidecarProxyAddr string
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
	cfgViper.SetDefault("web.sidecarproxyaddr", web.DefaultSidecarProxyAddr)
	cfgViper.SetDefault("web.jwtsigningsecret", "secret")

	cfgViper.SetDefault("zipkin.collectoruri", "http://localhost:9411/api/v2/spans")
	cfgViper.SetDefault("zipkin.servicename", "proxy-server")
	cfgViper.SetDefault("zipkin.probability", 0.8)

	cfgViper.SetDefault("database.host", "redis.karavi.svc.cluster.local:6379")
	cfgViper.SetDefault("database.password", "")

	cfgViper.SetDefault("openpolicyagent.host", "localhost:8181")

	if err := cfgViper.ReadInConfig(); err != nil {
		log.Fatalf("reading config file: %+v", err)
	}
	if err := cfgViper.Unmarshal(&cfg); err != nil {
		log.Fatalf("decoding config file: %+v", err)
	}

	log.Printf("Config: %+v", cfg)

	// Initializing application

	cfg.Version = build
	expvar.NewString("build").Set(build)

	log.Printf("main: started application version %q", build)
	defer log.Println("main: stopped application")

	// Initialize authentication

	// Initialize OPA

	// Initialize database connections

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Database.Host, // "redis.karavi.svc.cluster.local:6379",
		Password: cfg.Database.Password,
		DB:       0,
	})
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Printf("closing redis: %+v", err)
		}
	}()
	enf := quota.NewRedisEnforcement(context.Background(), rdb)

	// Start tracing support

	log.Println("main: initializing otel/zipkin tracing support")

	exporter, err := zipkin.NewRawExporter(
		cfg.Zipkin.CollectorURI,
		cfg.Zipkin.ServiceName,
		zipkin.WithLogger(stdLog.New(ioutil.Discard, "", stdLog.LstdFlags)),
	)
	if err != nil {
		return fmt.Errorf("creating zipkin exporter: %w", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithConfig(trace.Config{DefaultSampler: trace.TraceIDRatioBased(cfg.Zipkin.Probability)}),
		trace.WithBatcher(
			exporter,
			trace.WithMaxExportBatchSize(trace.DefaultMaxExportBatchSize),
			trace.WithBatchTimeout(trace.DefaultBatchTimeout),
			trace.WithMaxExportBatchSize(trace.DefaultMaxExportBatchSize),
		),
	)
	otel.SetTracerProvider(tp)

	// Start debug service
	//
	// /debug/pprof - added to the default mux by importing the net/http/pprof package.
	// /debug/vars - added to the default mux by importing the expvar package.
	//
	log.Println("main: initializing debugging support")

	metricsExp, err := prometheus.InstallNewPipeline(prometheus.Config{})
	if err != nil {
		return err
	}
	http.HandleFunc("/metrics", metricsExp.ServeHTTP)

	go func() {
		expvar.Publish("goroutines", expvar.Func(func() interface{} {
			return fmt.Sprintf("%d", runtime.NumGoroutine())
		}))
		log.Printf("main: debug listening %s", cfg.Web.DebugHost)
		s := http.Server{
			Addr:    cfg.Web.DebugHost,
			Handler: http.DefaultServeMux,
		}
		if err := s.ListenAndServe(); err != nil {
			log.Printf("main: debug listener closed: %+v", err)
		}
	}()

	// Start watching for config changes for storage systems

	sysViper := viper.New()
	sysViper.SetConfigName("storage-systems")
	sysViper.AddConfigPath(".")
	sysViper.AddConfigPath("/etc/karavi-authorization/storage/")
	sysViper.WatchConfig()

	// Create the Powerflex handler

	powerFlexHandler := proxy.NewPowerFlexHandler(log, enf, cfg.OpenPolicyAgent.Host)
	updaterFn := func() {
		if err := sysViper.ReadInConfig(); err != nil {
			log.Fatalf("reading storage config file: %+v", err)
		}
		v := sysViper.Get("storage")
		b, err := json.Marshal(&v)
		if err != nil {
			log.Errorf("main: failed to marshal config: %+v", err)
			return
		}
		err = powerFlexHandler.UpdateSystems(context.Background(), bytes.NewReader(b))
		if err != nil {
			log.Errorf("main: failed to update system: %+v", err)
			return
		}
	}

	// Update on config changes.
	sysViper.OnConfigChange(func(e fsnotify.Event) {
		log.Printf("Changed! %+v, %s", e.Op, e.Name)
		updaterFn()
	})
	updaterFn()

	// Create the handlers

	systemHandlers := map[string]http.Handler{
		"powerflex": web.Adapt(powerFlexHandler, web.OtelMW(tp, "powerflex"), web.AuthMW(log, cfg.Web.JWTSigningSecret)),
	}
	dh := proxy.NewDispatchHandler(log, systemHandlers)

	insecure := cfg.Certificate.CrtFile == "" && cfg.Certificate.KeyFile == ""

	router := &web.Router{
		RolesHandler: web.Adapt(rolesHandler(), web.OtelMW(tp, "roles")),
		TokenHandler: web.Adapt(refreshTokenHandler(cfg.Web.JWTSigningSecret), web.OtelMW(tp, "refresh")),
		ProxyHandler: web.Adapt(dh, web.OtelMW(tp, "dispatch")),
		ClientInstallScriptHandler: web.Adapt(web.ClientInstallHandler(cfg.Web.SidecarProxyAddr, cfg.Web.JWTSigningSecret, insecure),
			web.OtelMW(tp, "client-installer")),
	}

	// Start the proxy service
	log.Println("main: initializing proxy service")

	svr := http.Server{
		Addr: cfg.Proxy.Host,
		Handler: web.Adapt(router.Handler(),
			web.LoggingMW(log, true), // log all requests
			web.CleanMW(),            // clean paths
			web.OtelMW(tp, "", // format the span name
				otelhttp.WithSpanNameFormatter(func(s string, r *http.Request) string {
					return fmt.Sprintf("%s %s", r.Method, r.URL.Path)
				}))),
		ReadTimeout:  cfg.Proxy.ReadTimeout,
		WriteTimeout: cfg.Proxy.WriteTimeout,
	}

	// Start listening for requests

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("main: proxy listening on %s", cfg.Proxy.Host)
		serverErrors <- svr.ListenAndServe()
	}()

	// Handle graceful shutdown

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		return fmt.Errorf("main: server error: %w", err)
	case sig := <-shutdown:
		log.Printf("main: starting shutdown: %v", sig)
		ctx, cancel := context.WithTimeout(context.Background(), cfg.Web.ShutdownTimeout)
		defer cancel()

		// Ask the proxy to shutdown and shed load
		if err := svr.Shutdown(ctx); err != nil {
			svr.Close()
			return fmt.Errorf("main: failed to gracefully shutdown server: %w", err)
		}
	}

	return nil
}

func refreshTokenHandler(secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO(ian): Establish this connection as part of service initialization.
		conn, err := grpc.Dial("tenant-service.karavi.svc.cluster.local:50051",
			grpc.WithTimeout(10*time.Second),
			grpc.WithInsecure())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		client := pb.NewTenantServiceClient(conn)

		log.Println("Refreshing token!")
		type tokenPair struct {
			RefreshToken string `json:"refreshToken,omitempty"`
			AccessToken  string `json:"accessToken"`
		}
		var input tokenPair
		err = json.NewDecoder(r.Body).Decode(&input)
		if err != nil {
			log.Printf("decoding token pair: %+v", err)
			http.Error(w, "decoding token pair", http.StatusInternalServerError)
			return
		}

		refreshResp, err := client.RefreshToken(r.Context(), &pb.RefreshTokenRequest{
			AccessToken:      input.AccessToken,
			RefreshToken:     input.RefreshToken,
			JWTSigningSecret: secret,
		})
		if err != nil {
			log.Printf("%+v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var output tokenPair
		output.AccessToken = refreshResp.AccessToken
		err = json.NewEncoder(w).Encode(&output)
		if err != nil {
			log.Printf("encoding token pair: %+v", err)
			http.Error(w, "encoding token pair", http.StatusInternalServerError)
			return
		}
	})
}

func rolesHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r, err := http.NewRequest(http.MethodGet, "http://localhost:8181/v1/data/karavi/common/roles", nil)
		if err != nil {
			log.Fatal(err)
		}
		res, err := http.DefaultClient.Do(r)
		if err != nil {
			log.Fatal(err)
		}
		_, err = io.Copy(w, res.Body)
		if err != nil {
			log.Fatal(err)
		}
		defer res.Body.Close()
	})
}
