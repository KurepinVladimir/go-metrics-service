// Code generated manually for metrics proto. DO NOT EDIT.
package proto

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - protoimpl.MinVersion)
)

// Metric type enumeration.
type Metric_MType int32

const (
	Metric_GAUGE   Metric_MType = 0
	Metric_COUNTER Metric_MType = 1
)

var (
	Metric_MType_name = map[int32]string{
		0: "GAUGE",
		1: "COUNTER",
	}
	Metric_MType_value = map[string]int32{
		"GAUGE":   0,
		"COUNTER": 1,
	}
)

func (x Metric_MType) Enum() *Metric_MType {
	p := new(Metric_MType)
	*p = x
	return p
}

func (x Metric_MType) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (Metric_MType) Descriptor() protoreflect.EnumDescriptor {
	return file_metrics_proto_enumTypes[0].Descriptor()
}

func (Metric_MType) Type() protoreflect.EnumType {
	return &file_metrics_proto_enumTypes[0]
}

func (x Metric_MType) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use Metric_MType.Descriptor instead.
func (Metric_MType) EnumDescriptor() ([]byte, []int) {
	return file_metrics_proto_rawDescGZIP(), []int{0, 0}
}

// Metric defines a single metric entry.
type Metric struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id    string       `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Type  Metric_MType `protobuf:"varint,2,opt,name=type,proto3,enum=metrics.Metric_MType" json:"type,omitempty"`
	Delta int64        `protobuf:"varint,3,opt,name=delta,proto3" json:"delta,omitempty"`
	Value float64      `protobuf:"fixed64,4,opt,name=value,proto3" json:"value,omitempty"`
}

func (x *Metric) Reset() {
	*x = Metric{}
	if protoimpl.UnsafeEnabled {
		mi := &file_metrics_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Metric) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Metric) ProtoMessage() {}

func (x *Metric) ProtoReflect() protoreflect.Message {
	mi := &file_metrics_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Metric.ProtoReflect.Descriptor instead.
func (*Metric) Descriptor() ([]byte, []int) {
	return file_metrics_proto_rawDescGZIP(), []int{0}
}

func (x *Metric) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *Metric) GetType() Metric_MType {
	if x != nil {
		return x.Type
	}
	return Metric_GAUGE
}

func (x *Metric) GetDelta() int64 {
	if x != nil {
		return x.Delta
	}
	return 0
}

func (x *Metric) GetValue() float64 {
	if x != nil {
		return x.Value
	}
	return 0
}

// UpdateMetricsRequest contains a batch of metrics.
type UpdateMetricsRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Metrics []*Metric `protobuf:"bytes,1,rep,name=metrics,proto3" json:"metrics,omitempty"`
}

func (x *UpdateMetricsRequest) Reset() {
	*x = UpdateMetricsRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_metrics_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateMetricsRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateMetricsRequest) ProtoMessage() {}

func (x *UpdateMetricsRequest) ProtoReflect() protoreflect.Message {
	mi := &file_metrics_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateMetricsRequest.ProtoReflect.Descriptor instead.
func (*UpdateMetricsRequest) Descriptor() ([]byte, []int) {
	return file_metrics_proto_rawDescGZIP(), []int{1}
}

func (x *UpdateMetricsRequest) GetMetrics() []*Metric {
	if x != nil {
		return x.Metrics
	}
	return nil
}

// UpdateMetricsResponse confirms successful update.
type UpdateMetricsResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields
}

func (x *UpdateMetricsResponse) Reset() {
	*x = UpdateMetricsResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_metrics_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *UpdateMetricsResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*UpdateMetricsResponse) ProtoMessage() {}

func (x *UpdateMetricsResponse) ProtoReflect() protoreflect.Message {
	mi := &file_metrics_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use UpdateMetricsResponse.ProtoReflect.Descriptor instead.
func (*UpdateMetricsResponse) Descriptor() ([]byte, []int) {
	return file_metrics_proto_rawDescGZIP(), []int{2}
}

var File_metrics_proto protoreflect.FileDescriptor

var file_metrics_proto_rawDesc = []byte{
	0x0a, 0x0d, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x12, 0x07, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x22, 0x90,
	0x01, 0x0a, 0x06, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x12, 0x0e, 0x0a, 0x02,
	0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12,
	0x29, 0x0a, 0x04, 0x74, 0x79, 0x70, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e,
	0x32, 0x15, 0x2e, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x4d, 0x65,
	0x74, 0x72, 0x69, 0x63, 0x2e, 0x4d, 0x54, 0x79, 0x70, 0x65, 0x52, 0x04, 0x74,
	0x79, 0x70, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x64, 0x65, 0x6c, 0x74, 0x61, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x03, 0x52, 0x05, 0x64, 0x65, 0x6c, 0x74, 0x61, 0x12,
	0x14, 0x0a, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28,
	0x01, 0x52, 0x05, 0x76, 0x61, 0x6c, 0x75, 0x65, 0x22, 0x1f, 0x0a, 0x05, 0x4d,
	0x54, 0x79, 0x70, 0x65, 0x12, 0x09, 0x0a, 0x05, 0x47, 0x41, 0x55, 0x47, 0x45,
	0x10, 0x00, 0x12, 0x0b, 0x0a, 0x07, 0x43, 0x4f, 0x55, 0x4e, 0x54, 0x45, 0x52,
	0x10, 0x01, 0x22, 0x41, 0x0a, 0x14, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x4d,
	0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x12, 0x29, 0x0a, 0x07, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x18, 0x01,
	0x20, 0x03, 0x28, 0x0b, 0x32, 0x0f, 0x2e, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63,
	0x73, 0x2e, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x52, 0x07, 0x6d, 0x65, 0x74,
	0x72, 0x69, 0x63, 0x73, 0x22, 0x17, 0x0a, 0x15, 0x55, 0x70, 0x64, 0x61, 0x74,
	0x65, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x32, 0x59, 0x0a, 0x07, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63,
	0x73, 0x12, 0x4e, 0x0a, 0x0d, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x4d, 0x65,
	0x74, 0x72, 0x69, 0x63, 0x73, 0x12, 0x1d, 0x2e, 0x6d, 0x65, 0x74, 0x72, 0x69,
	0x63, 0x73, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74, 0x65, 0x4d, 0x65, 0x74, 0x72,
	0x69, 0x63, 0x73, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x1e, 0x2e,
	0x6d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x2e, 0x55, 0x70, 0x64, 0x61, 0x74,
	0x65, 0x4d, 0x65, 0x74, 0x72, 0x69, 0x63, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x42, 0x47, 0x5a, 0x45, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62,
	0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x4b, 0x75, 0x72, 0x65, 0x70, 0x69, 0x6e, 0x56,
	0x6c, 0x61, 0x64, 0x69, 0x6d, 0x69, 0x72, 0x2f, 0x67, 0x6f, 0x2d, 0x6d, 0x75,
	0x73, 0x74, 0x68, 0x61, 0x76, 0x65, 0x2d, 0x6d, 0x65, 0x74, 0x72, 0x69, 0x63,
	0x73, 0x2d, 0x74, 0x70, 0x6c, 0x2e, 0x67, 0x69, 0x74, 0x2f, 0x69, 0x6e, 0x74,
	0x65, 0x72, 0x6e, 0x61, 0x6c, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_metrics_proto_rawDescOnce sync.Once
	file_metrics_proto_rawDescData = file_metrics_proto_rawDesc
)

func file_metrics_proto_rawDescGZIP() []byte {
	file_metrics_proto_rawDescOnce.Do(func() {
		file_metrics_proto_rawDescData = protoimpl.X.CompressGZIP(file_metrics_proto_rawDescData)
	})
	return file_metrics_proto_rawDescData
}

var file_metrics_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_metrics_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_metrics_proto_goTypes = []interface{}{
	(Metric_MType)(0),             // 0: metrics.Metric.MType
	(*Metric)(nil),                // 1: metrics.Metric
	(*UpdateMetricsRequest)(nil),  // 2: metrics.UpdateMetricsRequest
	(*UpdateMetricsResponse)(nil), // 3: metrics.UpdateMetricsResponse
}
var file_metrics_proto_depIdxs = []int32{
	0, // 0: metrics.Metric.type:type_name -> metrics.Metric.MType
	1, // 1: metrics.UpdateMetricsRequest.metrics:type_name -> metrics.Metric
	2, // 2: metrics.Metrics.UpdateMetrics:input_type -> metrics.UpdateMetricsRequest
	3, // 3: metrics.Metrics.UpdateMetrics:output_type -> metrics.UpdateMetricsResponse
	3, // [3:4] is the sub-list for method output_type
	2, // [2:3] is the sub-list for method input_type
	0, // [0:2] is the sub-list for extension type_name
	0, // [0:2] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_metrics_proto_init() }
func file_metrics_proto_init() {
	if File_metrics_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_metrics_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Metric); i {
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
		file_metrics_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateMetricsRequest); i {
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
		file_metrics_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*UpdateMetricsResponse); i {
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
			RawDescriptor: file_metrics_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_metrics_proto_goTypes,
		DependencyIndexes: file_metrics_proto_depIdxs,
		EnumInfos:         file_metrics_proto_enumTypes,
		MessageInfos:      file_metrics_proto_msgTypes,
	}.Build()
	File_metrics_proto = out.File
	file_metrics_proto_rawDesc = nil
	file_metrics_proto_goTypes = nil
	file_metrics_proto_depIdxs = nil
}
