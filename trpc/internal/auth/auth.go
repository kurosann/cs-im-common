package auth

import (
	"context"
	"fmt"

	"vane-common/trpc/common"

	"google.golang.org/grpc/credentials"
)

var _ credentials.PerRPCCredentials = (*SdkAuthToken)(nil)
var _ credentials.PerRPCCredentials = (*CtxAuthToken)(nil)

type SdkAuthToken struct {
	Token string
	Tls   bool
}

// RequireTransportSecurity 是否需要tls安全传输
func (s SdkAuthToken) RequireTransportSecurity() bool {
	return s.Tls
}

func (s SdkAuthToken) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{common.AuthHeaderKey: toBearerToken(s.Token)}, nil
}

// CtxAuthToken 从 ctx 动态取 token，供「按服务共用一条连接」的池使用。
//
// 每次 RPC 读 ctx 中的 common.Token；为空时用 Fallback（通常是 InternalToken）。
// Dial 时不要再挂固定 SdkAuthToken，否则会和本凭据叠加出两个 Authorization。
type CtxAuthToken struct {
	Fallback string
	Tls      bool
}

func (s CtxAuthToken) RequireTransportSecurity() bool {
	return s.Tls
}

func (s CtxAuthToken) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	token := ""
	if ctx != nil {
		if t, ok := ctx.Value(common.Token).(string); ok {
			token = t
		}
	}
	if token == "" {
		token = s.Fallback
	}
	if token == "" {
		return nil, fmt.Errorf("trpc auth: missing token in context")
	}
	return map[string]string{common.AuthHeaderKey: toBearerToken(token)}, nil
}

func toBearerToken(token string) string {
	return fmt.Sprintf("Bearer %s", token)
}
