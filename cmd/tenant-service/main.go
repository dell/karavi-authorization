package main

import (
	"fmt"
	"karavi-authorization/internal/tenantsvc"
	"karavi-authorization/pb"
	"net"
	"os"

	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

const (
	DefaultListenAddr = ":50051"
)

func main() {
	log := logrus.NewEntry(logrus.New())

	l, err := net.Listen("tcp", DefaultListenAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := l.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "closing listener: %+v\n", err)
		}
	}()
	tenantSvc := tenantsvc.NewTenantService(tenantsvc.WithLogger(log))
	gs := grpc.NewServer()
	pb.RegisterTenantServiceServer(gs, tenantSvc)

	log.Println("Serving tenant service on", DefaultListenAddr)
	gs.Serve(l)
}
