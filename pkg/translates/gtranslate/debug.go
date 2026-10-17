package gtranslate

import (
	"context"
	"fmt"
	"net/http"

	"vane-common/pkg/logger"

	"go.uber.org/zap"
)

const debugGTranslateHTTPLogPrefix = "[DEBUG-gtranslate-http]"

type HTTPClientDebugInfo struct {
	ClientPtr             string `json:"clientPtr"`
	TransportPtr          string `json:"transportPtr,omitempty"`
	TransportType         string `json:"transportType,omitempty"`
	Timeout               string `json:"timeout,omitempty"`
	DisableKeepAlives     bool   `json:"disableKeepAlives,omitempty"`
	MaxIdleConns          int    `json:"maxIdleConns,omitempty"`
	MaxIdleConnsPerHost   int    `json:"maxIdleConnsPerHost,omitempty"`
	IdleConnTimeout       string `json:"idleConnTimeout,omitempty"`
	ExpectContinueTimeout string `json:"expectContinueTimeout,omitempty"`
}

func DescribeHTTPClient(client *http.Client) HTTPClientDebugInfo {
	if client == nil {
		return HTTPClientDebugInfo{ClientPtr: "<nil>"}
	}

	info := HTTPClientDebugInfo{
		ClientPtr: fmt.Sprintf("%p", client),
		Timeout:   client.Timeout.String(),
	}

	if client.Transport == nil {
		return info
	}

	info.TransportPtr = fmt.Sprintf("%p", client.Transport)
	info.TransportType = fmt.Sprintf("%T", client.Transport)

	if transport, ok := client.Transport.(*http.Transport); ok {
		info.DisableKeepAlives = transport.DisableKeepAlives
		info.MaxIdleConns = transport.MaxIdleConns
		info.MaxIdleConnsPerHost = transport.MaxIdleConnsPerHost
		info.IdleConnTimeout = transport.IdleConnTimeout.String()
		info.ExpectContinueTimeout = transport.ExpectContinueTimeout.String()
	}

	return info
}

func logHTTPClientRequest(ctx context.Context, msg string, client *http.Client, req *http.Request, fields ...zap.Field) {
	info := DescribeHTTPClient(client)
	baseFields := []zap.Field{
		zap.String("host", req.URL.Host),
		zap.String("path", req.URL.Path),
		zap.Any("httpClient", info),
	}
	baseFields = append(baseFields, fields...)
	logger.Debug(ctx, msg, baseFields...)
}

func logHTTPClientRequestError(ctx context.Context, msg string, client *http.Client, req *http.Request, err error, fields ...zap.Field) {
	info := DescribeHTTPClient(client)
	baseFields := []zap.Field{
		zap.Error(err),
		zap.String("host", req.URL.Host),
		zap.String("path", req.URL.Path),
		zap.Any("httpClient", info),
	}
	baseFields = append(baseFields, fields...)
	logger.Error(ctx, msg, baseFields...)
}

func logHTTPClientResponse(ctx context.Context, msg string, client *http.Client, req *http.Request, resp *http.Response, fields ...zap.Field) {
	info := DescribeHTTPClient(client)
	baseFields := []zap.Field{
		zap.Int("statusCode", resp.StatusCode),
		zap.String("host", req.URL.Host),
		zap.String("path", req.URL.Path),
		zap.Any("httpClient", info),
	}
	baseFields = append(baseFields, fields...)
	logger.Debug(ctx, msg, baseFields...)
}
