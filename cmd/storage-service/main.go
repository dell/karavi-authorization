// Copyright © 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"karavi-authorization/internal/k8s"
	"karavi-authorization/internal/storage-service"
	"karavi-authorization/internal/storage-service/validate"
	"karavi-authorization/pb"
	"net"
	"os"
	"strings"
	"sync"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	listenAddr   = ":50051"
	namespaceEnv = "NAMESPACE"
	logLevel     = "LOG_LEVEL"
	logFormat    = "LOG_FORMAT"
)

func main() {
	log := logrus.NewEntry(logrus.New())

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

	addr := struct {
		address string
	}{
		listenAddr,
	}

	l, err := net.Listen("tcp", addr.address)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := l.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "closing listener: %+v\n", err)
		}
	}()

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

	storageSvc := storage.NewService(api, validate.NewStorageValidator(api, log))

	gs := grpc.NewServer()
	pb.RegisterStorageServiceServer(gs, storageSvc)

	log.Infof("Serving storage service on %s", listenAddr)
	log.Fatal(gs.Serve(l))
}
