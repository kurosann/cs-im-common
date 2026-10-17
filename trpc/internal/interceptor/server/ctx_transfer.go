package server

import (
	"context"
	"strings"

	"vane-common/pkg/idutil"
	"vane-common/pkg/logger"
	"vane-common/trpc/common"
	pkgToken "vane-common/trpc/token"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func CtxTransferInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	requestId := parseRequestId(ctx)
	ctx = context.WithValue(ctx, common.RequestIdKey, requestId)
	callRole, _ := parseCallRole(ctx)
	ctx = context.WithValue(ctx, common.CallRole, callRole)

	// Public 方法不强制鉴权，但 metadata 里若带了 Bearer，仍注入 ctx，
	// 让业务层能从 ctx 取用户身份（而不必再从 proto 字段传一遍）。
	if pkgToken.IsPublic("", info.FullMethod) {
		if token, err := parseToken(ctx); err == nil && token != "" {
			ctx = context.WithValue(ctx, common.Token, token)
		}
		return handler(ctx, req)
	}

	token, err := parseToken(ctx)
	if err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, common.Token, token)
	return handler(ctx, req)
}

func StreamCtxTransferInterceptor(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	requestId := parseRequestId(stream.Context())
	ctx := context.WithValue(stream.Context(), common.RequestIdKey, requestId)
	callRole, _ := parseCallRole(stream.Context())
	ctx = context.WithValue(ctx, common.CallRole, callRole)

	if pkgToken.IsPublic("", info.FullMethod) {
		if token, err := parseToken(stream.Context()); err == nil && token != "" {
			ctx = context.WithValue(ctx, common.Token, token)
		}
		return handler(srv, &WrapServerStream{ctx: ctx, ServerStream: stream})
	}

	token, err := parseToken(stream.Context())
	if err != nil {
		return err
	}
	ctx = context.WithValue(ctx, common.Token, token)
	return handler(srv, &WrapServerStream{ctx: ctx, ServerStream: stream})
}

func parseToken(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}
	token := md.Get(common.AuthHeaderKey)
	if len(token) == 0 {
		return "", status.Error(codes.Unauthenticated, "认证失败")
	}
	splits := strings.SplitN(token[0], " ", 2)
	if len(splits) < 2 {
		logger.Error(ctx, "Bad authorization string", zap.String("token", token[0]))
		return "", status.Errorf(codes.Unauthenticated, "Bad authorization string")
	}
	if !strings.EqualFold(splits[0], common.Bearer) {
		logger.Error(ctx, "Request unauthenticated with bearer", zap.String("token", token[0]))
		return "", status.Errorf(codes.Unauthenticated, "Request unauthenticated with bearer")
	}
	return splits[1], nil
}

func parseRequestId(ctx context.Context) string {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}
	requestId := md.Get(string(common.RequestIdKey))
	if len(requestId) == 0 {
		return idutil.GenUUID()
	}
	return requestId[0]
}

func parseCallRole(ctx context.Context) (string, bool) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.New(nil)
	}
	callRole := md.Get(string(common.CallRole))
	if len(callRole) == 0 {
		return common.UserCallRole, false
	}
	return callRole[0], true
}

type WrapServerStream struct {
	ctx context.Context

	grpc.ServerStream
}

func (w *WrapServerStream) Context() context.Context {
	return w.ctx
}
