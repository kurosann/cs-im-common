package common

import (
	"context"
)

const (
	AuthHeaderKey = "authorization"
	Bearer        = "bearer"
	InternalToken = "tlg-internal"
)

const (
	emptyContext   = "empty-context"
	emptyRequestId = ""
	errorRequestId = "error-request-id"
	emptyCallRole  = ""
	errorCallRole  = "error-call-role"
)

type CtxKey string

const (
	Token          CtxKey = "token"
	CallRole       CtxKey = "call_role"
	RequestIdKey   CtxKey = "X-Request-Id"
	ServiceNameKey CtxKey = "service_name"
)

const (
	SystemCallRole = "system"
	UserCallRole   = "user"
)

// GetRequestId 获取请求ID
func GetRequestId(ctx context.Context) string {
	if ctx == nil {
		return emptyContext
	}
	value := ctx.Value(RequestIdKey)
	if value == nil {
		return emptyRequestId
	}

	_requestId, ok := value.(string)
	if !ok {
		return errorRequestId
	}

	return _requestId
}

func GetCallRole(ctx context.Context) string {
	if ctx == nil {
		return emptyContext
	}
	value := ctx.Value(CallRole)
	if value == nil {
		return emptyCallRole
	}
	_role, ok := value.(string)
	if !ok {
		return errorCallRole
	}
	return _role
}

// GetServiceName 获取服务名称
func GetServiceName(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	value := ctx.Value(ServiceNameKey)
	if value == nil {
		return ""
	}
	serviceName, ok := value.(string)
	if !ok {
		return ""
	}
	return serviceName
}
