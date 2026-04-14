// Code generated manually. DO NOT EDIT.
// Source: logs.proto

package pb

import (
	"context"

	"google.golang.org/grpc"
)

// LogServiceClient is the client API for LogService service.
type LogServiceClient interface {
	Register(ctx context.Context, in *RegisterRequest, opts ...grpc.CallOption) (*RegisterResponse, error)
	StreamLogs(ctx context.Context, opts ...grpc.CallOption) (LogService_StreamLogsClient, error)
	Heartbeat(ctx context.Context, in *HeartbeatRequest, opts ...grpc.CallOption) (*HeartbeatResponse, error)
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
	stream, err := c.cc.NewStream(ctx, &logServiceStreamDesc, "/pb.LogService/StreamLogs", opts...)
	if err != nil {
		return nil, err
	}
	x := &logServiceStreamLogsClient{stream}
	return x, nil
}

// logServiceStreamDesc describes the StreamLogs bidirectional stream.
var logServiceStreamDesc = grpc.StreamDesc{
	StreamName:    "StreamLogs",
	ServerStreams: true,
	ClientStreams: true,
}

// LogService_StreamLogsClient is the client-side stream for StreamLogs.
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
