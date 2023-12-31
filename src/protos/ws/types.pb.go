// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.30.0
// 	protoc        v3.15.8
// source: types.proto

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

// response code to give success & failure context
type ResponseCode int32

const (
	ResponseCode_SUCCESS                    ResponseCode = 0
	ResponseCode_SUCCESS_STREAM_COMPLETE    ResponseCode = 1
	ResponseCode_FAILED_AUTHENTICATION      ResponseCode = 2
	ResponseCode_FAILED_AUTHORIZATION       ResponseCode = 3
	ResponseCode_MALFORMED_REQUEST          ResponseCode = 4
	ResponseCode_SERVICE_BLOCK              ResponseCode = 5
	ResponseCode_SERVER_EXECUTION_ERROR     ResponseCode = 6
	ResponseCode_CLIENT_STREAM_FAILURE      ResponseCode = 7
	ResponseCode_SERVER_STREAM_FAILURE      ResponseCode = 8
	ResponseCode_NOT_FOUND                  ResponseCode = 9
	ResponseCode_TF_INIT_FAILURE            ResponseCode = 10
	ResponseCode_TF_VALIDATION_ERROR        ResponseCode = 11
	ResponseCode_TF_PROVISIONING_FAILURE    ResponseCode = 12
	ResponseCode_ALTERNATIVE_REQUEST_ACTIVE ResponseCode = 13
)

// Enum value maps for ResponseCode.
var (
	ResponseCode_name = map[int32]string{
		0:  "SUCCESS",
		1:  "SUCCESS_STREAM_COMPLETE",
		2:  "FAILED_AUTHENTICATION",
		3:  "FAILED_AUTHORIZATION",
		4:  "MALFORMED_REQUEST",
		5:  "SERVICE_BLOCK",
		6:  "SERVER_EXECUTION_ERROR",
		7:  "CLIENT_STREAM_FAILURE",
		8:  "SERVER_STREAM_FAILURE",
		9:  "NOT_FOUND",
		10: "TF_INIT_FAILURE",
		11: "TF_VALIDATION_ERROR",
		12: "TF_PROVISIONING_FAILURE",
		13: "ALTERNATIVE_REQUEST_ACTIVE",
	}
	ResponseCode_value = map[string]int32{
		"SUCCESS":                    0,
		"SUCCESS_STREAM_COMPLETE":    1,
		"FAILED_AUTHENTICATION":      2,
		"FAILED_AUTHORIZATION":       3,
		"MALFORMED_REQUEST":          4,
		"SERVICE_BLOCK":              5,
		"SERVER_EXECUTION_ERROR":     6,
		"CLIENT_STREAM_FAILURE":      7,
		"SERVER_STREAM_FAILURE":      8,
		"NOT_FOUND":                  9,
		"TF_INIT_FAILURE":            10,
		"TF_VALIDATION_ERROR":        11,
		"TF_PROVISIONING_FAILURE":    12,
		"ALTERNATIVE_REQUEST_ACTIVE": 13,
	}
)

func (x ResponseCode) Enum() *ResponseCode {
	p := new(ResponseCode)
	*p = x
	return p
}

func (x ResponseCode) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (ResponseCode) Descriptor() protoreflect.EnumDescriptor {
	return file_types_proto_enumTypes[0].Descriptor()
}

func (ResponseCode) Type() protoreflect.EnumType {
	return &file_types_proto_enumTypes[0]
}

func (x ResponseCode) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use ResponseCode.Descriptor instead.
func (ResponseCode) EnumDescriptor() ([]byte, []int) {
	return file_types_proto_rawDescGZIP(), []int{0}
}

type Success struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	StartTime   int64 `protobuf:"varint,1,opt,name=start_time,json=startTime,proto3" json:"start_time,omitempty"`
	EndTime     int64 `protobuf:"varint,2,opt,name=end_time,json=endTime,proto3" json:"end_time,omitempty"`
	ElapsedTime int64 `protobuf:"varint,3,opt,name=elapsed_time,json=elapsedTime,proto3" json:"elapsed_time,omitempty"`
}

func (x *Success) Reset() {
	*x = Success{}
	if protoimpl.UnsafeEnabled {
		mi := &file_types_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Success) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Success) ProtoMessage() {}

func (x *Success) ProtoReflect() protoreflect.Message {
	mi := &file_types_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Success.ProtoReflect.Descriptor instead.
func (*Success) Descriptor() ([]byte, []int) {
	return file_types_proto_rawDescGZIP(), []int{0}
}

func (x *Success) GetStartTime() int64 {
	if x != nil {
		return x.StartTime
	}
	return 0
}

func (x *Success) GetEndTime() int64 {
	if x != nil {
		return x.EndTime
	}
	return 0
}

func (x *Success) GetElapsedTime() int64 {
	if x != nil {
		return x.ElapsedTime
	}
	return 0
}

type CommandError struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ExitCode    int32  `protobuf:"varint,1,opt,name=exit_code,json=exitCode,proto3" json:"exit_code,omitempty"`
	Stdout      string `protobuf:"bytes,2,opt,name=stdout,proto3" json:"stdout,omitempty"`
	Stderr      string `protobuf:"bytes,3,opt,name=stderr,proto3" json:"stderr,omitempty"`
	StartTime   int64  `protobuf:"varint,4,opt,name=start_time,json=startTime,proto3" json:"start_time,omitempty"`
	EndTime     int64  `protobuf:"varint,5,opt,name=end_time,json=endTime,proto3" json:"end_time,omitempty"`
	ElapsedTime int64  `protobuf:"varint,6,opt,name=elapsed_time,json=elapsedTime,proto3" json:"elapsed_time,omitempty"`
}

func (x *CommandError) Reset() {
	*x = CommandError{}
	if protoimpl.UnsafeEnabled {
		mi := &file_types_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *CommandError) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*CommandError) ProtoMessage() {}

func (x *CommandError) ProtoReflect() protoreflect.Message {
	mi := &file_types_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use CommandError.ProtoReflect.Descriptor instead.
func (*CommandError) Descriptor() ([]byte, []int) {
	return file_types_proto_rawDescGZIP(), []int{1}
}

func (x *CommandError) GetExitCode() int32 {
	if x != nil {
		return x.ExitCode
	}
	return 0
}

func (x *CommandError) GetStdout() string {
	if x != nil {
		return x.Stdout
	}
	return ""
}

func (x *CommandError) GetStderr() string {
	if x != nil {
		return x.Stderr
	}
	return ""
}

func (x *CommandError) GetStartTime() int64 {
	if x != nil {
		return x.StartTime
	}
	return 0
}

func (x *CommandError) GetEndTime() int64 {
	if x != nil {
		return x.EndTime
	}
	return 0
}

func (x *CommandError) GetElapsedTime() int64 {
	if x != nil {
		return x.ElapsedTime
	}
	return 0
}

type Error struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	CmdError *CommandError `protobuf:"bytes,2,opt,name=cmd_error,json=cmdError,proto3" json:"cmd_error,omitempty"`
	GoError  string        `protobuf:"bytes,3,opt,name=go_error,json=goError,proto3" json:"go_error,omitempty"`
}

func (x *Error) Reset() {
	*x = Error{}
	if protoimpl.UnsafeEnabled {
		mi := &file_types_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Error) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Error) ProtoMessage() {}

func (x *Error) ProtoReflect() protoreflect.Message {
	mi := &file_types_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Error.ProtoReflect.Descriptor instead.
func (*Error) Descriptor() ([]byte, []int) {
	return file_types_proto_rawDescGZIP(), []int{2}
}

func (x *Error) GetCmdError() *CommandError {
	if x != nil {
		return x.CmdError
	}
	return nil
}

func (x *Error) GetGoError() string {
	if x != nil {
		return x.GoError
	}
	return ""
}

var File_types_proto protoreflect.FileDescriptor

var file_types_proto_rawDesc = []byte{
	0x0a, 0x0b, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x02, 0x77,
	0x73, 0x22, 0x66, 0x0a, 0x07, 0x53, 0x75, 0x63, 0x63, 0x65, 0x73, 0x73, 0x12, 0x1d, 0x0a, 0x0a,
	0x73, 0x74, 0x61, 0x72, 0x74, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03,
	0x52, 0x09, 0x73, 0x74, 0x61, 0x72, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x12, 0x19, 0x0a, 0x08, 0x65,
	0x6e, 0x64, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x65,
	0x6e, 0x64, 0x54, 0x69, 0x6d, 0x65, 0x12, 0x21, 0x0a, 0x0c, 0x65, 0x6c, 0x61, 0x70, 0x73, 0x65,
	0x64, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0b, 0x65, 0x6c,
	0x61, 0x70, 0x73, 0x65, 0x64, 0x54, 0x69, 0x6d, 0x65, 0x22, 0xb8, 0x01, 0x0a, 0x0c, 0x43, 0x6f,
	0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x12, 0x1b, 0x0a, 0x09, 0x65, 0x78,
	0x69, 0x74, 0x5f, 0x63, 0x6f, 0x64, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x05, 0x52, 0x08, 0x65,
	0x78, 0x69, 0x74, 0x43, 0x6f, 0x64, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x73, 0x74, 0x64, 0x6f, 0x75,
	0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73, 0x74, 0x64, 0x6f, 0x75, 0x74, 0x12,
	0x16, 0x0a, 0x06, 0x73, 0x74, 0x64, 0x65, 0x72, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x06, 0x73, 0x74, 0x64, 0x65, 0x72, 0x72, 0x12, 0x1d, 0x0a, 0x0a, 0x73, 0x74, 0x61, 0x72, 0x74,
	0x5f, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x03, 0x52, 0x09, 0x73, 0x74, 0x61,
	0x72, 0x74, 0x54, 0x69, 0x6d, 0x65, 0x12, 0x19, 0x0a, 0x08, 0x65, 0x6e, 0x64, 0x5f, 0x74, 0x69,
	0x6d, 0x65, 0x18, 0x05, 0x20, 0x01, 0x28, 0x03, 0x52, 0x07, 0x65, 0x6e, 0x64, 0x54, 0x69, 0x6d,
	0x65, 0x12, 0x21, 0x0a, 0x0c, 0x65, 0x6c, 0x61, 0x70, 0x73, 0x65, 0x64, 0x5f, 0x74, 0x69, 0x6d,
	0x65, 0x18, 0x06, 0x20, 0x01, 0x28, 0x03, 0x52, 0x0b, 0x65, 0x6c, 0x61, 0x70, 0x73, 0x65, 0x64,
	0x54, 0x69, 0x6d, 0x65, 0x22, 0x51, 0x0a, 0x05, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x12, 0x2d, 0x0a,
	0x09, 0x63, 0x6d, 0x64, 0x5f, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x10, 0x2e, 0x77, 0x73, 0x2e, 0x43, 0x6f, 0x6d, 0x6d, 0x61, 0x6e, 0x64, 0x45, 0x72, 0x72,
	0x6f, 0x72, 0x52, 0x08, 0x63, 0x6d, 0x64, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x12, 0x19, 0x0a, 0x08,
	0x67, 0x6f, 0x5f, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07,
	0x67, 0x6f, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x2a, 0xe3, 0x02, 0x0a, 0x0c, 0x52, 0x65, 0x73, 0x70,
	0x6f, 0x6e, 0x73, 0x65, 0x43, 0x6f, 0x64, 0x65, 0x12, 0x0b, 0x0a, 0x07, 0x53, 0x55, 0x43, 0x43,
	0x45, 0x53, 0x53, 0x10, 0x00, 0x12, 0x1b, 0x0a, 0x17, 0x53, 0x55, 0x43, 0x43, 0x45, 0x53, 0x53,
	0x5f, 0x53, 0x54, 0x52, 0x45, 0x41, 0x4d, 0x5f, 0x43, 0x4f, 0x4d, 0x50, 0x4c, 0x45, 0x54, 0x45,
	0x10, 0x01, 0x12, 0x19, 0x0a, 0x15, 0x46, 0x41, 0x49, 0x4c, 0x45, 0x44, 0x5f, 0x41, 0x55, 0x54,
	0x48, 0x45, 0x4e, 0x54, 0x49, 0x43, 0x41, 0x54, 0x49, 0x4f, 0x4e, 0x10, 0x02, 0x12, 0x18, 0x0a,
	0x14, 0x46, 0x41, 0x49, 0x4c, 0x45, 0x44, 0x5f, 0x41, 0x55, 0x54, 0x48, 0x4f, 0x52, 0x49, 0x5a,
	0x41, 0x54, 0x49, 0x4f, 0x4e, 0x10, 0x03, 0x12, 0x15, 0x0a, 0x11, 0x4d, 0x41, 0x4c, 0x46, 0x4f,
	0x52, 0x4d, 0x45, 0x44, 0x5f, 0x52, 0x45, 0x51, 0x55, 0x45, 0x53, 0x54, 0x10, 0x04, 0x12, 0x11,
	0x0a, 0x0d, 0x53, 0x45, 0x52, 0x56, 0x49, 0x43, 0x45, 0x5f, 0x42, 0x4c, 0x4f, 0x43, 0x4b, 0x10,
	0x05, 0x12, 0x1a, 0x0a, 0x16, 0x53, 0x45, 0x52, 0x56, 0x45, 0x52, 0x5f, 0x45, 0x58, 0x45, 0x43,
	0x55, 0x54, 0x49, 0x4f, 0x4e, 0x5f, 0x45, 0x52, 0x52, 0x4f, 0x52, 0x10, 0x06, 0x12, 0x19, 0x0a,
	0x15, 0x43, 0x4c, 0x49, 0x45, 0x4e, 0x54, 0x5f, 0x53, 0x54, 0x52, 0x45, 0x41, 0x4d, 0x5f, 0x46,
	0x41, 0x49, 0x4c, 0x55, 0x52, 0x45, 0x10, 0x07, 0x12, 0x19, 0x0a, 0x15, 0x53, 0x45, 0x52, 0x56,
	0x45, 0x52, 0x5f, 0x53, 0x54, 0x52, 0x45, 0x41, 0x4d, 0x5f, 0x46, 0x41, 0x49, 0x4c, 0x55, 0x52,
	0x45, 0x10, 0x08, 0x12, 0x0d, 0x0a, 0x09, 0x4e, 0x4f, 0x54, 0x5f, 0x46, 0x4f, 0x55, 0x4e, 0x44,
	0x10, 0x09, 0x12, 0x13, 0x0a, 0x0f, 0x54, 0x46, 0x5f, 0x49, 0x4e, 0x49, 0x54, 0x5f, 0x46, 0x41,
	0x49, 0x4c, 0x55, 0x52, 0x45, 0x10, 0x0a, 0x12, 0x17, 0x0a, 0x13, 0x54, 0x46, 0x5f, 0x56, 0x41,
	0x4c, 0x49, 0x44, 0x41, 0x54, 0x49, 0x4f, 0x4e, 0x5f, 0x45, 0x52, 0x52, 0x4f, 0x52, 0x10, 0x0b,
	0x12, 0x1b, 0x0a, 0x17, 0x54, 0x46, 0x5f, 0x50, 0x52, 0x4f, 0x56, 0x49, 0x53, 0x49, 0x4f, 0x4e,
	0x49, 0x4e, 0x47, 0x5f, 0x46, 0x41, 0x49, 0x4c, 0x55, 0x52, 0x45, 0x10, 0x0c, 0x12, 0x1e, 0x0a,
	0x1a, 0x41, 0x4c, 0x54, 0x45, 0x52, 0x4e, 0x41, 0x54, 0x49, 0x56, 0x45, 0x5f, 0x52, 0x45, 0x51,
	0x55, 0x45, 0x53, 0x54, 0x5f, 0x41, 0x43, 0x54, 0x49, 0x56, 0x45, 0x10, 0x0d, 0x42, 0x0b, 0x5a,
	0x09, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x73, 0x2f, 0x77, 0x73, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
}

var (
	file_types_proto_rawDescOnce sync.Once
	file_types_proto_rawDescData = file_types_proto_rawDesc
)

func file_types_proto_rawDescGZIP() []byte {
	file_types_proto_rawDescOnce.Do(func() {
		file_types_proto_rawDescData = protoimpl.X.CompressGZIP(file_types_proto_rawDescData)
	})
	return file_types_proto_rawDescData
}

var file_types_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_types_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_types_proto_goTypes = []interface{}{
	(ResponseCode)(0),    // 0: ws.ResponseCode
	(*Success)(nil),      // 1: ws.Success
	(*CommandError)(nil), // 2: ws.CommandError
	(*Error)(nil),        // 3: ws.Error
}
var file_types_proto_depIdxs = []int32{
	2, // 0: ws.Error.cmd_error:type_name -> ws.CommandError
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_types_proto_init() }
func file_types_proto_init() {
	if File_types_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_types_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Success); i {
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
		file_types_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*CommandError); i {
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
		file_types_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Error); i {
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
			RawDescriptor: file_types_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_types_proto_goTypes,
		DependencyIndexes: file_types_proto_depIdxs,
		EnumInfos:         file_types_proto_enumTypes,
		MessageInfos:      file_types_proto_msgTypes,
	}.Build()
	File_types_proto = out.File
	file_types_proto_rawDesc = nil
	file_types_proto_goTypes = nil
	file_types_proto_depIdxs = nil
}
