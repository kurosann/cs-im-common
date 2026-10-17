package internal

import (
	"context"
	"fmt"
	"net"
	"path"

	"vane-common/module"
	"vane-common/pkg/logger"
	"vane-common/server/pkg/role"
	"vane-common/trpc"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type GrpcRuntime struct {
	serviceName role.ServiceRole
	port        int
	opts        []grpc.ServerOption
	server      *grpc.Server
	listener    net.Listener
}

func NewGrpcRuntime(serviceName role.ServiceRole, port int, opts ...grpc.ServerOption) *GrpcRuntime {
	return &GrpcRuntime{
		serviceName: serviceName,
		port:        port,
		opts:        opts,
	}
}

func (r *GrpcRuntime) Init(ctx context.Context, modules []module.ModuleInfo) error {
	r.server = trpc.NewSvcServer(string(r.serviceName), r.opts...)
	for _, info := range modules {
		isPublic := false
		if publicModule, ok := info.Module.(module.IPublicModule); ok {
			isPublic = publicModule.Public()
		}
		for _, api := range info.Module.Api() {
			if isPublic {
				for _, api := range info.Module.Api() {
					for _, method := range api.Desc.Methods {
						trpc.RegisterPublicService(path.Join("/", api.Desc.ServiceName, method.MethodName))
					}
					for _, method := range api.Desc.Streams {
						trpc.RegisterPublicService(path.Join("/", api.Desc.ServiceName, method.StreamName))
					}
				}
			}
			r.server.RegisterService(&api.Desc, api.Proto)
		}
	}

	if r.port == 0 {
		return nil
	}

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", r.port))
	if err != nil {
		logger.Error(ctx, "listen grpc server error", zap.Error(err))
		return err
	}
	r.listener = listener
	return nil
}

func (r *GrpcRuntime) Listener() net.Listener {
	return r.listener
}

func (r *GrpcRuntime) Server() *grpc.Server {
	return r.server
}
