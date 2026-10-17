package server

import (
	"context"
	"vane-common/pkg/logger"
	"vane-common/pkg/safe"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func RecoverInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		if panicErr := safe.Try(func() { resp, err = handler(ctx, req) }); panicErr != nil {
			logger.Errorf(ctx, "grpc panic: %v", panicErr)
			return nil, status.Errorf(codes.Internal, "grpc panic: %v", panicErr)
		}
		if err != nil {
			if _, ok := status.FromError(err); ok {
				return nil, err
			}
			err = status.Errorf(codes.Internal, "err: %v", err)
			return nil, err
		}
		return resp, err
	}
}

func StreamRecoverInterceptor(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
	if panicErr := safe.Try(func() { err = handler(srv, stream) }); panicErr != nil {
		logger.Errorf(stream.Context(), "grpc panic: %v", panicErr)
		return status.Errorf(codes.Internal, "grpc panic: %v", panicErr)
	}
	return err
}
