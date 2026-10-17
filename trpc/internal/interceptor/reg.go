package interceptor

import (
	"vane-common/trpc/internal/interceptor/client"
	"vane-common/trpc/internal/interceptor/server"

	"google.golang.org/grpc"
)

func ClientInterceptors() []grpc.UnaryClientInterceptor {
	return []grpc.UnaryClientInterceptor{
		client.CtxTransferInterceptor,
	}
}

func StreamClientInterceptors() []grpc.StreamClientInterceptor {
	return []grpc.StreamClientInterceptor{
		client.StreamCtxTransferInterceptor,
	}
}

func ServerInterceptors(serviceName string) []grpc.UnaryServerInterceptor {
	inters := []grpc.UnaryServerInterceptor{
		server.CtxTransferInterceptor,
		server.RecoverInterceptor(),
	}
	inters = append(inters, server.CustomServerInterceptors()...)
	return inters
}

func StreamServerInterceptors(serviceName string) []grpc.StreamServerInterceptor {
	inters := []grpc.StreamServerInterceptor{
		server.StreamCtxTransferInterceptor,
		server.StreamRecoverInterceptor,
	}
	inters = append(inters, server.CustomStreamServerInterceptors()...)
	return inters
}
