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
	"flag"
	"fmt"
	"karavi-authorization/internal/tenantsvc"
	"karavi-authorization/internal/token/jwx"
	"karavi-authorization/pb"
	"net"
	"os"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

const (
	logLevel  = "LOG_LEVEL"
	logFormat = "LOG_FORMAT"
)

var (
	cfg Config
)

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
	cfgViper.OnConfigChange(func(e fsnotify.Event) {
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
	csmViper.OnConfigChange(func(e fsnotify.Event) {
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
	gs := grpc.NewServer()
	pb.RegisterTenantServiceServer(gs, tenantSvc)

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
