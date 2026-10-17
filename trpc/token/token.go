package token

import (
	"context"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"strings"
	"vane-common/pkg/logger"
	"vane-common/trpc/common"
	"vane-common/trpc/internal/auth"
)

func ParseToken(ctx context.Context) (string, error) {
	if token, ok := ctx.Value(common.Token).(string); ok {
		return token, nil
	}
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", status.Error(codes.Unauthenticated, "认证失败")
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

func IsPublic(token string, fullMethod string) bool {
	//if token == "" {
	//	if _, ok := auth.PublicTokenAllowedMethods[fullMethod]; ok {
	//		return true
	//	}
	//}

	if _, ok := auth.PublicTokenAllowedMethods[fullMethod]; ok {
		return true
	}
	return false
}

func IsInternal(token string) bool {
	return token == common.InternalToken
}

func IsSkip(token string, fullMethod string) bool {
	if IsPublic(token, fullMethod) {
		return true
	}
	return IsInternal(token)
}
