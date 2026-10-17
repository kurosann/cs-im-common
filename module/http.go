package module

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type IHTTPModule interface {
	HttpApi() HttpApiInfo
}

type HttpReqContext interface {
	Context() context.Context
	Request() *http.Request
	Query(key string) string
	QueryValues() url.Values
	Header(key string) string
	Param(key string) string
	ClientIP() string
	Cookie(key string) (string, error)
	Body() io.ReadCloser
	BindJSON(target any) error
	BindQuery(target any) error
	BindForm(target any) error
	Value(key any) any
	SetValue(key any, value any)
	SetHeader(key string, value string)
	SetCookie(cookie *http.Cookie)
	Status(code int)
	Write(data []byte) (int, error)
	// SetWriteDeadline 设置响应的写入截止时间；传 time.Time{} 表示清除截止时间，
	// 用于长流式响应（如 SSE）需要关闭框架 write timeout 的场景。
	SetWriteDeadline(deadline time.Time) error
	// Flush 将缓冲的响应数据立即刷给客户端；流式响应（SSE）每写一块后调用。
	Flush() error
	JSON(code int, payload any) error
	Ok(data any)
	Fail(code int, message string)
	FailData(code int, message string, data any)
	Err(message string)
	Redirect(code int, location string)
	File(filePath string)
	Download(filePath string, fileName ...string)
}

type (
	HttpHandler    func(ctx HttpReqContext)
	HttpMiddleware func(next HttpHandler) HttpHandler
)

type HttpRespBody struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

const (
	HttpMethodGet     = "GET"
	HttpMethodPost    = "POST"
	HttpMethodPut     = "PUT"
	HttpMethodDelete  = "DELETE"
	HttpMethodPatch   = "PATCH"
	HttpMethodHead    = "HEAD"
	HttpMethodOptions = "OPTIONS"
)

type HttpHandlerRegInfo struct {
	Method      string
	Path        string
	Handler     HttpHandler
	Middlewares []HttpMiddleware
}

type HttpRouteGroup struct {
	prefix      string
	middlewares []HttpMiddleware
	handlers    []HttpHandlerRegInfo
}

type HttpApiInfo struct {
	// 前缀
	Prefix string
	// 处理
	Handlers []HttpHandlerRegInfo
	// 中间件
	Middlewares []HttpMiddleware
}

type HttpHandlerRegOption interface {
	apply(*HttpHandlerRegInfo)
}

type httpHandlerRegOptionFunc func(*HttpHandlerRegInfo)

func (f httpHandlerRegOptionFunc) apply(info *HttpHandlerRegInfo) {
	f(info)
}

func WithMiddlewares(middlewares ...HttpMiddleware) HttpHandlerRegOption {
	return httpHandlerRegOptionFunc(func(info *HttpHandlerRegInfo) {
		info.Middlewares = append(info.Middlewares, middlewares...)
	})
}

func newHttpHandlerRegInfo(method string, routePath string, handler HttpHandler, opts ...HttpHandlerRegOption) HttpHandlerRegInfo {
	info := HttpHandlerRegInfo{
		Method:  method,
		Path:    routePath,
		Handler: handler,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.apply(&info)
	}
	return info
}

func (r HttpRouteGroup) Handle(method string, routePath string, handler HttpHandler, opts ...HttpHandlerRegOption) HttpHandlerRegInfo {
	info := newHttpHandlerRegInfo(method, joinRoutePath(r.prefix, routePath), handler, opts...)
	if len(r.middlewares) > 0 {
		info.Middlewares = append(append(make([]HttpMiddleware, 0, len(r.middlewares)+len(info.Middlewares)), r.middlewares...), info.Middlewares...)
	}
	return info
}

// NewReqHttpRegInfo 创建顶层 HTTP 路由注册信息。
// 模块层路由统一使用 :name 表示路径参数，使用 *name 表示通配参数，
// 例如 "/session/:id"、"/files/*path"。
func NewReqHttpRegInfo(method string, routePath string, handler HttpHandler, opts ...HttpHandlerRegOption) HttpHandlerRegInfo {
	return HttpRouteGroup{}.Handle(method, routePath, handler, opts...)
}

// NewGetReqHttpRegInfo 创建顶层 GET 路由注册信息。
// 路径参数请使用 :name 形式，例如 "/session/:id"。
func NewGetReqHttpRegInfo(routePath string, handler HttpHandler, opts ...HttpHandlerRegOption) HttpHandlerRegInfo {
	return NewReqHttpRegInfo(HttpMethodGet, routePath, handler, opts...)
}

// NewPostReqHttpRegInfo 创建顶层 POST 路由注册信息。
func NewPostReqHttpRegInfo(routePath string, handler HttpHandler, opts ...HttpHandlerRegOption) HttpHandlerRegInfo {
	return NewReqHttpRegInfo(HttpMethodPost, routePath, handler, opts...)
}

// NewPutReqHttpRegInfo 创建顶层 PUT 路由注册信息。
func NewPutReqHttpRegInfo(routePath string, handler HttpHandler, opts ...HttpHandlerRegOption) HttpHandlerRegInfo {
	return NewReqHttpRegInfo(HttpMethodPut, routePath, handler, opts...)
}

// NewDeleteReqHttpRegInfo 创建顶层 DELETE 路由注册信息。
func NewDeleteReqHttpRegInfo(routePath string, handler HttpHandler, opts ...HttpHandlerRegOption) HttpHandlerRegInfo {
	return NewReqHttpRegInfo(HttpMethodDelete, routePath, handler, opts...)
}

// NewPatchReqHttpRegInfo 创建顶层 PATCH 路由注册信息。
func NewPatchReqHttpRegInfo(routePath string, handler HttpHandler, opts ...HttpHandlerRegOption) HttpHandlerRegInfo {
	return NewReqHttpRegInfo(HttpMethodPatch, routePath, handler, opts...)
}

// NewHeadReqHttpRegInfo 创建顶层 HEAD 路由注册信息。
func NewHeadReqHttpRegInfo(routePath string, handler HttpHandler, opts ...HttpHandlerRegOption) HttpHandlerRegInfo {
	return NewReqHttpRegInfo(HttpMethodHead, routePath, handler, opts...)
}

// NewOptionsReqHttpRegInfo 创建顶层 OPTIONS 路由注册信息。
func NewOptionsReqHttpRegInfo(routePath string, handler HttpHandler, opts ...HttpHandlerRegOption) HttpHandlerRegInfo {
	return NewReqHttpRegInfo(HttpMethodOptions, routePath, handler, opts...)
}

func (g *HttpRouteGroup) Get(routePath string, handler HttpHandler, opts ...HttpHandlerRegOption) HttpHandlerRegInfo {
	info := g.Handle(HttpMethodGet, routePath, handler, opts...)
	g.handlers = append(g.handlers, info)
	return info
}

func (g *HttpRouteGroup) Post(routePath string, handler HttpHandler, opts ...HttpHandlerRegOption) HttpHandlerRegInfo {
	info := g.Handle(HttpMethodPost, routePath, handler, opts...)
	g.handlers = append(g.handlers, info)
	return info
}

func (g *HttpRouteGroup) Put(routePath string, handler HttpHandler, opts ...HttpHandlerRegOption) HttpHandlerRegInfo {
	info := g.Handle(HttpMethodPut, routePath, handler, opts...)
	g.handlers = append(g.handlers, info)
	return info
}

func (g *HttpRouteGroup) Delete(routePath string, handler HttpHandler, opts ...HttpHandlerRegOption) HttpHandlerRegInfo {
	info := g.Handle(HttpMethodDelete, routePath, handler, opts...)
	g.handlers = append(g.handlers, info)
	return info
}

func (g *HttpRouteGroup) Patch(routePath string, handler HttpHandler, opts ...HttpHandlerRegOption) HttpHandlerRegInfo {
	info := g.Handle(HttpMethodPatch, routePath, handler, opts...)
	g.handlers = append(g.handlers, info)
	return info
}

func (g *HttpRouteGroup) Head(routePath string, handler HttpHandler, opts ...HttpHandlerRegOption) HttpHandlerRegInfo {
	info := g.Handle(HttpMethodHead, routePath, handler, opts...)
	g.handlers = append(g.handlers, info)
	return info
}

func (g *HttpRouteGroup) Options(routePath string, handler HttpHandler, opts ...HttpHandlerRegOption) HttpHandlerRegInfo {
	info := g.Handle(HttpMethodOptions, routePath, handler, opts...)
	g.handlers = append(g.handlers, info)
	return info
}

type HttpRouteGroupOption interface {
	applyGroup(*HttpRouteGroup)
}

type httpRouteGroupOptionFunc func(*HttpRouteGroup)

func (f httpRouteGroupOptionFunc) applyGroup(group *HttpRouteGroup) {
	f(group)
}

func WithGroupMiddlewares(middlewares ...HttpMiddleware) HttpRouteGroupOption {
	return httpRouteGroupOptionFunc(func(group *HttpRouteGroup) {
		group.middlewares = append(group.middlewares, middlewares...)
	})
}

func NewHttpRouteGroup(prefix string, opts ...HttpRouteGroupOption) HttpRouteGroup {
	group := HttpRouteGroup{prefix: normalizeGroupPrefix(prefix)}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.applyGroup(&group)
	}
	return group
}

func (g HttpRouteGroup) Group(prefix string, opts ...HttpRouteGroupOption) HttpRouteGroup {
	group := NewHttpRouteGroup(joinRoutePath(g.prefix, prefix), opts...)
	group.middlewares = append(
		append(make([]HttpMiddleware, 0, len(g.middlewares)+len(group.middlewares)), g.middlewares...),
		group.middlewares...,
	)
	return group
}

func (g *HttpRouteGroup) Handlers() []HttpHandlerRegInfo {
	return append(make([]HttpHandlerRegInfo, 0, len(g.handlers)), g.handlers...)
}

func (g HttpRouteGroup) Merge(groups ...HttpRouteGroup) []HttpHandlerRegInfo {
	combined := g.Handlers()
	for _, group := range groups {
		combined = append(combined, group.Handlers()...)
	}
	return combined
}

func (g HttpRouteGroup) Compose(groups ...HttpRouteGroup) HttpRouteGroup {
	combined := g
	for _, group := range groups {
		for _, handler := range group.handlers {
			handler.Path = joinRoutePath(g.prefix, handler.Path)
			handler.Middlewares = append(append(
				make([]HttpMiddleware, 0, len(g.middlewares)+len(handler.Middlewares)),
				g.middlewares...,
			), handler.Middlewares...)
			combined.handlers = append(combined.handlers, handler)
		}
	}
	return combined
}

func MergeHandlers(handlerSets ...[]HttpHandlerRegInfo) []HttpHandlerRegInfo {
	total := 0
	for _, handlers := range handlerSets {
		total += len(handlers)
	}
	combined := make([]HttpHandlerRegInfo, 0, total)
	for _, handlers := range handlerSets {
		combined = append(combined, handlers...)
	}
	return combined
}

func normalizeGroupPrefix(prefix string) string {
	if prefix == "" || prefix == "/" {
		return ""
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return strings.TrimRight(prefix, "/")
}

func joinRoutePath(prefix string, routePath string) string {
	if prefix == "" {
		if routePath == "" {
			return "/"
		}
		if strings.HasPrefix(routePath, "/") {
			return routePath
		}
		return "/" + routePath
	}
	if routePath == "" || routePath == "/" {
		return prefix
	}
	return path.Clean(prefix + "/" + strings.TrimLeft(routePath, "/"))
}
