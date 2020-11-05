package main

import (
	"context"
	"net"
	"powerflex-reverse-proxy/pb"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestConnection(t *testing.T) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := lis.Close(); err != nil {
			t.Logf("closing listener: %+v", err)
		}
	}()

	s := grpc.NewServer()
	pb.RegisterAuthServiceServer(s, &mockGatekeeper{})
	go func() {
		if err := s.Serve(lis); err != nil {
			t.Fatalf("serve: %+v", err)
		}
	}()

	t.Logf("Using address: %v", lis.Addr().String())

	err = run([]string{"app", "-address", lis.Addr().String()})
	if err != nil {
		t.Fatal(err)
	}
}

type mockGatekeeper struct{}

func (m *mockGatekeeper) Login(_ *pb.LoginRequest, _ pb.AuthService_LoginServer) error {
	return status.Errorf(codes.Internal, "testing")
}

func (m *mockGatekeeper) Refresh(_ context.Context, _ *pb.RefreshRequest) (*pb.RefreshResponse, error) {
	panic("not implemented") // TODO: Implement
}
