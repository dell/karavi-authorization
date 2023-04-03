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
	"fmt"
	"io/ioutil"
	"karavi-authorization/internal/k8s"
	storage "karavi-authorization/internal/storage-service"
	"karavi-authorization/internal/storage-service/middleware"
	"karavi-authorization/pb"
	stdLog "log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	listenAddr                  = ":50051"
	namespaceEnv                = "NAMESPACE"
	logLevel                    = "LOG_LEVEL"
	logFormat                   = "LOG_FORMAT"
	concurrentPowerFlexRequests = "CONCURRENT_POWERFLEX_REQUESTS"
)

var (
	cfg Config
)

// Config is the configuration details on the tenant-service
type Config struct {
	GrpcListenAddr string
	Zipkin         struct {
		CollectorURI string
		ServiceName  string
		Probability  float64
	}
}

func main() {
	// define the logger
	log := logrus.NewEntry(logrus.New())

	//declare Config values
	cfgViper := viper.New()
	cfgViper.SetConfigName("config")
	cfgViper.AddConfigPath(".")
	cfgViper.AddConfigPath("/etc/karavi-authorization/config")

	cfgViper.SetDefault("grpclistenaddr", listenAddr)
	cfgViper.SetDefault("zipkin.collectoruri", "http://localhost:9411/api/v2/spans")
	cfgViper.SetDefault("zipkin.servicename", "proxy-server")
	cfgViper.SetDefault("zipkin.probability", 0.8)

	if err := cfgViper.ReadInConfig(); err != nil {
		log.Fatalf("reading config file: %+v", err)
	}
	if err := cfgViper.Unmarshal(&cfg); err != nil {
		log.Fatalf("decoding config file: %+v", err)
	}

	// define the storage service
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Fatal(err)
	}
	k8sClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	ns := os.Getenv(namespaceEnv)

	api := &k8s.API{
		Client:    k8sClient,
		Lock:      sync.Mutex{},
		Namespace: ns,
		Log:       log,
	}

	storageSvc := storage.NewService(api, storage.NewSystemValidator(api, log))

	// read and watch configuration
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
		log.WithField("LOG_LEVEL", level).Info("Configuration updated")
	}
	updateLoggingSettings(log)

	updateConcurrentPowerFlexRequests := func(s *storage.Service, log *logrus.Entry) {
		requests := csmViper.GetString(concurrentPowerFlexRequests)
		n, err := strconv.Atoi(requests)
		if err != nil {
			log.WithError(err).Fatal("CONCURRENT_POWERFLEX_REQUESTS was not set to a valid number")
		}
		s.SetConcurrentPowerFlexRequests(n)
		log.WithField(concurrentPowerFlexRequests, n).Info("Configuration updated")
	}
	updateConcurrentPowerFlexRequests(storageSvc, log)

	csmViper.WatchConfig()
	csmViper.OnConfigChange(func(e fsnotify.Event) {
		updateLoggingSettings(log)
		updateConcurrentPowerFlexRequests(storageSvc, log)
	})

	// Start tracing support

	_, err = initTracing(log,
		cfg.Zipkin.CollectorURI,
		"csm-authorization-storage-service",
		cfg.Zipkin.Probability)
	if err != nil {
		log.WithError(err).Println("main: initializng tracing")
	}

	l, err := net.Listen("tcp", cfg.GrpcListenAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := l.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "closing listener: %+v\n", err)
		}
	}()

	gs := grpc.NewServer(grpc.UnaryInterceptor(otelgrpc.UnaryServerInterceptor()), grpc.StreamInterceptor(otelgrpc.StreamServerInterceptor()))
	pb.RegisterStorageServiceServer(gs, middleware.NewStorageTelemetryMW(log, storageSvc))

	log.Infof("Serving storage service on %s", cfg.GrpcListenAddr)
	log.Fatal(gs.Serve(l))
}

func initTracing(log *logrus.Entry, uri, name string, prob float64) (*trace.TracerProvider, error) {
	if len(strings.TrimSpace(uri)) == 0 {
		return nil, nil
	}

	log.Info("main: initializing otel/zipkin tracing support")

	exporter, err := zipkin.New(
		uri,
		zipkin.WithLogger(stdLog.New(ioutil.Discard, "", stdLog.LstdFlags)),
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
