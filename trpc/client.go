package trpc

import (
	"context"
	"errors"
	"maps"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/proxy"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"vane-common/pkg/idutil"
	"vane-common/pkg/safe"
	"vane-common/trpc/common"
	"vane-common/trpc/internal/auth"
	"vane-common/trpc/internal/interceptor"
	internalResolver "vane-common/trpc/internal/resolver"
	"vane-common/v1/pilot/system"

	"vane-common/pkg/logger"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/resolver"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	globalResolverMode DiscoverResolverMode = LbDiscoverResolverMode

	globalClientPool = NewClientPool(globalResolverMode)

	// globalSvcClientPool 按服务分池（身份从 ctx 动态挂），与 globalClientPool 并存。
	globalSvcClientPool = NewSvcClientPool(globalResolverMode)

	// 全局代理 dialer
	proxyDialer atomic.Value // proxies.Dialer
)

// SetProxyDialer 设置全局代理 dialer
func SetProxyDialer(dialer proxy.Dialer) {
	proxyDialer.Store(dialer)
}

// GetProxyDialer 获取全局代理 dialer
func GetProxyDialer() proxy.Dialer {
	if v := proxyDialer.Load(); v != nil {
		return v.(proxy.Dialer)
	}
	return nil
}

// GetGlobalClientPool 获取全局客户端连接池，用于优雅关闭
func GetGlobalClientPool() *ClientPool {
	return globalClientPool
}

// GetGlobalSvcClientPool 获取按服务分池的全局连接池（身份从 ctx 动态挂）。
func GetGlobalSvcClientPool() *SvcClientPool {
	return globalSvcClientPool
}

func NewClient(target string, opts ...grpc.DialOption) (*CanCloseClientConn, error) {
	defaultOpts := []grpc.DialOption{
		grpc.WithKeepaliveParams(keepalive.ClientParameters{
			Time:    30 * time.Second,
			Timeout: 5 * time.Second,
		}),
		grpc.WithConnectParams(grpc.ConnectParams{
			Backoff: backoff.Config{
				BaseDelay:  1.0 * time.Second,
				Multiplier: 1.6,
				Jitter:     0.2,
				MaxDelay:   30 * time.Second,
			},
			MinConnectTimeout: 5 * time.Second,
		}),
		grpc.WithIdleTimeout(300 * time.Second),
	}

	// dialer 选择：优先全局代理；无代理时强制 IPv4——外部 target（如 api.forscrm.com:443
	// 走 Cloudflare）在无 v6 路由的环境下会因 AAAA 优先导致拨号失败，内部服务均为 v4 不受影响。
	if dialer := GetProxyDialer(); dialer != nil {
		defaultOpts = append(defaultOpts, grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			if contextDialer, ok := dialer.(proxy.ContextDialer); ok {
				return contextDialer.DialContext(ctx, "tcp", addr)
			}
			return dialer.Dial("tcp", addr)
		}))
	} else {
		defaultOpts = append(defaultOpts, grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp4", addr)
		}))
	}

	opts = append(defaultOpts, opts...)
	conn, err := grpc.NewClient(target, opts...)
	if err != nil {
		return nil, err
	}
	return &CanCloseClientConn{ClientConn: conn}, nil
}

type config struct {
	cred credentials.TransportCredentials
	mode DiscoverResolverMode
}

type TrpcOption interface {
	apply(opts *config)
}

type trpcOptionFunc func(opts *config)

func (o trpcOptionFunc) apply(opts *config) {
	o(opts)
}
func WithDiscoverResolverMode(mode DiscoverResolverMode) TrpcOption {
	return trpcOptionFunc(func(opts *config) {
		opts.mode = mode
	})
}
func WithCred(cred credentials.TransportCredentials) TrpcOption {
	return trpcOptionFunc(func(opts *config) {
		opts.cred = cred
	})
}

func NewTokenClient(svcName SvcName, token string, opts ...TrpcOption) (*CanCloseClientConn, error) {
	cfg := &config{
		mode: LbDiscoverResolverMode,
		cred: insecure.NewCredentials(),
	}
	for _, o := range opts {
		o.apply(cfg)
	}
	target := ""
	switch cfg.mode {
	case LbDiscoverResolverMode:
		target = buildDiscoverTarget(svcName)
	case FailoverDiscoverResolverMode:
		target = buildFailoverTarget(svcName)
	default:
		return nil, errors.New("invalid discover mode")
	}
	return NewClient(target, buildOptions(token, grpc.WithTransportCredentials(cfg.cred))...)
}

func NewTokenClientWithHost(host string, token string, opts ...TrpcOption) (*CanCloseClientConn, error) {
	cfg := &config{
		cred: insecure.NewCredentials(),
	}
	for _, o := range opts {
		o.apply(cfg)
	}
	return NewClient(host, buildOptions(token, grpc.WithTransportCredentials(cfg.cred))...)
}

func FixedToken(token string) func() string {
	return func() string {
		return token
	}
}

func buildOptions(token string, opts ...grpc.DialOption) []grpc.DialOption {
	return append(opts, grpc.WithPerRPCCredentials(auth.SdkAuthToken{Token: token}),
		grpc.WithChainUnaryInterceptor(interceptor.ClientInterceptors()...),
		grpc.WithChainStreamInterceptor(interceptor.StreamClientInterceptors()...),
	)
}

// buildSvcOptions 建连不绑固定 token；每次 RPC 从 ctx 读 common.Token（缺省 InternalToken）。
func buildSvcOptions(opts ...grpc.DialOption) []grpc.DialOption {
	return append(opts, grpc.WithPerRPCCredentials(auth.CtxAuthToken{Fallback: common.InternalToken}),
		grpc.WithChainUnaryInterceptor(interceptor.ClientInterceptors()...),
		grpc.WithChainStreamInterceptor(interceptor.StreamClientInterceptors()...),
	)
}

// NewSvcClient 按服务发现建连，身份走 ctx（CtxAuthToken），供 SvcClientPool 使用。
func NewSvcClient(svcName SvcName, opts ...TrpcOption) (*CanCloseClientConn, error) {
	cfg := &config{
		mode: LbDiscoverResolverMode,
		cred: insecure.NewCredentials(),
	}
	for _, o := range opts {
		o.apply(cfg)
	}
	target := ""
	switch cfg.mode {
	case LbDiscoverResolverMode:
		target = buildDiscoverTarget(svcName)
	case FailoverDiscoverResolverMode:
		target = buildFailoverTarget(svcName)
	default:
		return nil, errors.New("invalid discover mode")
	}
	return NewClient(target, buildSvcOptions(grpc.WithTransportCredentials(cfg.cred))...)
}

// NewSvcClientWithHost 直连 host，身份走 ctx（CtxAuthToken）。
func NewSvcClientWithHost(host string, opts ...TrpcOption) (*CanCloseClientConn, error) {
	cfg := &config{
		cred: insecure.NewCredentials(),
	}
	for _, o := range opts {
		o.apply(cfg)
	}
	return NewClient(host, buildSvcOptions(grpc.WithTransportCredentials(cfg.cred))...)
}

func NewInternalClient(svcName SvcName) (*CanCloseClientConn, error) {
	return NewTokenClient(svcName, common.InternalToken, WithDiscoverResolverMode(globalResolverMode))
}
func NewInternalClientWithHost(host string) (*CanCloseClientConn, error) {
	return NewTokenClientWithHost(host, common.InternalToken)
}

// SvcDiscoverDepend 服务发现依赖
type SvcDiscoverDepend internalResolver.SvcDiscoverDepend

// InitTrpcDiscover 初始化服务发现
func InitTrpcDiscover(serviceName string, serviceEndpoint string, svcmap SvcDiscoverDepend) {
	resolver.Register(internalResolver.NewDiscoverResolverBuilder(serviceName, serviceEndpoint, svcmap))
}

// InitTrpcDiscover 初始化服务发现
func InitTrpcDiscoverOnlyRemote(svcmap SvcDiscoverDepend) {
	resolver.Register(internalResolver.NewDiscoverResolverBuilder("", "", svcmap))
}

// InitTrpcFailover 初始化failover resolver
func InitTrpcFailover(serviceName string, serviceEndpoint string, svcmap SvcDiscoverDepend) {
	resolver.Register(internalResolver.NewFailoverResolverBuilder(serviceName, serviceEndpoint, svcmap))
}

func buildDiscoverTarget(svcName SvcName) string {
	return internalResolver.BuildDiscoverTarget(string(svcName))
}

func buildFailoverTarget(svcName SvcName) string {
	return internalResolver.BuildFailOverTarget(string(svcName))
}

var _ SvcDiscoverDepend = (*SimpleSvcmap)(nil)

// SimpleSvcmap 服务发现依赖
type SimpleSvcmap struct {
	svcmap map[string][]string

	mu sync.RWMutex
}

func (s *SimpleSvcmap) GetTelemetrySvcmap() map[string][]string {
	return nil
}

// NewSimpleSvcmap 创建服务发现
func NewSimpleSvcmap(svcmap map[string][]string) *SimpleSvcmap {
	return &SimpleSvcmap{svcmap: svcmap}
}

// GetSvcmap 获取服务发现依赖
func (s *SimpleSvcmap) GetSvcmap() map[string][]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.svcmap
}

func SyncDynamicSvcmap(ctx context.Context) {
	conn, err := GetSvcConn(GetInternalCtx(ctx), CenterServer)
	if err != nil {
		logger.Errorf(ctx, "get svc conn error: %v", err)
		return
	}
	client := system.NewDiscoverServiceClient(conn)
	svcList, err := client.List(context.Background(), &emptypb.Empty{})
	if err != nil {
		logger.Errorf(ctx, "get svc list error: %v", err)
		return
	}
	svcmap := make(map[string]map[string]bool)
	for _, svc := range svcList.Services {
		if _, ok := svcmap[svc.ServiceName]; !ok {
			svcmap[svc.ServiceName] = make(map[string]bool)
		}
		if svc.ServiceVipEndpoint != "" {
			svcmap[svc.ServiceName][svc.ServiceVipEndpoint] = true
		} else {
			svcmap[svc.ServiceName][svc.ServiceEndpoint] = true
		}
	}
	telemetrySvcmap := make(map[string][]string)
	for _, svc := range svcList.Services {
		if svc.ServiceVipEndpoint != "" {
			telemetrySvcmap[svc.ServiceName] = append(telemetrySvcmap[svc.ServiceName], svc.TelemetryEndpoint)
		}
	}
	resultSvcmap := make(map[string][]string)
	for svcName, svcMap := range svcmap {
		for svcEndpoint := range svcMap {
			resultSvcmap[svcName] = append(resultSvcmap[svcName], svcEndpoint)
		}
	}

	discoverSvcmap.SetClusterSvcmap(resultSvcmap)
	discoverSvcmap.SetTelemetrySvcmap(telemetrySvcmap)
}
func WatchDynamicSvcmap(ctx context.Context) {
	safe.GoKeep(ctx, time.Second*10, func() {
		conn, err := GetSvcConn(ctx, CenterServer)
		if err != nil {
			logger.Errorf(ctx, "get svc conn error: %v", err)
			return
		}
		client := system.NewDiscoverServiceClient(conn)
		ch, err := client.Watch(ctx, &emptypb.Empty{})
		if err != nil {
			logger.Errorf(ctx, "watch dynamic svcmap get client error: %v", err)
			return
		}
		defer ch.CloseSend()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			action, err := ch.Recv()
			if err != nil {
				return
			}
			logger.Debug(ctx, "watch dynamic svcmap recv", zap.Any("action", action))
			ip := []string{action.Service.ServiceEndpoint}
			if action.Service.ServiceVipEndpoint != "" {
				ip = append(ip, action.Service.ServiceVipEndpoint)
			}

			mp := discoverSvcmap.GetClusterSvcmap()
			switch action.Action {
			case system.ServiceAction_SERVICE_ACTION_ADD:
				mp[action.Service.ServiceName] = ip
			case system.ServiceAction_SERVICE_ACTION_UPDATE:
			case system.ServiceAction_SERVICE_ACTION_DELETE:
				if endpoints, ok := mp[action.Service.ServiceName]; ok {
					for i, endpoint := range endpoints {
						if endpoint == action.Service.ServiceEndpoint {
							endpoints = append(endpoints[:i], endpoints[i+1:]...)
							mp[action.Service.ServiceName] = endpoints
							break
						}
					}
				}
			}

			discoverSvcmap.SetClusterSvcmap(mp)
		}
	})
}

var _ SvcDiscoverDepend = (*SimpleSvcmap)(nil)

// SimpleSvcmap 服务发现依赖
type DynamicSvcmap struct {
	getSvcmap func() map[string][]string

	mu sync.RWMutex
}

// NewDynamicSvcmap 创建服务发现
func NewDynamicSvcmap(getSvcmap func() map[string][]string) *DynamicSvcmap {
	return &DynamicSvcmap{getSvcmap: getSvcmap}
}

// GetSvcmap 获取服务发现依赖
func (s *DynamicSvcmap) GetSvcmap() map[string][]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.getSvcmap()
}

func (s *DynamicSvcmap) GetTelemetrySvcmap() map[string][]string {
	return nil
}

// SvcName 服务名称
type SvcName string

var CenterServer SvcName = ""

func SetCenterServer(server SvcName) {
	CenterServer = server
}

var (
	OplogRecordServer SvcName = ""
	AlertLogServer    SvcName = ""
)

func SetOplogRecordServer(server SvcName) {
	OplogRecordServer = server
}

func SetAlertLogServer(server SvcName) {
	AlertLogServer = server
}

var _ grpc.ClientConnInterface = (*ClientConn)(nil)

type ClientConn struct {
	ccc      *CanCloseClientConn
	lastUsed atomic.Int64
}

func (c *ClientConn) GetState() connectivity.State {
	return c.ccc.ClientConn.GetState()
}

func (c *ClientConn) Invoke(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error {
	return c.ccc.ClientConn.Invoke(ctx, method, args, reply, opts...)
}

func (c *ClientConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return c.ccc.ClientConn.NewStream(ctx, desc, method, opts...)
}

func (c *ClientConn) close() {
	c.ccc.Close()
}

type CanCloseClientConn struct {
	*grpc.ClientConn
}

func (c *CanCloseClientConn) Close() error {
	return c.ClientConn.Close()
}

type TokenKey string
type ITokenService interface {
	GetToken() string
}

type emptyTokenService struct{}

func (*emptyTokenService) GetToken() string {
	return ""
}

func SetTokenService(its ITokenService) {
	ts = its
}

var ts ITokenService = &emptyTokenService{}

func getTokenKey(ctx context.Context) string {
	token, ok := ctx.Value(common.Token).(string)
	if ok {
		return token
	}
	return ts.GetToken()
}
func getClientConn(key string) (*ClientConn, bool) {
	return globalClientPool.getClientConn(key)
}
func setClientConn(key string, conn *CanCloseClientConn) *ClientConn {
	return globalClientPool.setClientConn(key, conn)
}

func GetSvcConn(ctx context.Context, svcName SvcName) (*ClientConn, error) {
	return globalClientPool.GetSvcConn(ctx, svcName)
}

func GetSvcConnWithHost(ctx context.Context, host string) (*ClientConn, error) {
	return globalClientPool.GetSvcConnWithHost(ctx, host)
}

func CleanSvcConn() {
	globalClientPool.CleanSvcConn()
}

// GetSvcSharedConn 从按服务分池的全局池取连接（一个 svc 一条连接）。
// 调用方须在 ctx 中放入 common.Token（GetInternalCtx / 用户 token）；缺省走 InternalToken。
func GetSvcSharedConn(ctx context.Context, svcName SvcName) (*ClientConn, error) {
	return globalSvcClientPool.GetSvcConn(ctx, svcName)
}

// GetSvcSharedConnWithHost 按 host 从共享池取连接。
func GetSvcSharedConnWithHost(ctx context.Context, host string) (*ClientConn, error) {
	return globalSvcClientPool.GetSvcConnWithHost(ctx, host)
}

// InvalidateSvcSharedConn 丢掉共享池中该 svc 的连接，下次 Get 重建。
func InvalidateSvcSharedConn(svcName SvcName) {
	globalSvcClientPool.Invalidate(svcName)
}

// CleanSvcSharedConn 清理共享服务池中的闲置连接。
func CleanSvcSharedConn() {
	globalSvcClientPool.CleanSvcConn()
}

func GetSystemCtx(ctx context.Context) context.Context {
	return newMapValueCtx(ctx, map[any]any{
		common.Token:    common.InternalToken,
		common.CallRole: common.SystemCallRole,
	})
}

func GetInternalCtx(ctx context.Context) context.Context {
	return newMapValueCtx(ctx, map[any]any{
		common.Token:    common.InternalToken,
		common.CallRole: common.UserCallRole,
	})
}

var userToken = ""

func SetUserToken(token string) {
	userToken = token
}

func GetUserCtx(ctx context.Context) context.Context {
	return newMapValueCtx(ctx, map[any]any{
		common.Token:    userToken,
		common.CallRole: common.UserCallRole,
	})
}

func GetRequestIdCtx(ctx context.Context) context.Context {
	return withMapValueCtx(ctx, common.RequestIdKey, idutil.GenUUID())
}

var _ context.Context = (*mapValueCtx)(nil)
var _ WithValueCtx = (*mapValueCtx)(nil)

type WithValueCtx interface {
	WithValue(key any, value any) context.Context
}

type mapValueCtx struct {
	ctx context.Context

	mu     sync.RWMutex
	values map[any]any
}

func withMapValueCtx(ctx context.Context, k, v any) context.Context {
	if c, ok := ctx.(WithValueCtx); ok {
		return c.WithValue(k, v)
	}
	return &mapValueCtx{ctx: ctx, values: map[any]any{k: v}}
}
func newMapValueCtx(ctx context.Context, values map[any]any) context.Context {
	return &mapValueCtx{ctx: ctx, values: values}
}

func (m *mapValueCtx) WithValue(key any, value any) context.Context {
	m.mu.Lock()
	defer m.mu.Unlock()
	newValues := make(map[any]any)
	maps.Copy(newValues, m.values)
	newValues[key] = value
	return &mapValueCtx{ctx: m.ctx, values: newValues}
}

func (m *mapValueCtx) Deadline() (deadline time.Time, ok bool) {
	return m.ctx.Deadline()
}

func (m *mapValueCtx) Done() <-chan struct{} {
	return m.ctx.Done()
}

func (m *mapValueCtx) Err() error {
	return m.ctx.Err()
}

func (m *mapValueCtx) Value(key any) any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	value, ok := m.values[key]
	if !ok {
		return m.ctx.Value(key)
	}
	return value
}

var (
	discoverSvcmap *ClusterSvcmap
	once           sync.Once
)

var _ SvcDiscoverDepend = (*ClusterSvcmap)(nil)

type ClusterSvcmap struct {
	localSvcmap func() map[string][]string

	clusterSvcmap map[string][]string

	telemetrySvcmap map[string][]string

	combineSvcmap map[string][]string

	mu   sync.RWMutex
	once sync.Once
}

func NewClusterSvcmap(localSvcmap func() map[string][]string) *ClusterSvcmap {
	once.Do(func() {
		discoverSvcmap = &ClusterSvcmap{
			localSvcmap:   localSvcmap,
			clusterSvcmap: make(map[string][]string),
			combineSvcmap: make(map[string][]string),
		}
	})
	return discoverSvcmap
}

func (c *ClusterSvcmap) GetSvcmap() map[string][]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	c.once.Do(func() {
		maps.Copy(c.combineSvcmap, c.localSvcmap())
	})
	return c.combineSvcmap
}

func (c *ClusterSvcmap) GetLocalSvcmap() map[string][]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.localSvcmap()
}

func (c *ClusterSvcmap) GetClusterSvcmap() map[string][]string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.clusterSvcmap
}
func (s *ClusterSvcmap) SetClusterSvcmap(clusterSvcmap map[string][]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.once.Do(func() {
		maps.Copy(s.combineSvcmap, s.localSvcmap())
	})
	s.clusterSvcmap = clusterSvcmap
	s.combineSvcmap = make(map[string][]string, len(s.clusterSvcmap)+len(s.localSvcmap()))
	maps.Copy(s.combineSvcmap, s.clusterSvcmap)
	maps.Copy(s.combineSvcmap, s.localSvcmap()) // 覆盖
}
func (s *ClusterSvcmap) SetSvcmap(svcmap map[string][]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.once.Do(func() {
		maps.Copy(s.combineSvcmap, s.localSvcmap())
	})
	s.clusterSvcmap = svcmap
	s.combineSvcmap = svcmap
}
func (s *ClusterSvcmap) SetTelemetrySvcmap(telemetrySvcmap map[string][]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.telemetrySvcmap = telemetrySvcmap
}

func (s *ClusterSvcmap) GetTelemetrySvcmap() map[string][]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.telemetrySvcmap
}

func GetSvcmap() SvcDiscoverDepend {
	if discoverSvcmap == nil {
		discoverSvcmap = NewClusterSvcmap(func() map[string][]string {
			return make(map[string][]string)
		})
	}
	return discoverSvcmap
}

// failoverUnaryInterceptor 创建failover一元拦截器
func failoverUnaryInterceptor(svcName string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		err := invoker(ctx, method, req, reply, cc, opts...)

		// 检查是否是连接相关的错误
		if isConnectionError(err) {
			// 触发resolver的ResolveNow来尝试恢复或切换地址
			cc.GetState() // 这会触发连接状态检查
			// 可以在这里添加更多的failover逻辑
		}

		return err
	}
}

// failoverStreamInterceptor 创建failover流拦截器
func failoverStreamInterceptor(svcName string) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		stream, err := streamer(ctx, desc, cc, method, opts...)

		// 检查是否是连接相关的错误
		if isConnectionError(err) {
			// 触发resolver的ResolveNow来尝试恢复或切换地址
			cc.GetState() // 这会触发连接状态检查
		}

		return stream, err
	}
}

// isConnectionError 判断是否是连接相关的错误
func isConnectionError(err error) bool {
	if err == nil {
		return false
	}

	// 检查常见的连接错误
	errStr := err.Error()
	connectionErrors := []string{
		"connection refused",
		"connection reset",
		"connection timeout",
		"no such host",
		"network is unreachable",
		"transport is closing",
		"connection closed",
		"unavailable",
	}

	for _, connErr := range connectionErrors {
		if strings.Contains(strings.ToLower(errStr), connErr) {
			return true
		}
	}

	return false
}

type DiscoverResolverMode string // 服务发现模式

const LbDiscoverResolverMode = "lb"             // 负载均衡模式
const FailoverDiscoverResolverMode = "failover" // 故障转移模式

func SetDiscoverResolverMode(mode DiscoverResolverMode) {
	globalResolverMode = mode
}
