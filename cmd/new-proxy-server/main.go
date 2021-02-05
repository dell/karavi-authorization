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
	"context"
	"crypto/tls"
	"expvar"
	"fmt"
	"io/ioutil"
	"karavi-authorization/internal/config"
	"karavi-authorization/internal/proxy"
	"karavi-authorization/internal/quota"
	"karavi-authorization/internal/web"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"syscall"
	"time"

	stdLog "log"

	"github.com/fsnotify/fsnotify"
	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/metric/prometheus"
	"go.opentelemetry.io/otel/exporters/trace/zipkin"
	"go.opentelemetry.io/otel/sdk/trace"
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
		Proxy struct {
			Host              string
			SystemsConfigPath string
			ReadTimeout       time.Duration
			WriteTimeout      time.Duration
		}
		Web struct {
			DebugHost       string
			ShutdownTimeout time.Duration
		}
		Database struct {
			Host     string
			Password string
		}
		OpenPolicyAgent struct {
			Host string
		}
	}

	// Initializing application
	cfg.Proxy.Host = ":8080"
	if v, ok := os.LookupEnv("KARAVI_SYSTEMS_CONFIG"); ok {
		cfg.Proxy.SystemsConfigPath = v
	} else {
		cfg.Proxy.SystemsConfigPath = "/etc/karavi/systems.json"
	}
	cfg.Proxy.ReadTimeout = 30 * time.Second
	cfg.Proxy.WriteTimeout = 30 * time.Second

	cfg.Web.DebugHost = ":9090"
	cfg.Web.ShutdownTimeout = 15 * time.Second

	cfg.Zipkin.CollectorURI = "http://localhost:9411/api/v2/spans"
	cfg.Zipkin.ServiceName = "proxy-server"
	cfg.Zipkin.Probability = 0.8

	cfg.Version = build
	expvar.NewString("build").Set(build)

	log.Printf("main: started application version %q", build)
	defer log.Println("main: stopped application")

	// Initialize authentication

	// Initialize OPA

	if v, ok := os.LookupEnv("KARAVI_OPA_HOST"); ok {
		cfg.OpenPolicyAgent.Host = v
	}

	// Initialize database connections

	if v, ok := os.LookupEnv("KARAVI_REDIS_HOST"); ok {
		cfg.Database.Host = v
	}
	if v, ok := os.LookupEnv("KARAVI_REDIS_PASSWORD"); ok {
		cfg.Database.Password = v
	}
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
		//trace.WithConfig(trace.Config{DefaultSampler: trace.TraceIDRatioBased(cfg.Zipkin.Probability)}),
		trace.WithConfig(trace.Config{DefaultSampler: trace.AlwaysSample()}),
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
		log.Printf("main: debug listening %s", cfg.Web.DebugHost)
		s := http.Server{
			Addr:    cfg.Web.DebugHost,
			Handler: http.DefaultServeMux,
			TLSConfig: &tls.Config{
				//Certificates:       nil,
				InsecureSkipVerify: true,
			},
		}
		if err := s.ListenAndServeTLS("cert.pem", "key.pem"); err != nil {
			log.Printf("main: debug listener closed: %+v", err)
		}
	}()

	// Start watching for config changes for storage systems

	sysConfig := config.New(log)
	sysConfig.SetPath(cfg.Proxy.SystemsConfigPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sysConfig.Watch(ctx)

	// Create the Powerflex handler

	powerFlexHandler := proxy.NewPowerFlexHandler(log, enf, cfg.OpenPolicyAgent.Host)
	updaterFn := func() {
		r, err := sysConfig.ReadConfig()
		if err != nil {
			log.Errorf("main: failed read config: %+v", err)
			return
		}
		err = powerFlexHandler.UpdateSystems(r)
		if err != nil {
			log.Errorf("main: failed to update system: %+v", err)
			return
		}
	}
	// Update on config changes.
	sysConfig.OnChange(func(e fsnotify.Event) {
		log.Printf("Changed! %+v, %s", e.Op, e.Name)
		updaterFn()
	})
	// Update immediately
	updaterFn()

	// Create the handlers

	systemHandlers := map[string]http.Handler{
		"powerflex": web.Adapt(powerFlexHandler, web.OtelMW(tp, "powerflex"), web.AuthMW(log)),
	}
	dh := proxy.NewDispatchHandler(log, systemHandlers)

	todoHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	})
	router := &web.Router{
		PolicyHandler: todoHandler,
		RolesHandler:  todoHandler,
		TokenHandler:  todoHandler,
		ProxyHandler:  web.Adapt(dh, web.OtelMW(tp, "dispatch")),
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
		serverErrors <- svr.ListenAndServeTLS("cert.pem", "key.pem")
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
