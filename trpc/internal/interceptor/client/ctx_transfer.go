package client

import (
	"context"
	"vane-common/pkg/idutil"
	"vane-common/trpc/common"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func CtxTransferInterceptor(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	return invoker(warpMetadata(ctx), method, req, reply, cc, opts...)
}

func warpMetadata(ctx context.Context) context.Context {
	md := getIncomingMetadata(ctx)
	insertRequestId(ctx, md)
	insertCallRole(ctx, md)
	return metadata.NewOutgoingContext(ctx, md)
}

func insertRequestId(ctx context.Context, md metadata.MD) {
	requestId, ok := ctx.Value(common.RequestIdKey).(string)
	if !ok {
		requestId = idutil.GenUUID()
	}
	md.Set(string(common.RequestIdKey), requestId)
}

func insertCallRole(ctx context.Context, md metadata.MD) {
	callRole, ok := ctx.Value(common.CallRole).(string)
	if ok {
		md.Set(string(common.CallRole), callRole)
	}
}

func getIncomingMetadata(ctx context.Context) metadata.MD {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}
	return md
}

func StreamCtxTransferInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return streamer(warpMetadata(ctx), desc, cc, method, opts...)
}
