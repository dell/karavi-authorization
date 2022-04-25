package main

import (
	"flag"
	"fmt"
	"karavi-authorization/internal/role-service"
	"karavi-authorization/internal/role-service/k8s"
	"karavi-authorization/internal/role-service/validate"
	"karavi-authorization/pb"
	"net"
	"os"
	"sync"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	LISTEN_ADDRESS = "50052"
	NAMESPACE      = "NAMESPACE"
)

func main() {
	namespace := flag.String("namespace", "", "namespace of helm deployment")
	flag.Parse()

	log := logrus.NewEntry(logrus.New())

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

	roleSvc := role.NewService(api, validate.NewRoleValidator(k8sClient, *namespace))

	gs := grpc.NewServer()
	pb.RegisterRoleServiceServer(gs, roleSvc)

	log.Infof("Serving role service on %s", LISTEN_ADDRESS)
	log.Fatal(gs.Serve(l))
}
