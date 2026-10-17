package itnlmodule

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"vane-common/alertlog"
	"vane-common/discover"
	"vane-common/module"
	"vane-common/module/monitor"
	"vane-common/module/statemgr"
	"vane-common/oplog"
	"vane-common/publish"
	"vane-common/server/internal/itnlmodule/api"
	"vane-common/server/internal/itnlmodule/depend"
	"vane-common/server/internal/itnlmodule/job"
	"vane-common/server/internal/itnlmodule/metric"
	taskApi "vane-common/task"
	"vane-common/telemetry"
	"vane-common/trpc"
	"vane-common/v1/pilot/system"
)

var (
	_ module.IModule         = (*Module)(nil)
	_ module.IMonitorModule  = (*Module)(nil)
	_ module.IModuleStateMgr = (*Module)(nil)
	_ module.IStopModule     = (*Module)(nil)
)

type Module struct {
	name           string
	configHost     []string
	port           int
	telemetryPort  int
	disableMonitor bool
}

func NewModule(name string, port int, telemetryPort int, disableMonitor bool, configHost []string) *Module {
	return &Module{
		name:           name,
		port:           port,
		telemetryPort:  telemetryPort,
		disableMonitor: disableMonitor,
		configHost:     configHost,
	}
}

func (m *Module) Init(ctx context.Context) error {
	trpc.WatchDynamicSvcmap(ctx)
	oplog.Init(ctx, depend.NewOplogRecord())
	publish.SetRemoteTransport(depend.NewPublishRemoteTransportService())
	alertlog.Init(ctx, depend.NewDefaultAlertLogRecorder())
	return nil
}

func (m *Module) Stop(ctx context.Context) error {
	discover.RemoveSelf(ctx)
	return nil
}

func (m *Module) Api() []module.ApiRegInfo {
	return []module.ApiRegInfo{
		module.NewApiRegInfo(system.TaskService_ServiceDesc, taskApi.NewTaskApi()),
		module.NewApiRegInfo(system.PublisherService_ServiceDesc, api.NewPublishApi()),
	}
}

func (m *Module) Job() []module.JobRegInfo {
	workers := make([]telemetry.CollectDataWorker, 0)
	if !m.disableMonitor && telemetry.CC != nil {
		workers = telemetry.CC.CollectDataWorkers()
	}

	// ttl 范围为3-5秒 错开服务超时时间
	ttl := time.Second*10 + time.Second*time.Duration(rand.Float64()*2)
	defJobs := []module.JobRegInfo{
		module.NewJobRegInfo(job.SyncSvcmapJobName, "@every 30s", job.NewSyncSvcmapJob(), module.WithJobLaunchRun()),
		module.NewJobRegInfo(job.PublishSubcriberJobName, "@every 30s", job.NewPublishSubcriberJob(), module.WithJobLaunchRun()),
		module.NewJobRegInfo(job.CleanSyncConnJobName, "@every 30s", job.NewCleanSyncConnJob(), module.WithJobLaunchRun()),
		module.NewJobRegInfo(job.ElectionMasterJobName, "@every "+(3*time.Second).String(), job.NewElectionMasterJob(m.name, ttl), module.WithJobLaunchRun()),
		module.NewJobRegInfo(job.CleanSyncConnJobName, "@every 30s", job.NewCleanSyncConnJob(), module.WithJobLaunchRun()),
	}
	regJobs := append(make([]module.JobRegInfo, 0, len(defJobs)+len(workers)), defJobs...)

	metricJobs := make([]module.JobRegInfo, 0, len(workers))
	if !m.disableMonitor {
		for _, worker := range workers {
			metricJobs = append(metricJobs, module.NewJobRegInfo(fmt.Sprintf("monitor:%s", worker.MonitorName), worker.Spec, job.NewMetricHealthCheckJob(worker.Worker), module.WithJobLaunchRun()))
		}
	}
	return append(regJobs, metricJobs...)
}

func (m *Module) Task() []module.TaskRegInfo {
	return nil
}

func (m *Module) Monitor() []monitor.MonitorRegInfo {
	metrics := []monitor.MonitorRegInfo{
		monitor.NewMonitorRegInfo("memory_status", metric.NewMemoryStatus(), monitor.WithSpec("@every 3s")),
		monitor.NewMonitorRegInfo("cpu_status", metric.NewCPUStatus(), monitor.WithSpec("@every 3s")),
		monitor.NewMonitorRegInfo("network_status", metric.NewNetworkStatus(), monitor.WithSpec("@every 3s")),
		monitor.NewMonitorRegInfo("discover_status", metric.NewDiscoverStatus(), monitor.WithSpec("@every 10s")),
	}
	return metrics
}

func (m *Module) StateMgr() []statemgr.StateSyncRegInfo {
	return []statemgr.StateSyncRegInfo{
		statemgr.NewPushStateSyncRegInfo("service_sync", discover.NewDiscover(m.name, m.port, m.telemetryPort), statemgr.WithPushLaunchRun()),
	}
}
