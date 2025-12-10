// Code generated manually for metrics proto gRPC bindings. DO NOT EDIT.
package proto

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

const (
	_ = grpc.SupportPackageIsVersion7
)

// MetricsClient is the client API for Metrics service.
type MetricsClient interface {
	UpdateMetrics(ctx context.Context, in *UpdateMetricsRequest, opts ...grpc.CallOption) (*UpdateMetricsResponse, error)
}

type metricsClient struct {
	cc grpc.ClientConnInterface
}

func NewMetricsClient(cc grpc.ClientConnInterface) MetricsClient {
	return &metricsClient{cc}
}

func (c *metricsClient) UpdateMetrics(ctx context.Context, in *UpdateMetricsRequest, opts ...grpc.CallOption) (*UpdateMetricsResponse, error) {
	out := new(UpdateMetricsResponse)
	err := c.cc.Invoke(ctx, "/metrics.Metrics/UpdateMetrics", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MetricsServer is the server API for Metrics service.
type MetricsServer interface {
	UpdateMetrics(context.Context, *UpdateMetricsRequest) (*UpdateMetricsResponse, error)
	mustEmbedUnimplementedMetricsServer()
}

type UnimplementedMetricsServer struct{}

func (UnimplementedMetricsServer) UpdateMetrics(context.Context, *UpdateMetricsRequest) (*UpdateMetricsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateMetrics not implemented")
}
func (UnimplementedMetricsServer) mustEmbedUnimplementedMetricsServer() {}

// UnsafeMetricsServer may be embedded to opt out of forward compatibility.
type UnsafeMetricsServer interface {
	mustEmbedUnimplementedMetricsServer()
}

func RegisterMetricsServer(s grpc.ServiceRegistrar, srv MetricsServer) {
	s.RegisterService(&Metrics_ServiceDesc, srv)
}

func _Metrics_UpdateMetrics_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateMetricsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MetricsServer).UpdateMetrics(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/metrics.Metrics/UpdateMetrics",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MetricsServer).UpdateMetrics(ctx, req.(*UpdateMetricsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// Metrics_ServiceDesc is the grpc.ServiceDesc for Metrics service.
var Metrics_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "metrics.Metrics",
	HandlerType: (*MetricsServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "UpdateMetrics",
			Handler:    _Metrics_UpdateMetrics_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "metrics.proto",
}
