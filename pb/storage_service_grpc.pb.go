// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.20.1
// source: pb/storage_service.proto

package pb

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// StorageServiceClient is the client API for StorageService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type StorageServiceClient interface {
	Create(ctx context.Context, in *StorageCreateRequest, opts ...grpc.CallOption) (*StorageCreateResponse, error)
	List(ctx context.Context, in *StorageListRequest, opts ...grpc.CallOption) (*StorageListResponse, error)
	Update(ctx context.Context, in *StorageUpdateRequest, opts ...grpc.CallOption) (*StorageUpdateResponse, error)
	Delete(ctx context.Context, in *StorageDeleteRequest, opts ...grpc.CallOption) (*StorageDeleteResponse, error)
	Get(ctx context.Context, in *StorageGetRequest, opts ...grpc.CallOption) (*StorageGetResponse, error)
}

type storageServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewStorageServiceClient(cc grpc.ClientConnInterface) StorageServiceClient {
	return &storageServiceClient{cc}
}

func (c *storageServiceClient) Create(ctx context.Context, in *StorageCreateRequest, opts ...grpc.CallOption) (*StorageCreateResponse, error) {
	out := new(StorageCreateResponse)
	err := c.cc.Invoke(ctx, "/karavi.StorageService/Create", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *storageServiceClient) List(ctx context.Context, in *StorageListRequest, opts ...grpc.CallOption) (*StorageListResponse, error) {
	out := new(StorageListResponse)
	err := c.cc.Invoke(ctx, "/karavi.StorageService/List", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *storageServiceClient) Update(ctx context.Context, in *StorageUpdateRequest, opts ...grpc.CallOption) (*StorageUpdateResponse, error) {
	out := new(StorageUpdateResponse)
	err := c.cc.Invoke(ctx, "/karavi.StorageService/Update", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *storageServiceClient) Delete(ctx context.Context, in *StorageDeleteRequest, opts ...grpc.CallOption) (*StorageDeleteResponse, error) {
	out := new(StorageDeleteResponse)
	err := c.cc.Invoke(ctx, "/karavi.StorageService/Delete", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *storageServiceClient) Get(ctx context.Context, in *StorageGetRequest, opts ...grpc.CallOption) (*StorageGetResponse, error) {
	out := new(StorageGetResponse)
	err := c.cc.Invoke(ctx, "/karavi.StorageService/Get", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StorageServiceServer is the server API for StorageService service.
// All implementations must embed UnimplementedStorageServiceServer
// for forward compatibility
type StorageServiceServer interface {
	Create(context.Context, *StorageCreateRequest) (*StorageCreateResponse, error)
	List(context.Context, *StorageListRequest) (*StorageListResponse, error)
	Update(context.Context, *StorageUpdateRequest) (*StorageUpdateResponse, error)
	Delete(context.Context, *StorageDeleteRequest) (*StorageDeleteResponse, error)
	Get(context.Context, *StorageGetRequest) (*StorageGetResponse, error)
	mustEmbedUnimplementedStorageServiceServer()
}

// UnimplementedStorageServiceServer must be embedded to have forward compatible implementations.
type UnimplementedStorageServiceServer struct {
}

func (UnimplementedStorageServiceServer) Create(context.Context, *StorageCreateRequest) (*StorageCreateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Create not implemented")
}
func (UnimplementedStorageServiceServer) List(context.Context, *StorageListRequest) (*StorageListResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method List not implemented")
}
func (UnimplementedStorageServiceServer) Update(context.Context, *StorageUpdateRequest) (*StorageUpdateResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Update not implemented")
}
func (UnimplementedStorageServiceServer) Delete(context.Context, *StorageDeleteRequest) (*StorageDeleteResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Delete not implemented")
}
func (UnimplementedStorageServiceServer) Get(context.Context, *StorageGetRequest) (*StorageGetResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Get not implemented")
}
func (UnimplementedStorageServiceServer) mustEmbedUnimplementedStorageServiceServer() {}

// UnsafeStorageServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to StorageServiceServer will
// result in compilation errors.
type UnsafeStorageServiceServer interface {
	mustEmbedUnimplementedStorageServiceServer()
}

func RegisterStorageServiceServer(s grpc.ServiceRegistrar, srv StorageServiceServer) {
	s.RegisterService(&StorageService_ServiceDesc, srv)
}

func _StorageService_Create_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StorageCreateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StorageServiceServer).Create(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/karavi.StorageService/Create",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StorageServiceServer).Create(ctx, req.(*StorageCreateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _StorageService_List_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StorageListRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StorageServiceServer).List(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/karavi.StorageService/List",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StorageServiceServer).List(ctx, req.(*StorageListRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _StorageService_Update_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StorageUpdateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StorageServiceServer).Update(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/karavi.StorageService/Update",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StorageServiceServer).Update(ctx, req.(*StorageUpdateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _StorageService_Delete_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StorageDeleteRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StorageServiceServer).Delete(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/karavi.StorageService/Delete",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StorageServiceServer).Delete(ctx, req.(*StorageDeleteRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _StorageService_Get_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StorageGetRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StorageServiceServer).Get(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/karavi.StorageService/Get",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StorageServiceServer).Get(ctx, req.(*StorageGetRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// StorageService_ServiceDesc is the grpc.ServiceDesc for StorageService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var StorageService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "karavi.StorageService",
	HandlerType: (*StorageServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Create",
			Handler:    _StorageService_Create_Handler,
		},
		{
			MethodName: "List",
			Handler:    _StorageService_List_Handler,
		},
		{
			MethodName: "Update",
			Handler:    _StorageService_Update_Handler,
		},
		{
			MethodName: "Delete",
			Handler:    _StorageService_Delete_Handler,
		},
		{
			MethodName: "Get",
			Handler:    _StorageService_Get_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "pb/storage_service.proto",
}