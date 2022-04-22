package main

import (
	"fmt"
	"karavi-authorization/internal/role-service"
	"karavi-authorization/pb"
	"net"
	"os"

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
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	roleSvc := role.NewService(client, os.Getenv(NAMESPACE), role.WithLogger(log))

	gs := grpc.NewServer()
	pb.RegisterRoleServiceServer(gs, roleSvc)

	log.Infof("Serving role service on %s", LISTEN_ADDRESS)
	log.Fatal(gs.Serve(l))
}
