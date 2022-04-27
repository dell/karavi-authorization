package main

import (
	"fmt"
	"karavi-authorization/internal/k8s"
	"karavi-authorization/internal/role-service"
	"karavi-authorization/internal/role-service/validate"
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

	l, err := net.Listen("tcp", listenAddr)
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

	roleSvc := role.NewService(api, validate.NewRoleValidator(api, log))

	gs := grpc.NewServer()
	pb.RegisterRoleServiceServer(gs, roleSvc)

	log.Infof("Serving role service on %s", listenAddr)
	log.Fatal(gs.Serve(l))
}
