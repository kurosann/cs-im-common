package internal

import (
	"context"
	"fmt"

	"vane-common/module"
	"vane-common/server/pkg/role"
	"vane-common/telemetry"
)

type MonitorRuntime struct {
	serviceName role.ServiceRole
	disabled    bool
}

func NewMonitorRuntime(serviceName role.ServiceRole, disabled bool) *MonitorRuntime {
	return &MonitorRuntime{
		serviceName: serviceName,
		disabled:    disabled,
	}
}

func (r *MonitorRuntime) Init(ctx context.Context, modules []module.ModuleInfo) error {
	if r.disabled {
		return nil
	}

	for _, info := range modules {
		if monitorModule, ok := info.Module.(module.IMonitorModule); ok {
			for _, monitor := range monitorModule.Monitor() {
				telemetry.CC.RegisterHook(monitor.Name, string(r.serviceName), monitor.Monitor.Collect)
				for _, metric := range monitor.Monitor.Metrics() {
					if _, err := telemetry.CC.Register(string(r.serviceName), metric.Collector); err != nil {
						return fmt.Errorf("module [%s] monitor [%s] register error: %w", info.Name, monitor.Name, err)
					}
				}
			}
		}
	}

	return nil
}
