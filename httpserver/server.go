package httpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"vane-common/module"
	"vane-common/pkg/logger"
)

const (
	defaultReadTimeout     = 15 * time.Second
	defaultWriteTimeout    = 30 * time.Second
	defaultIdleTimeout     = 60 * time.Second
	defaultShutdownTimeout = 5 * time.Second
	defaultRoutePrefix     = "/api"
)

type Config struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}

type MainHTTPServer struct {
	initConfig  func() Config
	modules     []module.ModuleInfo
	globalMW    []module.HttpMiddleware
	routePrefix string
	extraRoutes []module.HttpHandlerRegInfo

	config   Config
	engine   *gin.Engine
	server   *http.Server
	listener net.Listener
}

type Option interface {
	apply(*MainHTTPServer)
}

type optionFunc func(*MainHTTPServer)

func (f optionFunc) apply(server *MainHTTPServer) {
	f(server)
}

func WithGlobalMiddlewares(middlewares ...module.HttpMiddleware) Option {
	return optionFunc(func(server *MainHTTPServer) {
		server.globalMW = append(server.globalMW, middlewares...)
	})
}

func WithRoutePrefix(prefix string) Option {
	return optionFunc(func(server *MainHTTPServer) {
		server.routePrefix = normalizeRoutePrefix(prefix)
	})
}

func WithExtraRoutes(routes ...module.HttpHandlerRegInfo) Option {
	return optionFunc(func(server *MainHTTPServer) {
		server.extraRoutes = append(server.extraRoutes, routes...)
	})
}

func EnableHTTPServer(f func() Config, modules []module.ModuleInfo, opts ...Option) *MainHTTPServer {
	server := &MainHTTPServer{
		initConfig:  f,
		modules:     modules,
		routePrefix: defaultRoutePrefix,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt.apply(server)
	}
	return server
}

func (s *MainHTTPServer) Init(ctx context.Context) error {
	if s.initConfig == nil {
		return errors.New("http server config initializer is nil")
	}

	s.config = normalizeConfig(s.initConfig())
	if s.config.Port == 0 {
		logger.Infof(ctx, "http server disabled: port is 0")
		return nil
	}

	gin.SetMode(gin.ReleaseMode)
	s.engine = gin.New()

	for _, info := range s.modules {
		httpModule, ok := info.Module.(module.IHTTPModule)
		if !ok {
			continue
		}

		httpApiInfo := httpModule.HttpApi()
		modulePrefix := normalizeRoutePrefix(httpApiInfo.Prefix)
		logger.Infof(ctx, "register http routes for module %s", info.Name)

		for _, route := range httpApiInfo.Handlers {
			if route.Handler == nil {
				continue
			}
			handler := applyMiddlewares(route.Handler, s.globalMW, httpApiInfo.Middlewares, route.Middlewares)
			moduleRoutePath := withRoutePrefix(modulePrefix, route.Path)
			fullRoutePath := withRoutePrefix(s.routePrefix, moduleRoutePath)
			s.registerRoute(route.Method, fullRoutePath, handler)
		}
	}

	for _, route := range s.extraRoutes {
		if route.Handler == nil {
			continue
		}
		handler := applyMiddlewares(route.Handler, s.globalMW, route.Middlewares)
		s.registerRoute(route.Method, route.Path, handler)
	}

	s.server = &http.Server{
		Addr:              buildAddr(s.config.Host, s.config.Port),
		Handler:           s.engine,
		ReadTimeout:       s.config.ReadTimeout,
		ReadHeaderTimeout: s.config.ReadTimeout,
		WriteTimeout:      s.config.WriteTimeout,
		IdleTimeout:       s.config.IdleTimeout,
	}

	listener, err := net.Listen("tcp", s.server.Addr)
	if err != nil {
		return err
	}
	s.listener = listener
	return nil
}

func (s *MainHTTPServer) Reload(ctx context.Context) error {
	if s.initConfig == nil {
		return nil
	}
	s.config = normalizeConfig(s.initConfig())
	logger.Infof(ctx, "reload http server config: host=%s port=%d", s.config.Host, s.config.Port)
	return nil
}

func (s *MainHTTPServer) Start(ctx context.Context) error {
	if s.server == nil || s.listener == nil {
		return nil
	}

	logger.Infof(ctx, "http server listen %s", s.listener.Addr().String())
	err := s.server.Serve(s.listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error(ctx, "http server serve error", zap.Error(err))
		return err
	}
	return nil
}

func (s *MainHTTPServer) Stop() {
	if s.server == nil {
		return
	}

	timeout := s.config.ShutdownTimeout
	if timeout <= 0 {
		timeout = defaultShutdownTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := s.server.Shutdown(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error(context.Background(), "http server shutdown error", zap.Error(err))
	}
}

func (s *MainHTTPServer) registerRoute(method string, routePath string, handler module.HttpHandler) {
	if s.engine == nil {
		return
	}

	method = strings.ToUpper(strings.TrimSpace(method))
	routePath = adaptRoutePathForGin(routePath)

	ginHandler := wrapHandler(handler)
	switch method {
	case "", http.MethodGet:
		s.engine.GET(routePath, ginHandler)
	case http.MethodPost:
		s.engine.POST(routePath, ginHandler)
	case http.MethodPut:
		s.engine.PUT(routePath, ginHandler)
	case http.MethodDelete:
		s.engine.DELETE(routePath, ginHandler)
	case http.MethodPatch:
		s.engine.PATCH(routePath, ginHandler)
	case http.MethodHead:
		s.engine.HEAD(routePath, ginHandler)
	case http.MethodOptions:
		s.engine.OPTIONS(routePath, ginHandler)
	default:
		s.engine.Handle(method, routePath, ginHandler)
	}
}

func normalizeConfig(cfg Config) Config {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = defaultReadTimeout
	}
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = defaultWriteTimeout
	}
	if cfg.IdleTimeout <= 0 {
		cfg.IdleTimeout = defaultIdleTimeout
	}
	if cfg.ShutdownTimeout <= 0 {
		cfg.ShutdownTimeout = defaultShutdownTimeout
	}
	return cfg
}

func buildAddr(host string, port int) string {
	if host == "" {
		return fmt.Sprintf(":%d", port)
	}
	return net.JoinHostPort(host, fmt.Sprintf("%d", port))
}

func normalizeRoutePrefix(prefix string) string {
	if prefix == "" || prefix == "/" {
		return ""
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return strings.TrimRight(prefix, "/")
}

func withRoutePrefix(prefix string, routePath string) string {
	if prefix == "" {
		if routePath == "" {
			return "/"
		}
		if strings.HasPrefix(routePath, "/") {
			return path.Clean(routePath)
		}
		return path.Clean("/" + routePath)
	}
	if routePath == "" || routePath == "/" {
		return prefix
	}
	return path.Clean(prefix + "/" + strings.TrimLeft(routePath, "/"))
}

func adaptRoutePathForGin(routePath string) string {
	if routePath == "" {
		return "/"
	}
	return routePath
}

func wrapHandler(handler module.HttpHandler) gin.HandlerFunc {
	return func(c *gin.Context) {
		handler(&reqContext{ctx: c})
	}
}

func applyMiddlewares(handler module.HttpHandler, middlewareSets ...[]module.HttpMiddleware) module.HttpHandler {
	finalHandler := handler
	var middlewares []module.HttpMiddleware
	for _, set := range middlewareSets {
		middlewares = append(middlewares, set...)
	}
	for i := len(middlewares) - 1; i >= 0; i-- {
		if middlewares[i] == nil {
			continue
		}
		finalHandler = middlewares[i](finalHandler)
	}
	return finalHandler
}

type reqContext struct {
	ctx        *gin.Context
	customCtx  context.Context
	statusCode int
}

func (c *reqContext) Context() context.Context {
	if c.customCtx != nil {
		return c.customCtx
	}
	if c.ctx == nil || c.ctx.Request == nil {
		return context.Background()
	}
	return c.ctx.Request.Context()
}

func (c *reqContext) Request() *http.Request {
	if c.ctx == nil {
		return nil
	}
	return c.ctx.Request
}

func (c *reqContext) Query(key string) string {
	if c.ctx == nil {
		return ""
	}
	return c.ctx.Query(key)
}

func (c *reqContext) QueryValues() url.Values {
	if c.ctx == nil || c.ctx.Request == nil || c.ctx.Request.URL == nil {
		return url.Values{}
	}
	return c.ctx.Request.URL.Query()
}

func (c *reqContext) Header(key string) string {
	if c.ctx == nil {
		return ""
	}
	return c.ctx.GetHeader(key)
}

func (c *reqContext) Param(key string) string {
	if c.ctx == nil {
		return ""
	}
	return c.ctx.Param(key)
}

func (c *reqContext) ClientIP() string {
	if c.ctx == nil {
		return ""
	}
	return c.ctx.ClientIP()
}

func (c *reqContext) Cookie(key string) (string, error) {
	if c.ctx == nil {
		return "", errors.New("gin context is nil")
	}
	return c.ctx.Cookie(key)
}

func (c *reqContext) Body() io.ReadCloser {
	if c.ctx == nil || c.ctx.Request == nil {
		return http.NoBody
	}
	return c.ctx.Request.Body
}

func (c *reqContext) BindJSON(target any) error {
	if c.ctx == nil {
		return errors.New("gin context is nil")
	}
	return c.ctx.ShouldBindJSON(target)
}

func (c *reqContext) BindQuery(target any) error {
	if c.ctx == nil || c.ctx.Request == nil || c.ctx.Request.URL == nil {
		return errors.New("gin context is nil")
	}
	return bindValues(c.ctx.Request.URL.Query(), target)
}

func (c *reqContext) BindForm(target any) error {
	if c.ctx == nil || c.ctx.Request == nil {
		return errors.New("gin context is nil")
	}
	if err := c.ctx.Request.ParseForm(); err != nil {
		return err
	}
	return bindValues(c.ctx.Request.Form, target)
}

func (c *reqContext) Value(key any) any {
	return c.Context().Value(key)
}

func (c *reqContext) SetValue(key any, value any) {
	c.customCtx = context.WithValue(c.Context(), key, value)
	if c.ctx != nil && c.ctx.Request != nil {
		c.ctx.Request = c.ctx.Request.WithContext(c.customCtx)
	}
}

func (c *reqContext) SetHeader(key string, value string) {
	if c.ctx == nil {
		return
	}
	c.ctx.Header(key, value)
}

func (c *reqContext) SetCookie(cookie *http.Cookie) {
	if c.ctx == nil || cookie == nil {
		return
	}

	http.SetCookie(c.ctx.Writer, cookie)
}

func (c *reqContext) Status(code int) {
	c.statusCode = code
	if c.ctx == nil {
		return
	}
	c.ctx.Status(code)
}

func (c *reqContext) Write(data []byte) (int, error) {
	if c.ctx == nil {
		return 0, errors.New("gin context is nil")
	}
	if c.statusCode == 0 {
		c.Status(http.StatusOK)
	}
	return c.ctx.Writer.Write(data)
}

func (c *reqContext) SetWriteDeadline(deadline time.Time) error {
	if c.ctx == nil {
		return errors.New("gin context is nil")
	}
	return http.NewResponseController(c.ctx.Writer).SetWriteDeadline(deadline)
}

func (c *reqContext) Flush() error {
	if c.ctx == nil {
		return errors.New("gin context is nil")
	}
	c.ctx.Writer.Flush()
	return nil
}

func (c *reqContext) JSON(code int, payload any) error {
	if c.ctx == nil {
		return errors.New("gin context is nil")
	}
	c.statusCode = code
	c.ctx.JSON(code, payload)
	return nil
}

func (c *reqContext) Ok(data any) {
	_ = c.JSON(http.StatusOK, module.HttpRespBody{
		OK:   true,
		Data: data,
	})
}

func (c *reqContext) Fail(code int, message string) {
	_ = c.JSON(code, module.HttpRespBody{
		OK:      false,
		Message: message,
	})
}

func (c *reqContext) FailData(code int, message string, data any) {
	_ = c.JSON(code, module.HttpRespBody{
		OK:      false,
		Message: message,
		Data:    data,
	})
}

func (c *reqContext) Err(message string) {
	c.Fail(http.StatusBadRequest, message)
}

func (c *reqContext) Redirect(code int, location string) {
	if c.ctx == nil {
		return
	}
	c.statusCode = code
	c.ctx.Redirect(code, location)
}

func (c *reqContext) File(filePath string) {
	if c.ctx == nil {
		return
	}
	c.ctx.File(filePath)
}

func (c *reqContext) Download(filePath string, fileName ...string) {
	if c.ctx == nil {
		return
	}
	if len(fileName) > 0 && strings.TrimSpace(fileName[0]) != "" {
		c.ctx.FileAttachment(filePath, fileName[0])
		return
	}
	c.ctx.File(filePath)
}

func bindValues(values url.Values, target any) error {
	if target == nil {
		return errors.New("bind target is nil")
	}

	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr || targetValue.IsNil() {
		return errors.New("bind target must be a non-nil pointer")
	}

	elem := targetValue.Elem()
	if elem.Kind() != reflect.Struct {
		return errors.New("bind target must point to a struct")
	}

	return bindStructValues(values, elem)
}

func bindStructValues(values url.Values, structValue reflect.Value) error {
	structType := structValue.Type()
	for i := 0; i < structValue.NumField(); i++ {
		fieldValue := structValue.Field(i)
		fieldType := structType.Field(i)

		if fieldType.PkgPath != "" {
			continue
		}

		if fieldValue.Kind() == reflect.Struct {
			if err := bindStructValues(values, fieldValue); err != nil {
				return err
			}
			continue
		}

		key := resolveBindFieldKey(fieldType)
		if key == "" {
			continue
		}

		fieldValues, ok := values[key]
		if !ok || len(fieldValues) == 0 {
			continue
		}

		if err := setFieldValue(fieldValue, fieldValues); err != nil {
			return fmt.Errorf("bind field %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

func resolveBindFieldKey(field reflect.StructField) string {
	for _, tagName := range []string{"form", "query", "json"} {
		tagValue := strings.TrimSpace(field.Tag.Get(tagName))
		if tagValue == "-" {
			return ""
		}
		if tagValue == "" {
			continue
		}
		name := strings.TrimSpace(strings.Split(tagValue, ",")[0])
		if name != "" {
			return name
		}
	}
	return field.Name
}

func setFieldValue(field reflect.Value, values []string) error {
	if !field.CanSet() {
		return nil
	}

	if field.Kind() == reflect.Ptr {
		field.Set(reflect.New(field.Type().Elem()))
		return setFieldValue(field.Elem(), values)
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(values[0])
		return nil
	case reflect.Bool:
		value, err := strconv.ParseBool(values[0])
		if err != nil {
			return err
		}
		field.SetBool(value)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := strconv.ParseInt(values[0], 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetInt(value)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		value, err := strconv.ParseUint(values[0], 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetUint(value)
		return nil
	case reflect.Float32, reflect.Float64:
		value, err := strconv.ParseFloat(values[0], field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetFloat(value)
		return nil
	case reflect.Slice:
		sliceValue := reflect.MakeSlice(field.Type(), 0, len(values))
		for _, item := range values {
			elemValue := reflect.New(field.Type().Elem()).Elem()
			if err := setFieldValue(elemValue, []string{item}); err != nil {
				return err
			}
			sliceValue = reflect.Append(sliceValue, elemValue)
		}
		field.Set(sliceValue)
		return nil
	default:
		return fmt.Errorf("unsupported kind %s", field.Kind())
	}
}
