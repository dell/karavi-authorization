package cmd

import (
	"context"
	"karavi-authorization/pb"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
)

type fakeTenantServiceClient struct {
	pb.TenantServiceClient
	CreateTenantFn func(context.Context, *pb.CreateTenantRequest, ...grpc.CallOption) (*pb.Tenant, error)
	GetTenantFn    func(context.Context, *pb.GetTenantRequest, ...grpc.CallOption) (*pb.Tenant, error)
	DeleteTenantFn func(context.Context, *pb.DeleteTenantRequest, ...grpc.CallOption) (*empty.Empty, error)
	ListTenantFn   func(context.Context, *pb.ListTenantRequest, ...grpc.CallOption) (*pb.ListTenantResponse, error)
	BindRoleFn     func(context.Context, *pb.BindRoleRequest, ...grpc.CallOption) (*pb.BindRoleResponse, error)
	UnbindRoleFn   func(context.Context, *pb.UnbindRoleRequest, ...grpc.CallOption) (*pb.UnbindRoleResponse, error)
}

func (f *fakeTenantServiceClient) CreateTenant(ctx context.Context, in *pb.CreateTenantRequest, opts ...grpc.CallOption) (*pb.Tenant, error) {
	if f.CreateTenantFn != nil {
		return f.CreateTenantFn(ctx, in, opts...)
	}
	return &pb.Tenant{
		Name: "testname",
	}, nil
}

func (f *fakeTenantServiceClient) GetTenant(ctx context.Context, in *pb.GetTenantRequest, opts ...grpc.CallOption) (*pb.Tenant, error) {
	if f.GetTenantFn != nil {
		return f.GetTenantFn(ctx, in, opts...)
	}
	return &pb.Tenant{
		Name: "testname",
	}, nil
}

func (f *fakeTenantServiceClient) UpdateTenant(ctx context.Context, in *pb.UpdateTenantRequest, opts ...grpc.CallOption) (*pb.Tenant, error) {
	panic("not implemented") // TODO: Implement
}

func (f *fakeTenantServiceClient) DeleteTenant(ctx context.Context, in *pb.DeleteTenantRequest, opts ...grpc.CallOption) (*empty.Empty, error) {
	if f.DeleteTenantFn != nil {
		return f.DeleteTenantFn(ctx, in, opts...)
	}
	return &empty.Empty{}, nil
}

func (f *fakeTenantServiceClient) ListTenant(ctx context.Context, in *pb.ListTenantRequest, opts ...grpc.CallOption) (*pb.ListTenantResponse, error) {
	if f.ListTenantFn != nil {
		return f.ListTenantFn(ctx, in, opts...)
	}
	return &pb.ListTenantResponse{}, nil
}

func (f *fakeTenantServiceClient) BindRole(ctx context.Context, in *pb.BindRoleRequest, opts ...grpc.CallOption) (*pb.BindRoleResponse, error) {
	if f.BindRoleFn != nil {
		return f.BindRoleFn(ctx, in, opts...)
	}
	return &pb.BindRoleResponse{}, nil
}

func (f *fakeTenantServiceClient) UnbindRole(ctx context.Context, in *pb.UnbindRoleRequest, opts ...grpc.CallOption) (*pb.UnbindRoleResponse, error) {
	if f.UnbindRoleFn != nil {
		return f.UnbindRoleFn(ctx, in, opts...)
	}
	return &pb.UnbindRoleResponse{}, nil
}
