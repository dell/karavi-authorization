// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0
// 	protoc        v3.13.0
// source: pb/tenant_service.proto

package pb

import (
	proto "github.com/golang/protobuf/proto"
	empty "github.com/golang/protobuf/ptypes/empty"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

type Tenant struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name  string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
	Roles string `protobuf:"bytes,2,opt,name=roles,proto3" json:"roles,omitempty"`
}

func (x *Tenant) Reset() {
	*x = Tenant{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pb_tenant_service_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Tenant) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Tenant) ProtoMessage() {}

func (x *Tenant) ProtoReflect() protoreflect.Message {
	mi := &file_pb_tenant_service_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Tenant.ProtoReflect.Descriptor instead.
func (*Tenant) Descriptor() ([]byte, []int) {
	return file_pb_tenant_service_proto_rawDescGZIP(), []int{0}
}

func (x *Tenant) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

func (x *Tenant) GetRoles() string {
	if x != nil {
		return x.Roles
	}
	return ""
}

type CreateTenantRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Tenant *Tenant `protobuf:"bytes,1,opt,name=tenant,proto3" json:"tenant,omitempty"`
}

func (x *CreateTenantRequest) Reset() {
	*x = CreateTenantRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pb_tenant_service_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CreateTenantRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CreateTenantRequest) ProtoMessage() {}

func (x *CreateTenantRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pb_tenant_service_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CreateTenantRequest.ProtoReflect.Descriptor instead.
func (*CreateTenantRequest) Descriptor() ([]byte, []int) {
	return file_pb_tenant_service_proto_rawDescGZIP(), []int{1}
}

func (x *CreateTenantRequest) GetTenant() *Tenant {
	if x != nil {
		return x.Tenant
	}
	return nil
}

type GetTenantRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *GetTenantRequest) Reset() {
	*x = GetTenantRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pb_tenant_service_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetTenantRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetTenantRequest) ProtoMessage() {}

func (x *GetTenantRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pb_tenant_service_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetTenantRequest.ProtoReflect.Descriptor instead.
func (*GetTenantRequest) Descriptor() ([]byte, []int) {
	return file_pb_tenant_service_proto_rawDescGZIP(), []int{2}
}

func (x *GetTenantRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type DeleteTenantRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *DeleteTenantRequest) Reset() {
	*x = DeleteTenantRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pb_tenant_service_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DeleteTenantRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DeleteTenantRequest) ProtoMessage() {}

func (x *DeleteTenantRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pb_tenant_service_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DeleteTenantRequest.ProtoReflect.Descriptor instead.
func (*DeleteTenantRequest) Descriptor() ([]byte, []int) {
	return file_pb_tenant_service_proto_rawDescGZIP(), []int{3}
}

func (x *DeleteTenantRequest) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

type ListTenantRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PageSize  int32 `protobuf:"varint,1,opt,name=page_size,json=pageSize,proto3" json:"page_size,omitempty"`
	PageToken int32 `protobuf:"varint,2,opt,name=page_token,json=pageToken,proto3" json:"page_token,omitempty"`
}

func (x *ListTenantRequest) Reset() {
	*x = ListTenantRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pb_tenant_service_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListTenantRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListTenantRequest) ProtoMessage() {}

func (x *ListTenantRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pb_tenant_service_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListTenantRequest.ProtoReflect.Descriptor instead.
func (*ListTenantRequest) Descriptor() ([]byte, []int) {
	return file_pb_tenant_service_proto_rawDescGZIP(), []int{4}
}

func (x *ListTenantRequest) GetPageSize() int32 {
	if x != nil {
		return x.PageSize
	}
	return 0
}

func (x *ListTenantRequest) GetPageToken() int32 {
	if x != nil {
		return x.PageToken
	}
	return 0
}

type ListTenantResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Tenants       []*Tenant `protobuf:"bytes,1,rep,name=tenants,proto3" json:"tenants,omitempty"`
	NextPageToken string    `protobuf:"bytes,2,opt,name=next_page_token,json=nextPageToken,proto3" json:"next_page_token,omitempty"`
}

func (x *ListTenantResponse) Reset() {
	*x = ListTenantResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pb_tenant_service_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListTenantResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListTenantResponse) ProtoMessage() {}

func (x *ListTenantResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pb_tenant_service_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListTenantResponse.ProtoReflect.Descriptor instead.
func (*ListTenantResponse) Descriptor() ([]byte, []int) {
	return file_pb_tenant_service_proto_rawDescGZIP(), []int{5}
}

func (x *ListTenantResponse) GetTenants() []*Tenant {
	if x != nil {
		return x.Tenants
	}
	return nil
}

func (x *ListTenantResponse) GetNextPageToken() string {
	if x != nil {
		return x.NextPageToken
	}
	return ""
}

type BindRoleRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	TenantName string `protobuf:"bytes,1,opt,name=TenantName,proto3" json:"TenantName,omitempty"`
	RoleName   string `protobuf:"bytes,2,opt,name=RoleName,proto3" json:"RoleName,omitempty"`
}

func (x *BindRoleRequest) Reset() {
	*x = BindRoleRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pb_tenant_service_proto_msgTypes[6]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BindRoleRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BindRoleRequest) ProtoMessage() {}

func (x *BindRoleRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pb_tenant_service_proto_msgTypes[6]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BindRoleRequest.ProtoReflect.Descriptor instead.
func (*BindRoleRequest) Descriptor() ([]byte, []int) {
	return file_pb_tenant_service_proto_rawDescGZIP(), []int{6}
}

func (x *BindRoleRequest) GetTenantName() string {
	if x != nil {
		return x.TenantName
	}
	return ""
}

func (x *BindRoleRequest) GetRoleName() string {
	if x != nil {
		return x.RoleName
	}
	return ""
}

type BindRoleResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *BindRoleResponse) Reset() {
	*x = BindRoleResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pb_tenant_service_proto_msgTypes[7]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BindRoleResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BindRoleResponse) ProtoMessage() {}

func (x *BindRoleResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pb_tenant_service_proto_msgTypes[7]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BindRoleResponse.ProtoReflect.Descriptor instead.
func (*BindRoleResponse) Descriptor() ([]byte, []int) {
	return file_pb_tenant_service_proto_rawDescGZIP(), []int{7}
}

type UnbindRoleRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	TenantName string `protobuf:"bytes,1,opt,name=TenantName,proto3" json:"TenantName,omitempty"`
	RoleName   string `protobuf:"bytes,2,opt,name=RoleName,proto3" json:"RoleName,omitempty"`
}

func (x *UnbindRoleRequest) Reset() {
	*x = UnbindRoleRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pb_tenant_service_proto_msgTypes[8]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UnbindRoleRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UnbindRoleRequest) ProtoMessage() {}

func (x *UnbindRoleRequest) ProtoReflect() protoreflect.Message {
	mi := &file_pb_tenant_service_proto_msgTypes[8]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UnbindRoleRequest.ProtoReflect.Descriptor instead.
func (*UnbindRoleRequest) Descriptor() ([]byte, []int) {
	return file_pb_tenant_service_proto_rawDescGZIP(), []int{8}
}

func (x *UnbindRoleRequest) GetTenantName() string {
	if x != nil {
		return x.TenantName
	}
	return ""
}

func (x *UnbindRoleRequest) GetRoleName() string {
	if x != nil {
		return x.RoleName
	}
	return ""
}

type UnbindRoleResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *UnbindRoleResponse) Reset() {
	*x = UnbindRoleResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_pb_tenant_service_proto_msgTypes[9]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UnbindRoleResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UnbindRoleResponse) ProtoMessage() {}

func (x *UnbindRoleResponse) ProtoReflect() protoreflect.Message {
	mi := &file_pb_tenant_service_proto_msgTypes[9]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UnbindRoleResponse.ProtoReflect.Descriptor instead.
func (*UnbindRoleResponse) Descriptor() ([]byte, []int) {
	return file_pb_tenant_service_proto_rawDescGZIP(), []int{9}
}

var File_pb_tenant_service_proto protoreflect.FileDescriptor

var file_pb_tenant_service_proto_rawDesc = []byte{
	0x0a, 0x17, 0x70, 0x62, 0x2f, 0x74, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x5f, 0x73, 0x65, 0x72, 0x76,
	0x69, 0x63, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x06, 0x6b, 0x61, 0x72, 0x61, 0x76,
	0x69, 0x1a, 0x1b, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62,
	0x75, 0x66, 0x2f, 0x65, 0x6d, 0x70, 0x74, 0x79, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x32,
	0x0a, 0x06, 0x54, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x12, 0x14, 0x0a, 0x05,
	0x72, 0x6f, 0x6c, 0x65, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x72, 0x6f, 0x6c,
	0x65, 0x73, 0x22, 0x3d, 0x0a, 0x13, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x54, 0x65, 0x6e, 0x61,
	0x6e, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x26, 0x0a, 0x06, 0x74, 0x65, 0x6e,
	0x61, 0x6e, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0e, 0x2e, 0x6b, 0x61, 0x72, 0x61,
	0x76, 0x69, 0x2e, 0x54, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x52, 0x06, 0x74, 0x65, 0x6e, 0x61, 0x6e,
	0x74, 0x22, 0x26, 0x0a, 0x10, 0x47, 0x65, 0x74, 0x54, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x22, 0x29, 0x0a, 0x13, 0x44, 0x65, 0x6c,
	0x65, 0x74, 0x65, 0x54, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x12, 0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04,
	0x6e, 0x61, 0x6d, 0x65, 0x22, 0x4f, 0x0a, 0x11, 0x4c, 0x69, 0x73, 0x74, 0x54, 0x65, 0x6e, 0x61,
	0x6e, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1b, 0x0a, 0x09, 0x70, 0x61, 0x67,
	0x65, 0x5f, 0x73, 0x69, 0x7a, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x08, 0x70, 0x61,
	0x67, 0x65, 0x53, 0x69, 0x7a, 0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x70, 0x61, 0x67, 0x65, 0x5f, 0x74,
	0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x05, 0x52, 0x09, 0x70, 0x61, 0x67, 0x65,
	0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x22, 0x66, 0x0a, 0x12, 0x4c, 0x69, 0x73, 0x74, 0x54, 0x65, 0x6e,
	0x61, 0x6e, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x28, 0x0a, 0x07, 0x74,
	0x65, 0x6e, 0x61, 0x6e, 0x74, 0x73, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x0e, 0x2e, 0x6b,
	0x61, 0x72, 0x61, 0x76, 0x69, 0x2e, 0x54, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x52, 0x07, 0x74, 0x65,
	0x6e, 0x61, 0x6e, 0x74, 0x73, 0x12, 0x26, 0x0a, 0x0f, 0x6e, 0x65, 0x78, 0x74, 0x5f, 0x70, 0x61,
	0x67, 0x65, 0x5f, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0d,
	0x6e, 0x65, 0x78, 0x74, 0x50, 0x61, 0x67, 0x65, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x22, 0x4d, 0x0a,
	0x0f, 0x42, 0x69, 0x6e, 0x64, 0x52, 0x6f, 0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x12, 0x1e, 0x0a, 0x0a, 0x54, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x4e, 0x61, 0x6d, 0x65, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x54, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x4e, 0x61, 0x6d, 0x65,
	0x12, 0x1a, 0x0a, 0x08, 0x52, 0x6f, 0x6c, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x08, 0x52, 0x6f, 0x6c, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x22, 0x12, 0x0a, 0x10,
	0x42, 0x69, 0x6e, 0x64, 0x52, 0x6f, 0x6c, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x22, 0x4f, 0x0a, 0x11, 0x55, 0x6e, 0x62, 0x69, 0x6e, 0x64, 0x52, 0x6f, 0x6c, 0x65, 0x52, 0x65,
	0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1e, 0x0a, 0x0a, 0x54, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x4e,
	0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0a, 0x54, 0x65, 0x6e, 0x61, 0x6e,
	0x74, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x1a, 0x0a, 0x08, 0x52, 0x6f, 0x6c, 0x65, 0x4e, 0x61, 0x6d,
	0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x52, 0x6f, 0x6c, 0x65, 0x4e, 0x61, 0x6d,
	0x65, 0x22, 0x14, 0x0a, 0x12, 0x55, 0x6e, 0x62, 0x69, 0x6e, 0x64, 0x52, 0x6f, 0x6c, 0x65, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x32, 0x9d, 0x03, 0x0a, 0x0d, 0x54, 0x65, 0x6e, 0x61,
	0x6e, 0x74, 0x53, 0x65, 0x72, 0x76, 0x69, 0x63, 0x65, 0x12, 0x3d, 0x0a, 0x0c, 0x43, 0x72, 0x65,
	0x61, 0x74, 0x65, 0x54, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x12, 0x1b, 0x2e, 0x6b, 0x61, 0x72, 0x61,
	0x76, 0x69, 0x2e, 0x43, 0x72, 0x65, 0x61, 0x74, 0x65, 0x54, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x0e, 0x2e, 0x6b, 0x61, 0x72, 0x61, 0x76, 0x69, 0x2e,
	0x54, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x22, 0x00, 0x12, 0x37, 0x0a, 0x09, 0x47, 0x65, 0x74, 0x54,
	0x65, 0x6e, 0x61, 0x6e, 0x74, 0x12, 0x18, 0x2e, 0x6b, 0x61, 0x72, 0x61, 0x76, 0x69, 0x2e, 0x47,
	0x65, 0x74, 0x54, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x0e, 0x2e, 0x6b, 0x61, 0x72, 0x61, 0x76, 0x69, 0x2e, 0x54, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x22,
	0x00, 0x12, 0x45, 0x0a, 0x0c, 0x44, 0x65, 0x6c, 0x65, 0x74, 0x65, 0x54, 0x65, 0x6e, 0x61, 0x6e,
	0x74, 0x12, 0x1b, 0x2e, 0x6b, 0x61, 0x72, 0x61, 0x76, 0x69, 0x2e, 0x44, 0x65, 0x6c, 0x65, 0x74,
	0x65, 0x54, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x16,
	0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x22, 0x00, 0x12, 0x45, 0x0a, 0x0a, 0x4c, 0x69, 0x73, 0x74,
	0x54, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x12, 0x19, 0x2e, 0x6b, 0x61, 0x72, 0x61, 0x76, 0x69, 0x2e,
	0x4c, 0x69, 0x73, 0x74, 0x54, 0x65, 0x6e, 0x61, 0x6e, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73,
	0x74, 0x1a, 0x1a, 0x2e, 0x6b, 0x61, 0x72, 0x61, 0x76, 0x69, 0x2e, 0x4c, 0x69, 0x73, 0x74, 0x54,
	0x65, 0x6e, 0x61, 0x6e, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12,
	0x3f, 0x0a, 0x08, 0x42, 0x69, 0x6e, 0x64, 0x52, 0x6f, 0x6c, 0x65, 0x12, 0x17, 0x2e, 0x6b, 0x61,
	0x72, 0x61, 0x76, 0x69, 0x2e, 0x42, 0x69, 0x6e, 0x64, 0x52, 0x6f, 0x6c, 0x65, 0x52, 0x65, 0x71,
	0x75, 0x65, 0x73, 0x74, 0x1a, 0x18, 0x2e, 0x6b, 0x61, 0x72, 0x61, 0x76, 0x69, 0x2e, 0x42, 0x69,
	0x6e, 0x64, 0x52, 0x6f, 0x6c, 0x65, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00,
	0x12, 0x45, 0x0a, 0x0a, 0x55, 0x6e, 0x62, 0x69, 0x6e, 0x64, 0x52, 0x6f, 0x6c, 0x65, 0x12, 0x19,
	0x2e, 0x6b, 0x61, 0x72, 0x61, 0x76, 0x69, 0x2e, 0x55, 0x6e, 0x62, 0x69, 0x6e, 0x64, 0x52, 0x6f,
	0x6c, 0x65, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1a, 0x2e, 0x6b, 0x61, 0x72, 0x61,
	0x76, 0x69, 0x2e, 0x55, 0x6e, 0x62, 0x69, 0x6e, 0x64, 0x52, 0x6f, 0x6c, 0x65, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x29, 0x5a, 0x27, 0x67, 0x69, 0x74, 0x68, 0x75,
	0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x64, 0x65, 0x6c, 0x6c, 0x2f, 0x6b, 0x61, 0x72, 0x61, 0x76,
	0x69, 0x2d, 0x61, 0x75, 0x74, 0x68, 0x6f, 0x72, 0x69, 0x7a, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2f,
	0x70, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_pb_tenant_service_proto_rawDescOnce sync.Once
	file_pb_tenant_service_proto_rawDescData = file_pb_tenant_service_proto_rawDesc
)

func file_pb_tenant_service_proto_rawDescGZIP() []byte {
	file_pb_tenant_service_proto_rawDescOnce.Do(func() {
		file_pb_tenant_service_proto_rawDescData = protoimpl.X.CompressGZIP(file_pb_tenant_service_proto_rawDescData)
	})
	return file_pb_tenant_service_proto_rawDescData
}

var file_pb_tenant_service_proto_msgTypes = make([]protoimpl.MessageInfo, 10)
var file_pb_tenant_service_proto_goTypes = []interface{}{
	(*Tenant)(nil),              // 0: karavi.Tenant
	(*CreateTenantRequest)(nil), // 1: karavi.CreateTenantRequest
	(*GetTenantRequest)(nil),    // 2: karavi.GetTenantRequest
	(*DeleteTenantRequest)(nil), // 3: karavi.DeleteTenantRequest
	(*ListTenantRequest)(nil),   // 4: karavi.ListTenantRequest
	(*ListTenantResponse)(nil),  // 5: karavi.ListTenantResponse
	(*BindRoleRequest)(nil),     // 6: karavi.BindRoleRequest
	(*BindRoleResponse)(nil),    // 7: karavi.BindRoleResponse
	(*UnbindRoleRequest)(nil),   // 8: karavi.UnbindRoleRequest
	(*UnbindRoleResponse)(nil),  // 9: karavi.UnbindRoleResponse
	(*empty.Empty)(nil),         // 10: google.protobuf.Empty
}
var file_pb_tenant_service_proto_depIdxs = []int32{
	0,  // 0: karavi.CreateTenantRequest.tenant:type_name -> karavi.Tenant
	0,  // 1: karavi.ListTenantResponse.tenants:type_name -> karavi.Tenant
	1,  // 2: karavi.TenantService.CreateTenant:input_type -> karavi.CreateTenantRequest
	2,  // 3: karavi.TenantService.GetTenant:input_type -> karavi.GetTenantRequest
	3,  // 4: karavi.TenantService.DeleteTenant:input_type -> karavi.DeleteTenantRequest
	4,  // 5: karavi.TenantService.ListTenant:input_type -> karavi.ListTenantRequest
	6,  // 6: karavi.TenantService.BindRole:input_type -> karavi.BindRoleRequest
	8,  // 7: karavi.TenantService.UnbindRole:input_type -> karavi.UnbindRoleRequest
	0,  // 8: karavi.TenantService.CreateTenant:output_type -> karavi.Tenant
	0,  // 9: karavi.TenantService.GetTenant:output_type -> karavi.Tenant
	10, // 10: karavi.TenantService.DeleteTenant:output_type -> google.protobuf.Empty
	5,  // 11: karavi.TenantService.ListTenant:output_type -> karavi.ListTenantResponse
	7,  // 12: karavi.TenantService.BindRole:output_type -> karavi.BindRoleResponse
	9,  // 13: karavi.TenantService.UnbindRole:output_type -> karavi.UnbindRoleResponse
	8,  // [8:14] is the sub-list for method output_type
	2,  // [2:8] is the sub-list for method input_type
	2,  // [2:2] is the sub-list for extension type_name
	2,  // [2:2] is the sub-list for extension extendee
	0,  // [0:2] is the sub-list for field type_name
}

func init() { file_pb_tenant_service_proto_init() }
func file_pb_tenant_service_proto_init() {
	if File_pb_tenant_service_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_pb_tenant_service_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Tenant); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pb_tenant_service_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CreateTenantRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pb_tenant_service_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetTenantRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pb_tenant_service_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DeleteTenantRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pb_tenant_service_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListTenantRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pb_tenant_service_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListTenantResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pb_tenant_service_proto_msgTypes[6].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BindRoleRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pb_tenant_service_proto_msgTypes[7].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BindRoleResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pb_tenant_service_proto_msgTypes[8].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UnbindRoleRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_pb_tenant_service_proto_msgTypes[9].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UnbindRoleResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_pb_tenant_service_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   10,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_pb_tenant_service_proto_goTypes,
		DependencyIndexes: file_pb_tenant_service_proto_depIdxs,
		MessageInfos:      file_pb_tenant_service_proto_msgTypes,
	}.Build()
	File_pb_tenant_service_proto = out.File
	file_pb_tenant_service_proto_rawDesc = nil
	file_pb_tenant_service_proto_goTypes = nil
	file_pb_tenant_service_proto_depIdxs = nil
}