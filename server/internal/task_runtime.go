package internal

import (
	"context"

	"vane-common/module"
	"vane-common/server/internal/itnlmodule/depend"
	serverTaskmgr "vane-common/server/pkg/taskmgr"
)

type TaskRuntime struct {
	distributeLock depend.IDistributeLock
	standalone     bool
}

func NewTaskRuntime(distributeLock depend.IDistributeLock, standalone bool) *TaskRuntime {
	return &TaskRuntime{distributeLock: distributeLock, standalone: standalone}
}

func (r *TaskRuntime) Init(ctx context.Context, modules []module.ModuleInfo) error {
	taskInfos := make([]module.TaskRegInfo, 0)
	for _, info := range modules {
		taskInfos = append(taskInfos, info.Module.Task()...)
	}

	if len(taskInfos) == 0 {
		return nil
	}

	var taskProvider serverTaskmgr.ProviderConfig
	if r.standalone {
		// standalone：CenterServer 不可达，用进程内 LocalTaskStore 承担
		// 任务存储/上报/派发/运行许可；锁仍走已解析的内存分布式锁。
		store := depend.NewLocalTaskStore()
		taskProvider = serverTaskmgr.ProviderConfig{
			StepInfoProvider: store,
			Reporter:         store,
			DataProvider:     store,
			CanRunTask:       store,
			LockProvider:     r.distributeLock,
		}
		serverTaskmgr.SetLocalTaskStore(store)
	} else {
		taskProvider = serverTaskmgr.ProviderConfig{
			StepInfoProvider: depend.NewTaskStepInfoProvider(),
			Reporter:         depend.NewTaskReporter(),
			DataProvider:     depend.NewTaskDataProvider(),
			CanRunTask:       depend.NewCanRunTaskChecker(),
			LockProvider:     r.distributeLock,
		}
	}

	if err := serverTaskmgr.InitTaskManager(taskProvider); err != nil {
		return err
	}
	for _, taskInfo := range taskInfos {
		if err := serverTaskmgr.RegisterTaskWorker(taskInfo.Code, taskInfo.Task); err != nil {
			return err
		}
	}
	return serverTaskmgr.StartTaskManager(ctx)
}
