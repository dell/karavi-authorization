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
	"flag"
	"fmt"
	"io"
	"karavi-authorization/internal/tenantsvc"
	"karavi-authorization/internal/tenantsvc/middleware"
	"karavi-authorization/internal/token/jwx"
	"karavi-authorization/pb"
	"net"
	"os"
	"strings"
	"time"

	stdLog "log"

	"github.com/fsnotify/fsnotify"
	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"google.golang.org/grpc"
)

const (
	logLevel  = "LOG_LEVEL"
	logFormat = "LOG_FORMAT"
)

var cfg Config

// Config is the configuration details on the tenant-service
type Config struct {
	GrpcListenAddr string
	Version        string
	Zipkin         struct {
		CollectorURI string
		ServiceName  string
		Probability  float64
	}
	Web struct {
		DebugHost        string
		ShutdownTimeout  time.Duration
		JWTSigningSecret string
	}
	Database struct {
		Host     string
		Password string
	}
}

func main() {
	log := logrus.NewEntry(logrus.New())

	redisHost := flag.String("redis-host", "", "address of redis host")
	flag.Parse()

	cfgViper := viper.New()
	cfgViper.SetConfigName("config")
	cfgViper.AddConfigPath(".")
	cfgViper.AddConfigPath("/etc/karavi-authorization/config/")

	cfgViper.SetDefault("grpclistenaddr", ":50051")

	cfgViper.SetDefault("web.debughost", ":9090")
	cfgViper.SetDefault("web.shutdowntimeout", 15*time.Second)
	cfgViper.SetDefault("web.jwtsigningsecret", "secret")

	cfgViper.SetDefault("zipkin.collectoruri", "http://localhost:9411/api/v2/spans")
	cfgViper.SetDefault("zipkin.servicename", "proxy-server")
	cfgViper.SetDefault("zipkin.probability", 0.8)

	cfgViper.SetDefault("database.host", "redis.karavi.svc.cluster.local:6379")
	cfgViper.SetDefault("database.password", "")

	if err := cfgViper.ReadInConfig(); err != nil {
		log.Fatalf("reading config file: %+v", err)
	}
	if err := cfgViper.Unmarshal(&cfg); err != nil {
		log.Fatalf("decoding config file: %+v", err)
	}

	cfgViper.WatchConfig()
	cfgViper.OnConfigChange(func(_ fsnotify.Event) {
		updateConfiguration(cfgViper, log)
	})

	log.Infof("Config: %+v", cfg)

	csmViper := viper.New()
	csmViper.SetConfigName("csm-config-params")
	csmViper.AddConfigPath("/etc/karavi-authorization/csm-config-params/")

	if err := csmViper.ReadInConfig(); err != nil {
		log.Fatalf("reading config file: %+v", err)
	}

	updateLoggingSettings := func(log *logrus.Entry) {
		logFormat := csmViper.GetString(logFormat)
		if strings.EqualFold(logFormat, "json") {
			log.Logger.SetFormatter(&logrus.JSONFormatter{})
		} else {
			// use text formatter by default
			log.Logger.SetFormatter(&logrus.TextFormatter{})
		}
		logLevel := csmViper.GetString(logLevel)
		level, err := logrus.ParseLevel(logLevel)
		if err != nil {
			// use INFO level by default
			level = logrus.InfoLevel
		}
		log.Logger.SetLevel(level)
	}
	updateLoggingSettings(log)

	csmViper.WatchConfig()
	csmViper.OnConfigChange(func(_ fsnotify.Event) {
		log.Info("csm-config-params changed!")
		updateLoggingSettings(log)
	})

	// Initialize the database connection

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
			log.Printf("closing redis: %+v", err)
		}
	}()

	// Start tracing support

	_, err := initTracing(log,
		cfg.Zipkin.CollectorURI,
		"csm-authorization-tenant-service",
		cfg.Zipkin.Probability)
	if err != nil {
		log.WithError(err).Println("main: initializng tracing")
	}

	// Start the server

	l, err := net.Listen("tcp", cfg.GrpcListenAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := l.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "closing listener: %+v\n", err)
		}
	}()

	tenantsvc.JWTSigningSecret = cfg.Web.JWTSigningSecret
	tenantSvc := tenantsvc.NewTenantService(
		tenantsvc.WithLogger(log),
		tenantsvc.WithRedis(rdb),
		tenantsvc.WithTokenManager(jwx.NewTokenManager(jwx.HS256)))
	gs := grpc.NewServer(grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()), grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()))
	pb.RegisterTenantServiceServer(gs, middleware.NewTelemetryMW(log, tenantSvc))

	log.Infof("Serving tenant service on %s", cfg.GrpcListenAddr)
	log.Fatal(gs.Serve(l))
}

func updateConfiguration(vc *viper.Viper, log *logrus.Entry) {
	jwtSigningSecret := cfg.Web.JWTSigningSecret
	if vc.IsSet("web.jwtsigningsecret") {
		value := vc.GetString("web.jwtsigningsecret")
		jwtSigningSecret = value
		log.WithField("web.jwtsigningsecret", "***").Info("configuration has been set.")
	}
	tenantsvc.JWTSigningSecret = jwtSigningSecret
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
			trace.WithBatchTimeout(trace.DefaultScheduleDelay),
			trace.WithMaxExportBatchSize(trace.DefaultMaxExportBatchSize),
		),
		trace.WithResource(resource.NewWithAttributes(semconv.SchemaURL,
			attribute.KeyValue{Key: semconv.ServiceNameKey, Value: attribute.StringValue(name)})),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}))

	return tp, nil
}
