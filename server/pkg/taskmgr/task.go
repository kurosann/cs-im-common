package taskmgr

import (
	"context"
	"errors"
	"sync"
	"time"

	"vane-common/pkg/taskmgr"
	"vane-common/server/internal/itnlmodule/depend"
)

var (
	globalTaskManager *taskmgr.TaskManager
	localTaskStore    *depend.LocalTaskStore // standalone 模式非空
	once              sync.Once
)

// SetLocalTaskStore 注入 standalone 本地任务存储（TaskRuntime 初始化时调用）。
func SetLocalTaskStore(s *depend.LocalTaskStore) {
	localTaskStore = s
}

// AsyncRunTask 提交异步任务。standalone 模式经 LocalTaskStore 进程内立即派发；
// 集群模式的任务创建需经 CenterServer（TaskManagerService 暂无创建入口），未实现。
func AsyncRunTask(_ context.Context, taskCode string, payload []byte) (taskID string, err error) {
	if localTaskStore == nil {
		return "", errors.New("task submit unavailable: local task store not initialized (standalone only)")
	}
	return localTaskStore.SubmitTask(taskCode, payload)
}

type ProviderConfig struct {
	StepInfoProvider taskmgr.ITaskStepInfoProvider
	Reporter         taskmgr.ITaskReporter
	DataProvider     taskmgr.ITaskDataProvider
	CanRunTask       taskmgr.ICanRunTask
	LockProvider     depend.IDistributeLock
}

// InitTaskManager 初始化全局 TaskManager
func InitTaskManager(config ProviderConfig) error {
	var err error
	once.Do(func() {
		if config.StepInfoProvider == nil {
			err = errors.New("step info provider is nil")
		}
		if config.Reporter == nil {
			err = errors.New("reporter is nil")
		}
		if config.DataProvider == nil {
			err = errors.New("data provider is nil")
		}
		if config.CanRunTask == nil {
			err = errors.New("can run task is nil")
		}
		// 加锁时长
		ttl := 30 * time.Second
		globalTaskManager = taskmgr.NewTaskManager(
			config.StepInfoProvider,
			config.Reporter,
			config.DataProvider,
			NewTaskLockProvider(config.LockProvider, ttl),
			config.CanRunTask,
			taskmgr.WithTaskLockKeepInterval(ttl-10*time.Second), // 续锁间隔
		)
	})
	return err
}

func GetTaskManager() *taskmgr.TaskManager {
	return globalTaskManager
}

func RegisterTaskWorker(taskCode string, worker taskmgr.TaskWorker) error {
	if globalTaskManager == nil {
		return ErrTaskManagerNotInitialized
	}
	return globalTaskManager.RegisterTaskWorker(taskCode, worker)
}

func StartTaskManager(ctx context.Context) error {
	if globalTaskManager == nil {
		return ErrTaskManagerNotInitialized
	}
	return globalTaskManager.Start(ctx)
}

func StopTaskManager() {
	if globalTaskManager != nil {
		globalTaskManager.Stop()
	}
}

func WaitTaskManager() {
	if globalTaskManager != nil {
		globalTaskManager.Wait()
	}
}
