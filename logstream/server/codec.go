package main

import (
	stdjson "encoding/json"

	grpcencoding "google.golang.org/grpc/encoding"
)

func init() {
	// Register JSON codec so gRPC messages are serialized as JSON.
	// Must match the codec registered by the agent.
	grpcencoding.RegisterCodec(jsonCodec{})
}

type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }

func (jsonCodec) Marshal(v interface{}) ([]byte, error) {
	return stdjson.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v interface{}) error {
	return stdjson.Unmarshal(data, v)
}
