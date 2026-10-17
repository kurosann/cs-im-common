package standalonemodule

import (
	"context"

	"vane-common/module"
	"vane-common/module/monitor"
	"vane-common/server/internal/itnlmodule/metric"
)

var (
	_ module.IModule        = (*Module)(nil)
	_ module.IMonitorModule = (*Module)(nil)
)

type Module struct {
	name           string
	disableMonitor bool
}

func NewModule(name string, disableMonitor bool) *Module {
	return &Module{
		name:           name,
		disableMonitor: disableMonitor,
	}
}

func (m *Module) Init(ctx context.Context) error {
	return nil
}

func (m *Module) Api() []module.ApiRegInfo {
	return nil
}

func (m *Module) Job() []module.JobRegInfo {
	return nil
}

func (m *Module) Task() []module.TaskRegInfo {
	return nil
}

func (m *Module) Monitor() []monitor.MonitorRegInfo {
	if m.disableMonitor {
		return nil
	}

	return []monitor.MonitorRegInfo{
		monitor.NewMonitorRegInfo("memory_status", metric.NewMemoryStatus(), monitor.WithSpec("@every 3s")),
		monitor.NewMonitorRegInfo("cpu_status", metric.NewCPUStatus(), monitor.WithSpec("@every 3s")),
		monitor.NewMonitorRegInfo("network_status", metric.NewNetworkStatus(), monitor.WithSpec("@every 3s")),
	}
}
