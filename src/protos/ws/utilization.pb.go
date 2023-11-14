// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        v3.21.12
// source: utilization.proto

package ws

import (
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

type GetResourceUtilRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// auth is currently unused but we allocate the slot for later
	Auth        string `protobuf:"bytes,1,opt,name=auth,proto3" json:"auth,omitempty"`
	WorkspaceId int64  `protobuf:"varint,2,opt,name=workspace_id,json=workspaceId,proto3" json:"workspace_id,omitempty"`
	OwnerId     int64  `protobuf:"varint,3,opt,name=owner_id,json=ownerId,proto3" json:"owner_id,omitempty"`
}

func (x *GetResourceUtilRequest) Reset() {
	*x = GetResourceUtilRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_utilization_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetResourceUtilRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetResourceUtilRequest) ProtoMessage() {}

func (x *GetResourceUtilRequest) ProtoReflect() protoreflect.Message {
	mi := &file_utilization_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetResourceUtilRequest.ProtoReflect.Descriptor instead.
func (*GetResourceUtilRequest) Descriptor() ([]byte, []int) {
	return file_utilization_proto_rawDescGZIP(), []int{0}
}

func (x *GetResourceUtilRequest) GetAuth() string {
	if x != nil {
		return x.Auth
	}
	return ""
}

func (x *GetResourceUtilRequest) GetWorkspaceId() int64 {
	if x != nil {
		return x.WorkspaceId
	}
	return 0
}

func (x *GetResourceUtilRequest) GetOwnerId() int64 {
	if x != nil {
		return x.OwnerId
	}
	return 0
}

type GetResourceUtilResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Status      ResponseCode `protobuf:"varint,1,opt,name=status,proto3,enum=ws.ResponseCode" json:"status,omitempty"`
	Success     *Success     `protobuf:"bytes,2,opt,name=success,proto3" json:"success,omitempty"`
	Error       *Error       `protobuf:"bytes,3,opt,name=error,proto3" json:"error,omitempty"`
	Cpu         float64      `protobuf:"fixed64,4,opt,name=cpu,proto3" json:"cpu,omitempty"`
	Memory      float64      `protobuf:"fixed64,5,opt,name=memory,proto3" json:"memory,omitempty"`
	CpuLimit    int64        `protobuf:"varint,6,opt,name=cpu_limit,json=cpuLimit,proto3" json:"cpu_limit,omitempty"`
	MemoryLimit int64        `protobuf:"varint,7,opt,name=memory_limit,json=memoryLimit,proto3" json:"memory_limit,omitempty"`
	CpuUsage    int64        `protobuf:"varint,8,opt,name=cpu_usage,json=cpuUsage,proto3" json:"cpu_usage,omitempty"`
	MemoryUsage int64        `protobuf:"varint,9,opt,name=memory_usage,json=memoryUsage,proto3" json:"memory_usage,omitempty"`
}

func (x *GetResourceUtilResponse) Reset() {
	*x = GetResourceUtilResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_utilization_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetResourceUtilResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetResourceUtilResponse) ProtoMessage() {}

func (x *GetResourceUtilResponse) ProtoReflect() protoreflect.Message {
	mi := &file_utilization_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetResourceUtilResponse.ProtoReflect.Descriptor instead.
func (*GetResourceUtilResponse) Descriptor() ([]byte, []int) {
	return file_utilization_proto_rawDescGZIP(), []int{1}
}

func (x *GetResourceUtilResponse) GetStatus() ResponseCode {
	if x != nil {
		return x.Status
	}
	return ResponseCode_SUCCESS
}

func (x *GetResourceUtilResponse) GetSuccess() *Success {
	if x != nil {
		return x.Success
	}
	return nil
}

func (x *GetResourceUtilResponse) GetError() *Error {
	if x != nil {
		return x.Error
	}
	return nil
}

func (x *GetResourceUtilResponse) GetCpu() float64 {
	if x != nil {
		return x.Cpu
	}
	return 0
}

func (x *GetResourceUtilResponse) GetMemory() float64 {
	if x != nil {
		return x.Memory
	}
	return 0
}

func (x *GetResourceUtilResponse) GetCpuLimit() int64 {
	if x != nil {
		return x.CpuLimit
	}
	return 0
}

func (x *GetResourceUtilResponse) GetMemoryLimit() int64 {
	if x != nil {
		return x.MemoryLimit
	}
	return 0
}

func (x *GetResourceUtilResponse) GetCpuUsage() int64 {
	if x != nil {
		return x.CpuUsage
	}
	return 0
}

func (x *GetResourceUtilResponse) GetMemoryUsage() int64 {
	if x != nil {
		return x.MemoryUsage
	}
	return 0
}

var File_utilization_proto protoreflect.FileDescriptor

var file_utilization_proto_rawDesc = []byte{
	0x0a, 0x11, 0x75, 0x74, 0x69, 0x6c, 0x69, 0x7a, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x02, 0x77, 0x73, 0x1a, 0x0b, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x22, 0x6a, 0x0a, 0x16, 0x47, 0x65, 0x74, 0x52, 0x65, 0x73, 0x6f, 0x75,
	0x72, 0x63, 0x65, 0x55, 0x74, 0x69, 0x6c, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x12,
	0x0a, 0x04, 0x61, 0x75, 0x74, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x61, 0x75,
	0x74, 0x68, 0x12, 0x21, 0x0a, 0x0c, 0x77, 0x6f, 0x72, 0x6b, 0x73, 0x70, 0x61, 0x63, 0x65, 0x5f,
	0x69, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0b, 0x77, 0x6f, 0x72, 0x6b, 0x73, 0x70,
	0x61, 0x63, 0x65, 0x49, 0x64, 0x12, 0x19, 0x0a, 0x08, 0x6f, 0x77, 0x6e, 0x65, 0x72, 0x5f, 0x69,
	0x64, 0x18, 0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x6f, 0x77, 0x6e, 0x65, 0x72, 0x49, 0x64,
	0x22, 0xb5, 0x02, 0x0a, 0x17, 0x47, 0x65, 0x74, 0x52, 0x65, 0x73, 0x6f, 0x75, 0x72, 0x63, 0x65,
	0x55, 0x74, 0x69, 0x6c, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x28, 0x0a, 0x06,
	0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x10, 0x2e, 0x77,
	0x73, 0x2e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x43, 0x6f, 0x64, 0x65, 0x52, 0x06,
	0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x12, 0x25, 0x0a, 0x07, 0x73, 0x75, 0x63, 0x63, 0x65, 0x73,
	0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x0b, 0x2e, 0x77, 0x73, 0x2e, 0x53, 0x75, 0x63,
	0x63, 0x65, 0x73, 0x73, 0x52, 0x07, 0x73, 0x75, 0x63, 0x63, 0x65, 0x73, 0x73, 0x12, 0x1f, 0x0a,
	0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x09, 0x2e, 0x77,
	0x73, 0x2e, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x52, 0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x12, 0x10,
	0x0a, 0x03, 0x63, 0x70, 0x75, 0x18, 0x04, 0x20, 0x01, 0x28, 0x01, 0x52, 0x03, 0x63, 0x70, 0x75,
	0x12, 0x16, 0x0a, 0x06, 0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x18, 0x05, 0x20, 0x01, 0x28, 0x01,
	0x52, 0x06, 0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x12, 0x1b, 0x0a, 0x09, 0x63, 0x70, 0x75, 0x5f,
	0x6c, 0x69, 0x6d, 0x69, 0x74, 0x18, 0x06, 0x20, 0x01, 0x28, 0x03, 0x52, 0x08, 0x63, 0x70, 0x75,
	0x4c, 0x69, 0x6d, 0x69, 0x74, 0x12, 0x21, 0x0a, 0x0c, 0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x5f,
	0x6c, 0x69, 0x6d, 0x69, 0x74, 0x18, 0x07, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0b, 0x6d, 0x65, 0x6d,
	0x6f, 0x72, 0x79, 0x4c, 0x69, 0x6d, 0x69, 0x74, 0x12, 0x1b, 0x0a, 0x09, 0x63, 0x70, 0x75, 0x5f,
	0x75, 0x73, 0x61, 0x67, 0x65, 0x18, 0x08, 0x20, 0x01, 0x28, 0x03, 0x52, 0x08, 0x63, 0x70, 0x75,
	0x55, 0x73, 0x61, 0x67, 0x65, 0x12, 0x21, 0x0a, 0x0c, 0x6d, 0x65, 0x6d, 0x6f, 0x72, 0x79, 0x5f,
	0x75, 0x73, 0x61, 0x67, 0x65, 0x18, 0x09, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0b, 0x6d, 0x65, 0x6d,
	0x6f, 0x72, 0x79, 0x55, 0x73, 0x61, 0x67, 0x65, 0x42, 0x0b, 0x5a, 0x09, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x73, 0x2f, 0x77, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_utilization_proto_rawDescOnce sync.Once
	file_utilization_proto_rawDescData = file_utilization_proto_rawDesc
)

func file_utilization_proto_rawDescGZIP() []byte {
	file_utilization_proto_rawDescOnce.Do(func() {
		file_utilization_proto_rawDescData = protoimpl.X.CompressGZIP(file_utilization_proto_rawDescData)
	})
	return file_utilization_proto_rawDescData
}

var file_utilization_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_utilization_proto_goTypes = []interface{}{
	(*GetResourceUtilRequest)(nil),  // 0: ws.GetResourceUtilRequest
	(*GetResourceUtilResponse)(nil), // 1: ws.GetResourceUtilResponse
	(ResponseCode)(0),               // 2: ws.ResponseCode
	(*Success)(nil),                 // 3: ws.Success
	(*Error)(nil),                   // 4: ws.Error
}
var file_utilization_proto_depIdxs = []int32{
	2, // 0: ws.GetResourceUtilResponse.status:type_name -> ws.ResponseCode
	3, // 1: ws.GetResourceUtilResponse.success:type_name -> ws.Success
	4, // 2: ws.GetResourceUtilResponse.error:type_name -> ws.Error
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_utilization_proto_init() }
func file_utilization_proto_init() {
	if File_utilization_proto != nil {
		return
	}
	file_types_proto_init()
	if !protoimpl.UnsafeEnabled {
		file_utilization_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetResourceUtilRequest); i {
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
		file_utilization_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetResourceUtilResponse); i {
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
			RawDescriptor: file_utilization_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_utilization_proto_goTypes,
		DependencyIndexes: file_utilization_proto_depIdxs,
		MessageInfos:      file_utilization_proto_msgTypes,
	}.Build()
	File_utilization_proto = out.File
	file_utilization_proto_rawDesc = nil
	file_utilization_proto_goTypes = nil
	file_utilization_proto_depIdxs = nil
}
