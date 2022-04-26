package main

import (
	"flag"
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
	LISTEN_ADDRESS = "50051"
	NAMESPACE      = "NAMESPACE"
	logLevel       = "LOG_LEVEL"
	logFormat      = "LOG_FORMAT"
)

func main() {
	namespace := flag.String("namespace", "", "namespace of helm deployment")
	flag.Parse()

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
	}
	updateLoggingSettings(log)

	l, err := net.Listen("tcp", LISTEN_ADDRESS)
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

	api := &k8s.API{
		Client:    k8sClient,
		Lock:      sync.Mutex{},
		Namespace: os.Getenv(NAMESPACE),
	}

	roleSvc := role.NewService(api, validate.NewRoleValidator(api, log, *namespace), log)

	gs := grpc.NewServer()
	pb.RegisterRoleServiceServer(gs, roleSvc)

	log.Infof("Serving role service on %s", LISTEN_ADDRESS)
	log.Fatal(gs.Serve(l))
}
