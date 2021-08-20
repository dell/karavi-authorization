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
	"karavi-authorization/internal/token/jwx"
	"karavi-authorization/internal/web"
	"karavi-authorization/pb"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"strings"
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

const (
	certificateCrtFile = "certificate.crtfile"
	certificateKeyFile = "certificate.keyfile"
	rootCertFile       = "certificate.rootcertificate"
	sidecarProxyAddr   = "web.sidecarproxyaddr"
	jwtSigningScrt     = "web.jwtsigningsecret"
	logLevel           = "LOG_LEVEL"
	logFormat          = "LOG_FORMAT"
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

func run(log *logrus.Entry) error {
	cfgViper := viper.New()
	cfgViper.SetConfigName("config")
	cfgViper.AddConfigPath(".")
	cfgViper.AddConfigPath("/etc/karavi-authorization/config/")

	cfgViper.SetDefault(certificateCrtFile, "")
	cfgViper.SetDefault(certificateKeyFile, "")

	cfgViper.SetDefault("proxy.host", ":8080")
	cfgViper.SetDefault("proxy.readtimeout", 30*time.Second)
	cfgViper.SetDefault("proxy.writetimeout", 30*time.Second)

	cfgViper.SetDefault("web.debughost", ":9090")
	cfgViper.SetDefault("web.shutdowntimeout", 15*time.Second)
	cfgViper.SetDefault(sidecarProxyAddr, web.DefaultSidecarProxyAddr)
	cfgViper.SetDefault(jwtSigningScrt, "secret")
	cfgViper.SetDefault("web.showdebughttp", false)

	cfgViper.SetDefault("zipkin.collectoruri", "")
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

	web.RootCertificate = cfg.Certificate.RootCertificate
	web.Insecure = cfg.Certificate.CrtFile == "" && cfg.Certificate.KeyFile == ""
	web.SidecarProxyAddr = cfg.Web.SidecarProxyAddr
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
		logFormat := csmViper.GetString(logFormat)
		if strings.EqualFold(logFormat, "json") {
			log.Logger.SetFormatter(&logrus.JSONFormatter{})
		} else {
			// use text formatter by default
			log.Logger.SetFormatter(&logrus.TextFormatter{})
		}
		log.WithField(logFormat, logFormat).Info("configuration has been set.")

		logLevel := csmViper.GetString(logLevel)
		level, err := logrus.ParseLevel(logLevel)
		if err != nil {
			// use INFO level by default
			level = logrus.InfoLevel
		}
		log.Logger.SetLevel(level)
		log.WithField(logLevel, level).Info("configuration has been set.")
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

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Database.Host, // "redis.karavi.svc.cluster.local:6379",
		Password: cfg.Database.Password,
		DB:       0,
	})
	defer func() {
		if err := rdb.Close(); err != nil {
			log.WithError(err).Warn("closing redis")
		}
	}()
	enf := quota.NewRedisEnforcement(context.Background(), quota.WithRedis(rdb))

	// Start tracing support

	tp, err := initTracing(log,
		cfg.Zipkin.CollectorURI,
		cfg.Zipkin.ServiceName,
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

	metricsExp, err := prometheus.InstallNewPipeline(prometheus.Config{})
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
			Addr:    cfg.Web.DebugHost,
			Handler: http.DefaultServeMux,
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
	powerFlexHandler := proxy.NewPowerFlexHandler(log, enf, cfg.OpenPolicyAgent.Host)
	powerMaxHandler := proxy.NewPowerMaxHandler(log, enf, cfg.OpenPolicyAgent.Host)
	powerScaleHandler := proxy.NewPowerScaleHandler(log, enf, cfg.OpenPolicyAgent.Host)

	updaterFn := func() {
		if err := sysViper.ReadInConfig(); err != nil {
			log.WithError(err).Fatal("main: reading storage config file")
		}
		v := sysViper.Get("storage")
		b, err := json.Marshal(&v)
		if err != nil {
			log.WithError(err).Error("main: marshaling config")
			return
		}
		err = powerFlexHandler.UpdateSystems(context.Background(), bytes.NewReader(b), log)
		if err != nil {
			log.WithError(err).Error("main: updating powerflex systems")
			return
		}
		err = powerMaxHandler.UpdateSystems(context.Background(), bytes.NewReader(b), log)
		if err != nil {
			log.WithError(err).Error("main: updating powermax systems")
			return
		}

		err = powerScaleHandler.UpdateSystems(context.Background(), bytes.NewReader(b), log)
		if err != nil {
			log.WithError(err).Error("main: updating powerscale systems")
			return
		}
		err = powerScaleHandler.UpdateSystems(context.Background(), bytes.NewReader(b), log)
		if err != nil {
			log.Errorf("main: failed to update system: %+v", err)
			return
		}
		err = powerScaleHandler.UpdateSystems(context.Background(), bytes.NewReader(b), log)
		if err != nil {
			log.Errorf("main: failed to update system: %+v", err)
			return
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
		"powerflex":  web.Adapt(powerFlexHandler, web.OtelMW(tp, "powerflex"), web.AuthMW(log, jwx.NewTokenManager(jwx.HS256))),
		"powermax":   web.Adapt(powerMaxHandler, web.OtelMW(tp, "powermax"), web.AuthMW(log, jwx.NewTokenManager(jwx.HS256))),
		"powerscale": web.Adapt(powerScaleHandler, web.OtelMW(tp, "powerscale"), web.AuthMW(log, jwx.NewTokenManager(jwx.HS256))),
	}
	dh := proxy.NewDispatchHandler(log, systemHandlers)

	router := &web.Router{
		RolesHandler: web.Adapt(rolesHandler(log), web.OtelMW(tp, "roles")),
		TokenHandler: web.Adapt(refreshTokenHandler(log), web.OtelMW(tp, "refresh")),
		ProxyHandler: web.Adapt(dh, web.OtelMW(tp, "dispatch")),
		ClientInstallScriptHandler: web.Adapt(web.ClientInstallHandler(),
			web.OtelMW(tp, "client-installer")),
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
				}))),
		ReadTimeout:  cfg.Proxy.ReadTimeout,
		WriteTimeout: cfg.Proxy.WriteTimeout,
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
	crtFile := cfg.Certificate.CrtFile
	if vc.IsSet(certificateCrtFile) {
		value := vc.GetString(certificateCrtFile)
		crtFile = value
		if crtFile != "" {
			log.WithField(certificateCrtFile, crtFile).Info("configuration has been set.")
		}
	}

	keyFile := cfg.Certificate.KeyFile
	if vc.IsSet(certificateKeyFile) {
		value := vc.GetString(certificateKeyFile)
		keyFile = value
		if keyFile != "" {
			log.WithField(certificateKeyFile, keyFile).Info("configuration has been set.")
		}
	}
	web.Insecure = crtFile == "" && keyFile == ""

	rootCAFile := cfg.Certificate.RootCertificate
	if vc.IsSet(rootCertFile) {
		value := vc.GetString(rootCertFile)
		rootCAFile = value
		if rootCertFile != "" {
			log.WithField(rootCertFile, rootCAFile).Info("configuration has been set.")
		}
	}
	web.RootCertificate = rootCAFile

	spa := cfg.Web.SidecarProxyAddr
	if vc.IsSet(sidecarProxyAddr) {
		value := vc.GetString(sidecarProxyAddr)
		spa = value
		log.WithField(sidecarProxyAddr, sidecarProxyAddr).Info("configuration has been set.")
	}
	web.SidecarProxyAddr = spa

	jss := cfg.Web.JWTSigningSecret
	if vc.IsSet(jwtSigningScrt) {
		value := vc.GetString(jwtSigningScrt)
		jss = value
		log.WithField(jwtSigningScrt, jwtSigningScrt).Info("configuration has been set.")
	}
	web.JWTSigningSecret = jss
	JWTSigningSecret = jss
}

func initTracing(log *logrus.Entry, uri, name string, prob float64) (*trace.TracerProvider, error) {
	if len(strings.TrimSpace(uri)) == 0 {
		return nil, nil
	}

	log.Info("main: initializing otel/zipkin tracing support")

	exporter, err := zipkin.NewRawExporter(
		uri,
		name,
		zipkin.WithLogger(stdLog.New(ioutil.Discard, "", stdLog.LstdFlags)),
	)
	if err != nil {
		return nil, fmt.Errorf("creating zipkin exporter: %w", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithConfig(trace.Config{DefaultSampler: trace.TraceIDRatioBased(prob)}),
		trace.WithBatcher(
			exporter,
			trace.WithMaxExportBatchSize(trace.DefaultMaxExportBatchSize),
			trace.WithBatchTimeout(trace.DefaultBatchTimeout),
			trace.WithMaxExportBatchSize(trace.DefaultMaxExportBatchSize),
		),
	)
	otel.SetTracerProvider(tp)

	return tp, nil
}

func refreshTokenHandler(log *logrus.Entry) http.Handler {
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

		log.Info("Refreshing token!")
		type tokenPair struct {
			RefreshToken string `json:"refreshToken,omitempty"`
			AccessToken  string `json:"accessToken"`
		}
		var input tokenPair
		err = json.NewDecoder(r.Body).Decode(&input)
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

func rolesHandler(log *logrus.Entry) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r, err := http.NewRequest(http.MethodGet, "http://localhost:8181/v1/data/karavi/common/roles", nil)
		if err != nil {
			log.WithError(err).Fatal()
		}
		res, err := http.DefaultClient.Do(r)
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
