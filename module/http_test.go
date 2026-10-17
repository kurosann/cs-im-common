package module

import (
	"testing"
)

func TestHttpRouteGroupGroup(t *testing.T) {
	auth := func(next HttpHandler) HttpHandler { return next }
	audit := func(next HttpHandler) HttpHandler { return next }
	handler := func(ctx HttpReqContext) {}

	api := NewHttpRouteGroup("/api/v1", WithGroupMiddlewares(auth))
	apps := api.Group("/apps", WithGroupMiddlewares(audit))
	route := apps.Post("/register", handler)

	if route.Method != HttpMethodPost {
		t.Fatalf("method = %s, want %s", route.Method, HttpMethodPost)
	}
	if route.Path != "/api/v1/apps/register" {
		t.Fatalf("path = %s, want /api/v1/apps/register", route.Path)
	}
	if len(route.Middlewares) != 2 || route.Middlewares[0] == nil || route.Middlewares[1] == nil {
		t.Fatalf("middlewares = %#v, want inherited middleware before group middleware", route.Middlewares)
	}
}

func TestHttpRouteGroupRouteMethodsRegister(t *testing.T) {
	handler := func(ctx HttpReqContext) {}
	group := NewHttpRouteGroup("/api/v1/apps")

	group.Post("/register", handler)
	group.Get("", handler)
	handlers := group.Handlers()

	if len(handlers) != 2 {
		t.Fatalf("handler count = %d, want 2", len(handlers))
	}
	if handlers[0].Path != "/api/v1/apps/register" {
		t.Fatalf("registered path = %s, want /api/v1/apps/register", handlers[0].Path)
	}
}

func TestHttpRouteGroupMergeKeepsPaths(t *testing.T) {
	handler := func(ctx HttpReqContext) {}

	left := NewHttpRouteGroup("/api/v1")
	right := NewHttpRouteGroup("/apps")
	left.Post("/health", handler)
	right.Post("/register", handler)

	handlers := left.Merge(right)
	if len(handlers) != 2 {
		t.Fatalf("handler count = %d, want 2", len(handlers))
	}
	if handlers[0].Path != "/api/v1/health" || handlers[1].Path != "/apps/register" {
		t.Fatalf("paths = %s, %s; want unchanged paths", handlers[0].Path, handlers[1].Path)
	}
}

func TestHttpRouteGroupComposeComposesPrefixAndMiddlewares(t *testing.T) {
	auth := func(next HttpHandler) HttpHandler { return next }
	audit := func(next HttpHandler) HttpHandler { return next }
	handler := func(ctx HttpReqContext) {}

	parent := NewHttpRouteGroup("/api/v1", WithGroupMiddlewares(auth))
	child := NewHttpRouteGroup("/apps", WithGroupMiddlewares(audit))
	child.Post("/register", handler)
	composed := parent.Compose(child)
	handlers := composed.Handlers()
	if len(handlers) != 1 {
		t.Fatalf("handler count = %d, want 1", len(handlers))
	}
	if handlers[0].Path != "/api/v1/apps/register" {
		t.Fatalf("path = %s, want /api/v1/apps/register", handlers[0].Path)
	}
	if len(handlers[0].Middlewares) != 2 {
		t.Fatalf("middleware count = %d, want 2", len(handlers[0].Middlewares))
	}
}

func TestMergeHandlers(t *testing.T) {
	handler := func(ctx HttpReqContext) {}
	first := []HttpHandlerRegInfo{{Path: "/api/v1/health", Handler: handler}}
	second := []HttpHandlerRegInfo{{Path: "/ws/v1/realtime", Handler: handler}}

	handlers := MergeHandlers(first, second)
	if len(handlers) != 2 {
		t.Fatalf("handler count = %d, want 2", len(handlers))
	}
	if handlers[0].Path != "/api/v1/health" || handlers[1].Path != "/ws/v1/realtime" {
		t.Fatalf("paths = %s, %s; want combined handlers", handlers[0].Path, handlers[1].Path)
	}
}
