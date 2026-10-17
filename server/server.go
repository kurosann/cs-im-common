package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"syscall"

	"vane-common/clusterprofile"
	"vane-common/module"
	"vane-common/pkg/logger"
	"vane-common/pkg/safe"
	"vane-common/server/internal"
	"vane-common/server/pkg/role"
	"vane-common/server/serverconfig"
	"vane-common/telemetry"
	"vane-common/trpc"
	"vane-common/trpc/common"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type IServer interface {
	Init(ctx context.Context) error
	Start(ctx context.Context) error
	Reload(ctx context.Context) error
	Stop()
}
type IDBServer interface {
	InitDB(ctx context.Context, regModels ...module.ModelRegInfo) error
}

var _ IServer = (*MainServer)(nil)

type MainServer struct {
	name          role.ServiceRole
	ctx           context.Context
	ctxCancel     context.CancelFunc
	moduleManager *internal.ModuleManager
	extandServers []IServer
	modules       []module.ModuleInfo

	config         serverconfig.Config
	configFn       func() serverconfig.Config
	grpcServerOpts []grpc.ServerOption
	roleConfig     clusterprofile.RoleConfig
	profileName    string

	telemetryAPI *http.ServeMux
	svcmap       trpc.SvcDiscoverDepend
}

func applyServerConfig(base serverconfig.Config, override serverconfig.Config) serverconfig.Config {
	if len(override.Hosts) > 0 {
		base.Hosts = override.Hosts
	}
	if override.Port != 0 {
		base.Port = override.Port
	}
	base.DisableGRPC = override.DisableGRPC
	if override.Reload != nil {
		base.Reload = override.Reload
	}
	base.EnableTelemetry = override.EnableTelemetry
	if override.TelemetryPort != 0 {
		base.TelemetryPort = override.TelemetryPort
	}
	base.Telemetry = override.Telemetry
	base.DisableMonitor = override.DisableMonitor
	base.Standalone = override.Standalone
	return base
}

func (s *MainServer) RegModule(modules []module.ModuleInfo) *MainServer {
	s.modules = append(s.modules, modules...)
	return s
}
func (s *MainServer) RegExtendServer(server ...IServer) *MainServer {
	s.extandServers = append(s.extandServers, server...)
	return s
}
func (s *MainServer) RegServiceMap(svcmap trpc.SvcDiscoverDepend) *MainServer {
	s.svcmap = svcmap
	return s
}

func (s *MainServer) RegGrpcServerOptions(opts ...grpc.ServerOption) *MainServer {
	s.grpcServerOpts = append(s.grpcServerOpts, opts...)
	return s
}
func (s *MainServer) checkConfig() {
	if s.configFn != nil {
		config := s.configFn()
		if config.Port != 0 {
			s.config.Port = config.Port
		}
		s.config.DisableGRPC = config.DisableGRPC
		if config.Reload != nil {
			s.config.Reload = config.Reload
		}
		s.config.EnableTelemetry = config.EnableTelemetry
		if config.TelemetryPort != 0 {
			s.config.TelemetryPort = config.TelemetryPort
		}
		s.config.DisableMonitor = config.DisableMonitor
		s.config.Telemetry = config.Telemetry
		s.config.Standalone = config.Standalone
	}
}

func (s *MainServer) Init(ctx context.Context) error {
	s.ctx, s.ctxCancel = context.WithCancel(ctx)
	s.ctx = trpc.GetSystemCtx(withCancelServerCtx(s.ctx, s.ctxCancel))
	s.ctx = context.WithValue(s.ctx, common.ServiceNameKey, string(s.name))
	logger.Infof(s.ctx, "Start init %s server all module", s.Name())
	s.checkConfig()
	logger.Infof(s.ctx, "Server profile %s", s.currentProfileName())
	s.assertConfig()
	if s.svcmap != nil && !s.config.Standalone {
		trpc.InitTrpcDiscover(string(s.name), fmt.Sprintf("localhost:%d", s.config.Port), s.svcmap)
	}
	role.SetServiceRole(s.name)
	role.SetNodeRole(s.roleConfig.NodeRole)
	role.SetReplicationRole(s.roleConfig.DefaultReplicationRole)
	if s.config.Standalone {
		// 独立部署无副本概念：置 None，全局 Job 经本地内存锁直接执行，
		// 不做 CenterServer 选主（standalone 下 CenterServer 不可达，Slave 会静默跳过所有全局 Job）
		role.SetReplicationRole(role.ReplicationRoleNone)
	}
	s.ctx = serverconfig.SetServerConfig(s.ctx, &s.config)
	s.moduleManager = internal.NewModuleManager(internal.ModuleManagerConfig{
		ApiPort:        s.config.Port,
		DisableGRPC:    s.config.DisableGRPC,
		TelemetryPort:  s.config.TelemetryPort,
		ServiceName:    s.name,
		DisableMonitor: !s.enableTelemetry(),
		Standalone:     s.config.Standalone,
		GrpcOpts:       s.grpcServerOpts,
		Hosts:          s.config.Hosts,
	})
	s.moduleManager.RegisterModule(s.modules...)

	dbServers, otherServers := splitExtendServers(s.extandServers)
	if len(dbServers) > 0 {
		if err := s.initExtendServers(dbServers); err != nil {
			return err
		}
	}

	if err := s.moduleManager.Init(s.ctx); err != nil {
		return err
	}
	logger.Infof(s.ctx, "Init %s all module done", s.Name())

	rest := s.extandServers
	if len(dbServers) > 0 {
		rest = otherServers
	}
	if err := s.initExtendServers(rest); err != nil {
		return err
	}
	logger.Infof(s.ctx, "Init %s extand server done", s.Name())
	if s.enableTelemetry() {
		s.telemetryAPI = http.NewServeMux()
		telemetry.SetupAPI(s.telemetryAPI, s.config.Telemetry)
		s.telemetryAPI.Handle("/", http.NotFoundHandler())
	}
	return nil
}

func splitExtendServers(servers []IServer) (dbServers []IServer, otherServers []IServer) {
	for _, server := range servers {
		if _, ok := server.(IDBServer); ok {
			dbServers = append(dbServers, server)
			continue
		}
		otherServers = append(otherServers, server)
	}
	return dbServers, otherServers
}

func (s *MainServer) collectDBModels() []module.ModelRegInfo {
	regModels := make([]module.ModelRegInfo, 0)
	for _, m := range s.modules {
		if dbModule, ok := m.Module.(module.IModuleDb); ok {
			regModels = append(regModels, dbModule.Model()...)
		}
	}
	return regModels
}

func (s *MainServer) initExtendServers(servers []IServer) error {
	if len(servers) == 0 {
		return nil
	}
	wg := safe.NewWaitGroup()
	regModels := s.collectDBModels()
	for _, server := range servers {
		wg.GoE(func() error {
			logger.Infof(s.ctx, "Init extand server %s", elem(reflect.ValueOf(server)))
			if err := server.Init(s.ctx); err != nil {
				return err
			}
			if dbServer, ok := server.(IDBServer); ok {
				if err := dbServer.InitDB(s.ctx, regModels...); err != nil {
					return err
				}
			}
			return nil
		})
	}
	return wg.WaitAndRecover()
}

const (
	CancelServerCtxKey common.CtxKey = "server.cancel"
)

func withCancelServerCtx(ctx context.Context, cancel context.CancelFunc) context.Context {
	return context.WithValue(ctx, CancelServerCtxKey, cancel)
}

func GetServerCancel(ctx context.Context) (context.CancelFunc, bool) {
	if ctx == nil {
		return nil, false
	}
	value := ctx.Value(CancelServerCtxKey)
	if value == nil {
		return nil, false
	}
	cancel, ok := value.(context.CancelFunc)
	if !ok {
		return nil, false
	}
	return cancel, true
}

func (s *MainServer) WithConfig(configFn func() serverconfig.Config) *MainServer {
	s.configFn = configFn
	return s
}

func (s *MainServer) WithRoleConfig(config clusterprofile.RoleConfig) *MainServer {
	s.roleConfig = config
	return s
}

func (s *MainServer) WithClusterProfile(profile clusterprofile.Profile) *MainServer {
	serviceProfile := profile.GetServiceProfile(s.name)
	s.roleConfig = serviceProfile.RoleConfig
	s.config = applyServerConfig(s.config, serviceProfile.ServerConfig)
	s.profileName = profile.Name
	return s
}

func (s *MainServer) enableTelemetry() bool {
	return s.config.EnableTelemetry && !s.config.DisableMonitor
}

func (s *MainServer) currentProfileName() string {
	if s.profileName == "" {
		return "default"
	}
	return s.profileName
}

func (s *MainServer) assertConfig() {
	if s.config.Port == 0 {
		logger.Panicf(s.ctx, "server port is required")
	}
	if s.config.Standalone {
		return
	}
	if s.svcmap == nil {
		logger.Panicf(s.ctx, "cluster mode requires service map")
	}
	if trpc.CenterServer == "" {
		logger.Panicf(s.ctx, "cluster mode requires trpc center server")
	}
	if s.enableTelemetry() && s.config.TelemetryPort == 0 {
		logger.Panicf(s.ctx, "telemetry is enabled but telemetry port is not set")
	}
}

func (s *MainServer) Name() string {
	return fmt.Sprintf("[%s]", s.name)
}

func elem(v reflect.Value) string {
	if v.Kind() == reflect.Ptr {
		return elem(v.Elem())
	}
	return v.Type().String()
}

func (s *MainServer) Start(_ context.Context) error {
	logger.Infof(s.ctx, "Start %s server", s.Name())
	if s.moduleManager != nil {
		safe.Go(func() { s.moduleManager.Run(s.ctx) })
	}
	for _, server := range s.extandServers {
		safe.Go(func() {
			if err := server.Start(s.ctx); err != nil {
				logger.Error(s.ctx, "extand server start error", zap.Error(err))
			}
		})
	}
	if s.enableTelemetry() {
		safe.Go(func() {
			logger.Infof(s.ctx, "Telemetry server %v", s.config.TelemetryPort)
			err := http.ListenAndServe(":"+strconv.Itoa(s.config.TelemetryPort), s.telemetryAPI)
			if err != nil {
				logger.Error(s.ctx, "Telemetry server start error", zap.Error(err))
			}
		})
	}
	logger.Infof(s.ctx, "Start %s server success", s.Name())
	return nil
}

func (s *MainServer) Reload(ctx context.Context) error {
	if s.config.Reload != nil {
		return s.config.Reload()
	}
	s.ctx = serverconfig.SetServerConfig(s.ctx, &s.config)
	logger.Infof(s.ctx, "Reload %s server", s.Name())
	for _, server := range s.extandServers {
		logger.Infof(s.ctx, "Reload extand server %s", elem(reflect.ValueOf(server)))
		server.Reload(s.ctx)
	}
	return nil
}

func (s *MainServer) Stop() {
	if s.ctxCancel != nil {
		s.ctxCancel()
	}
	wg := safe.NewWaitGroup()
	if s.moduleManager != nil {
		wg.Go(func() {
			logger.Infof(s.ctx, "Stop %s all module", s.Name())
			s.moduleManager.Stop()
		})
	}
	for _, server := range s.extandServers {
		wg.Go(func() {
			logger.Infof(s.ctx, "Stop extand server %s", elem(reflect.ValueOf(server)))
			server.Stop()
		})
	}
	wg.Wait()

	telemetry.ClosePush()
	logger.Infof(s.ctx, "server %s all done", s.Name())
}
func (s *MainServer) Wait(_ context.Context) {
	var sig os.Signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)
	defer func() { logger.Sync() }()

loop:
	for {
		select {
		case sig = <-sigCh:
		case <-s.ctx.Done():
			sig = syscall.SIGTERM
		}
		logger.Infof(s.ctx, "got signal %v", sig)
		switch sig {
		case syscall.SIGHUP:
			if err := safe.TryE(func() error {
				return s.Reload(s.ctx)
			}); err != nil {
				logger.Errorf(s.ctx, "reload error: %v", err)
			}
		case syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM:
			s.Stop()
			break loop
		}
	}
}

func NewServer(name role.ServiceRole, profiles ...clusterprofile.Profile) *MainServer {
	server := &MainServer{
		name:       name,
		roleConfig: clusterprofile.DefaultRoleConfig(),
	}
	for _, profile := range profiles {
		server.WithClusterProfile(profile)
	}
	return server
}
