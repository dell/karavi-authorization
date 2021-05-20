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
	"fmt"
	"karavi-authorization/internal/tenantsvc"
	"karavi-authorization/internal/token/jwx"
	"karavi-authorization/pb"
	"log"
	"net"
	"os"
	"time"

	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
)

func main() {
	var cfg struct {
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

	log.Printf("Config: %+v", cfg)

	// Initialize the logger
	log := logrus.NewEntry(logrus.New())

	// Initialize the database connection

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

	l, err := net.Listen("tcp", cfg.GrpcListenAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := l.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "closing listener: %+v\n", err)
		}
	}()

	tenantSvc := tenantsvc.NewTenantService(
		tenantsvc.WithLogger(log),
		tenantsvc.WithRedis(rdb),
		tenantsvc.WithJWTSigningSecret(cfg.Web.JWTSigningSecret),
		tenantsvc.WithTokenManager(jwx.NewTokenManager(jwx.HS256)))
	gs := grpc.NewServer()
	pb.RegisterTenantServiceServer(gs, tenantSvc)

	log.Println("Serving tenant service on", cfg.GrpcListenAddr)
	log.Fatal(gs.Serve(l))
}
