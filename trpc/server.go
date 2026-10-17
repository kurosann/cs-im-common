package trpc

import (
	"math"
	"time"
	"vane-common/trpc/internal/interceptor/server"

	"vane-common/trpc/internal/auth"
	"vane-common/trpc/internal/interceptor"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
)

var (
	keepaliveParams = keepalive.ServerParameters{
		MaxConnectionIdle: 5 * time.Minute,
	}
	serverCreds credentials.TransportCredentials
)

func SetServerCreds(c credentials.TransportCredentials) {
	serverCreds = c
}

func NewServer(opts ...grpc.ServerOption) *grpc.Server {
	return grpc.NewServer(opts...)
}

func NewSvcServer(serviceName string, opts ...grpc.ServerOption) *grpc.Server {
	baseOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(interceptor.ServerInterceptors(serviceName)...),
		grpc.ChainStreamInterceptor(interceptor.StreamServerInterceptors(serviceName)...),
		grpc.KeepaliveParams(keepaliveParams),
		grpc.MaxSendMsgSize(math.MaxInt32),
		grpc.MaxRecvMsgSize(math.MaxInt32),
	}
	if serverCreds != nil {
		baseOpts = append([]grpc.ServerOption{grpc.Creds(serverCreds)}, baseOpts...)
	}
	baseOpts = append(baseOpts, opts...)
	return NewServer(baseOpts...)
}
func RegisterPublicService(path string) {
	auth.PublicTokenAllowedMethods[path] = true
}

func RegisterServerCustomInterceptor(inter grpc.UnaryServerInterceptor) {
	server.RegisterCustomInterceptor(inter)
}

func RegisterServerStreamCustomInterceptor(inter grpc.StreamServerInterceptor) {
	server.RegisterCustomStreamInterceptor(inter)
}
