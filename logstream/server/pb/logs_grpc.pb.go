// Code generated manually. DO NOT EDIT.
// Source: logs.proto

package pb

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// LogServiceClient is the client API for LogService service.
type LogServiceClient interface {
	Register(ctx context.Context, in *RegisterRequest, opts ...grpc.CallOption) (*RegisterResponse, error)
	StreamLogs(ctx context.Context, opts ...grpc.CallOption) (LogService_StreamLogsClient, error)
	Heartbeat(ctx context.Context, in *HeartbeatRequest, opts ...grpc.CallOption) (*HeartbeatResponse, error)
	Subscribe(ctx context.Context, in *SubscribeRequest, opts ...grpc.CallOption) (LogService_SubscribeClient, error)
	Query(ctx context.Context, in *QueryRequest, opts ...grpc.CallOption) (*QueryResponse, error)
	UpdateConfig(ctx context.Context, in *UpdateConfigRequest, opts ...grpc.CallOption) (*UpdateConfigResponse, error)
}

type logServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewLogServiceClient(cc grpc.ClientConnInterface) LogServiceClient {
	return &logServiceClient{cc}
}

func (c *logServiceClient) Register(ctx context.Context, in *RegisterRequest, opts ...grpc.CallOption) (*RegisterResponse, error) {
	out := new(RegisterResponse)
	err := c.cc.Invoke(ctx, "/pb.LogService/Register", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *logServiceClient) StreamLogs(ctx context.Context, opts ...grpc.CallOption) (LogService_StreamLogsClient, error) {
	stream, err := c.cc.NewStream(ctx, &LogService_ServiceDesc.Streams[0], "/pb.LogService/StreamLogs", opts...)
	if err != nil {
		return nil, err
	}
	x := &logServiceStreamLogsClient{stream}
	return x, nil
}

type LogService_StreamLogsClient interface {
	Send(*LogBatch) error
	Recv() (*StreamResponse, error)
	grpc.ClientStream
}

type logServiceStreamLogsClient struct {
	grpc.ClientStream
}

func (x *logServiceStreamLogsClient) Send(m *LogBatch) error {
	return x.ClientStream.SendMsg(m)
}

func (x *logServiceStreamLogsClient) Recv() (*StreamResponse, error) {
	m := new(StreamResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *logServiceClient) Heartbeat(ctx context.Context, in *HeartbeatRequest, opts ...grpc.CallOption) (*HeartbeatResponse, error) {
	out := new(HeartbeatResponse)
	err := c.cc.Invoke(ctx, "/pb.LogService/Heartbeat", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *logServiceClient) Subscribe(ctx context.Context, in *SubscribeRequest, opts ...grpc.CallOption) (LogService_SubscribeClient, error) {
	stream, err := c.cc.NewStream(ctx, &LogService_ServiceDesc.Streams[1], "/pb.LogService/Subscribe", opts...)
	if err != nil {
		return nil, err
	}
	x := &logServiceSubscribeClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type LogService_SubscribeClient interface {
	Recv() (*LogEntry, error)
	grpc.ClientStream
}

type logServiceSubscribeClient struct {
	grpc.ClientStream
}

func (x *logServiceSubscribeClient) Recv() (*LogEntry, error) {
	m := new(LogEntry)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

func (c *logServiceClient) Query(ctx context.Context, in *QueryRequest, opts ...grpc.CallOption) (*QueryResponse, error) {
	out := new(QueryResponse)
	err := c.cc.Invoke(ctx, "/pb.LogService/Query", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *logServiceClient) UpdateConfig(ctx context.Context, in *UpdateConfigRequest, opts ...grpc.CallOption) (*UpdateConfigResponse, error) {
	out := new(UpdateConfigResponse)
	err := c.cc.Invoke(ctx, "/pb.LogService/UpdateConfig", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// LogServiceServer is the server API for LogService service.
type LogServiceServer interface {
	Register(context.Context, *RegisterRequest) (*RegisterResponse, error)
	StreamLogs(LogService_StreamLogsServer) error
	Heartbeat(context.Context, *HeartbeatRequest) (*HeartbeatResponse, error)
	Subscribe(*SubscribeRequest, LogService_SubscribeServer) error
	Query(context.Context, *QueryRequest) (*QueryResponse, error)
	UpdateConfig(context.Context, *UpdateConfigRequest) (*UpdateConfigResponse, error)
	mustEmbedUnimplementedLogServiceServer()
}

// UnimplementedLogServiceServer must be embedded to have forward compatible implementations.
type UnimplementedLogServiceServer struct{}

func (UnimplementedLogServiceServer) Register(context.Context, *RegisterRequest) (*RegisterResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Register not implemented")
}
func (UnimplementedLogServiceServer) StreamLogs(LogService_StreamLogsServer) error {
	return status.Errorf(codes.Unimplemented, "method StreamLogs not implemented")
}
func (UnimplementedLogServiceServer) Heartbeat(context.Context, *HeartbeatRequest) (*HeartbeatResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Heartbeat not implemented")
}
func (UnimplementedLogServiceServer) Subscribe(*SubscribeRequest, LogService_SubscribeServer) error {
	return status.Errorf(codes.Unimplemented, "method Subscribe not implemented")
}
func (UnimplementedLogServiceServer) Query(context.Context, *QueryRequest) (*QueryResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Query not implemented")
}
func (UnimplementedLogServiceServer) UpdateConfig(context.Context, *UpdateConfigRequest) (*UpdateConfigResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UpdateConfig not implemented")
}
func (UnimplementedLogServiceServer) mustEmbedUnimplementedLogServiceServer() {}

// UnsafeLogServiceServer may be embedded to opt out of forward compatibility for this service.
type UnsafeLogServiceServer interface {
	mustEmbedUnimplementedLogServiceServer()
}

type LogService_StreamLogsServer interface {
	Send(*StreamResponse) error
	Recv() (*LogBatch, error)
	grpc.ServerStream
}

type logServiceStreamLogsServer struct {
	grpc.ServerStream
}

func (x *logServiceStreamLogsServer) Send(m *StreamResponse) error {
	return x.ServerStream.SendMsg(m)
}

func (x *logServiceStreamLogsServer) Recv() (*LogBatch, error) {
	m := new(LogBatch)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

type LogService_SubscribeServer interface {
	Send(*LogEntry) error
	grpc.ServerStream
}

type logServiceSubscribeServer struct {
	grpc.ServerStream
}

func (x *logServiceSubscribeServer) Send(m *LogEntry) error {
	return x.ServerStream.SendMsg(m)
}

func _LogService_Register_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RegisterRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LogServiceServer).Register(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.LogService/Register",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LogServiceServer).Register(ctx, req.(*RegisterRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LogService_StreamLogs_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(LogServiceServer).StreamLogs(&logServiceStreamLogsServer{stream})
}

func _LogService_Heartbeat_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HeartbeatRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LogServiceServer).Heartbeat(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.LogService/Heartbeat",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LogServiceServer).Heartbeat(ctx, req.(*HeartbeatRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LogService_Subscribe_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(SubscribeRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(LogServiceServer).Subscribe(m, &logServiceSubscribeServer{stream})
}

func _LogService_Query_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LogServiceServer).Query(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.LogService/Query",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LogServiceServer).Query(ctx, req.(*QueryRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _LogService_UpdateConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UpdateConfigRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(LogServiceServer).UpdateConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/pb.LogService/UpdateConfig",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(LogServiceServer).UpdateConfig(ctx, req.(*UpdateConfigRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// LogService_ServiceDesc is the grpc.ServiceDesc for LogService service.
var LogService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "pb.LogService",
	HandlerType: (*LogServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Register",
			Handler:    _LogService_Register_Handler,
		},
		{
			MethodName: "Heartbeat",
			Handler:    _LogService_Heartbeat_Handler,
		},
		{
			MethodName: "Query",
			Handler:    _LogService_Query_Handler,
		},
		{
			MethodName: "UpdateConfig",
			Handler:    _LogService_UpdateConfig_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StreamLogs",
			Handler:       _LogService_StreamLogs_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
		{
			StreamName:    "Subscribe",
			Handler:       _LogService_Subscribe_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "logs.proto",
}

// RegisterLogServiceServer registers the server implementation with the gRPC server.
func RegisterLogServiceServer(s grpc.ServiceRegistrar, srv LogServiceServer) {
	s.RegisterService(&LogService_ServiceDesc, srv)
}
