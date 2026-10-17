package internal

import (
	"context"
	"os"
	"sync"
	"time"

	"vane-common/module"
	"vane-common/pkg/logger"
	"vane-common/pkg/safe"
	"vane-common/server/internal/itnlmodule"
	"vane-common/server/internal/itnlmodule/depend"
	"vane-common/server/internal/standalonemodule"
	pkgJob "vane-common/server/pkg/job"
	"vane-common/server/pkg/role"
	"vane-common/telemetry"

	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// 模块管理器
type ModuleManager struct {
	Modules []module.ModuleInfo

	distributeLock depend.IDistributeLock
	once           sync.Once
	config         ModuleManagerConfig
	grpcRuntime    *GrpcRuntime
	cronRuntime    *CronRuntime
	monitorRuntime *MonitorRuntime
	taskRuntime    *TaskRuntime
}

type ModuleManagerConfig struct {
	ApiPort        int
	DisableGRPC    bool
	TelemetryPort  int
	ServiceName    role.ServiceRole
	DisableMonitor bool
	Standalone     bool
	GrpcOpts       []grpc.ServerOption
	Hosts          []string
	DistributeLock depend.IDistributeLock
}

func NewModuleManager(config ModuleManagerConfig) *ModuleManager {
	return &ModuleManager{
		Modules: make([]module.ModuleInfo, 0),
		config:  config,
	}
}

func (m *ModuleManager) RegisterModule(modules ...module.ModuleInfo) {
	m.Modules = append(m.Modules, modules...)
}

// RegisterGrpcServerOptions 注册 gRPC 服务端选项，如 mTLS credentials
// 需在 Init 之前调用
func (m *ModuleManager) RegisterGrpcServerOptions(opts ...grpc.ServerOption) {
	m.config.GrpcOpts = append(m.config.GrpcOpts, opts...)
}

func (m *ModuleManager) Init(ctx context.Context) error {
	var err error
	m.once.Do(func() {
		m.distributeLock = resolveDistributeLock(m.config)
		pkgJob.SetDistributeLock(m.distributeLock)
		if !m.config.DisableMonitor {
			telemetry.InitCollectCenter(ctx, telemetry.DefaultMaxGoroutine)
		}
		modeModule := resolveInternalModule(m.config)
		m.Modules = append([]module.ModuleInfo{modeModule}, m.Modules...)
		for _, module := range m.Modules {
			logger.Infof(ctx, "Init module %s", module.Name)
			if err = module.Module.Init(ctx); err != nil {
				return
			}
		}
		if !m.config.DisableGRPC {
			m.grpcRuntime = NewGrpcRuntime(m.config.ServiceName, m.config.ApiPort, m.config.GrpcOpts...)
			if err = m.grpcRuntime.Init(ctx, m.Modules); err != nil {
				return
			}
		}
		m.cronRuntime = NewCronRuntime()
		if err = m.cronRuntime.Init(ctx, m.Modules); err != nil {
			return
		}
		m.monitorRuntime = NewMonitorRuntime(m.config.ServiceName, m.config.DisableMonitor)
		if err = m.monitorRuntime.Init(ctx, m.Modules); err != nil {
			return
		}
		m.taskRuntime = NewTaskRuntime(m.distributeLock, m.config.Standalone)
		if err = m.taskRuntime.Init(ctx, m.Modules); err != nil {
			return
		}
	})
	return err
}
func (m *ModuleManager) Run(ctx context.Context) {
	wg := safe.NewWaitGroupContext(ctx)
	if m.grpcRuntime != nil && m.grpcRuntime.Listener() != nil {
		wg.Go(func() {
			logger.Infof(ctx, "Grpc server listen %s", m.grpcRuntime.Listener().Addr().String())
			if err := m.grpcRuntime.Server().Serve(m.grpcRuntime.Listener()); err != nil {
				logger.Error(ctx, "grpc server serve error", zap.Error(err))
				os.Exit(1)
			}
		})
	}
	wg.Go(func() {
		if m.cronRuntime != nil {
			m.cronRuntime.Run(ctx, m.Modules)
		}
	})
	wg.Wait()
	time.Sleep(time.Millisecond)
}

func (m *ModuleManager) Stop() {
	ttl := 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), ttl)
	defer cancel()
	wg := safe.NewWaitGroupContext(ctx)
	if m.cronRuntime != nil {
		cronCtx := m.cronRuntime.Stop()
		wg.GoCtx(buildWaitTask(cronCtx, "cron"))
	}
	if m.grpcRuntime != nil && m.grpcRuntime.Server() != nil {
		grpcCtx, cancel := context.WithTimeout(context.Background(), ttl-time.Second)
		defer cancel()
		safe.Go(func() {
			m.grpcRuntime.Server().GracefulStop()
			cancel()
		})
		wg.GoCtx(buildWaitTask(grpcCtx, "grpc server"))
	}
	if len(m.Modules) > 0 {
		moduleCtx, cancel := context.WithTimeout(context.Background(), ttl-time.Second)
		defer cancel()
		safe.Go(func() {
			wg := safe.NewWaitGroupContext(moduleCtx)
			for _, info := range m.Modules {
				if stopModule, ok := info.Module.(module.IStopModule); ok {
					wg.GoCtxE(stopModule.Stop)
				}
			}
			if err := wg.WaitAndRecover(); err != nil {
				logger.Error(moduleCtx, "stop module error", zap.Error(err))
			}
			cancel()
		})
		wg.GoCtx(buildWaitTask(moduleCtx, "module"))
	}
	if err := wg.WaitAndRecover(); err != nil {
		logger.Error(ctx, "stop module error", zap.Error(err))
	}
}

func buildWaitTask(waitCtx context.Context, name string) func(ctx context.Context) {
	return func(ctx context.Context) {
		select {
		case <-waitCtx.Done():
		case <-ctx.Done():
		}
		logger.Debugf(ctx, "%s stop", name)
	}
}
func resolveInternalModule(config ModuleManagerConfig) module.ModuleInfo {
	if config.Standalone {
		return module.NewModule("internal",
			standalonemodule.NewModule(
				string(config.ServiceName),
				config.DisableMonitor,
			),
		)
	}

	return module.NewModule("internal",
		itnlmodule.NewModule(
			string(config.ServiceName),
			config.ApiPort,
			config.TelemetryPort,
			config.DisableMonitor,
			config.Hosts,
		),
	)
}
func resolveDistributeLock(config ModuleManagerConfig) depend.IDistributeLock {
	if config.DistributeLock != nil {
		return config.DistributeLock
	}
	if config.Standalone {
		return depend.NewMemoryDistributeLock()
	}
	return depend.NewDistributeLock(string(config.ServiceName))
}
