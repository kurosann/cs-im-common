package taskmgr

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"vane-common/pkg/clock"
	"vane-common/pkg/logger"
	"vane-common/pkg/safe"

	"go.uber.org/zap"
)

type TaskManager struct {
	engine            *TaskEngine
	DataProvider      ITaskDataProvider
	LockProvider      ITaskLockProvider
	CanRunTask        ICanRunTask
	PollInterval      time.Duration
	LockKeepInterval  time.Duration
	DisableAutoRevert bool
	wg                *safe.WaitGroup
	ctx               context.Context
	cancel            context.CancelFunc
	// 任务上下文管理，用于主动停止任务
	taskContexts sync.Map // map[taskID]context.CancelFunc
	// 任务处理器映射，用于直接修改processor状态
	taskProcessors sync.Map // map[taskID]*TaskProcessor
}

type TaskManagerOption func(*TaskManager)

func WithTaskPollInterval(interval time.Duration) TaskManagerOption {
	return func(tm *TaskManager) {
		tm.PollInterval = interval
	}
}

func WithTaskLockKeepInterval(interval time.Duration) TaskManagerOption {
	return func(tm *TaskManager) {
		tm.LockKeepInterval = interval
	}
}
func WithNeedAutoRevert(needAutoRevert bool) TaskManagerOption {
	return func(tm *TaskManager) {
		tm.DisableAutoRevert = needAutoRevert
	}
}

func NewTaskManager(
	stepInfoProvider ITaskStepInfoProvider,
	reporter ITaskReporter,
	dataProvider ITaskDataProvider,
	lockProvider ITaskLockProvider,
	canRunTask ICanRunTask,
	opts ...TaskManagerOption,
) *TaskManager {
	engine := newTaskEngine(stepInfoProvider, reporter, lockProvider)

	tm := &TaskManager{
		engine:           engine,
		DataProvider:     dataProvider,
		LockProvider:     lockProvider,
		CanRunTask:       canRunTask,
		PollInterval:     5 * time.Second,
		LockKeepInterval: 20 * time.Second,
	}

	for _, opt := range opts {
		opt(tm)
	}

	return tm
}

func (tm *TaskManager) RegisterTaskWorker(taskCode string, worker TaskWorker) error {
	return tm.engine.RegisterTaskWorker(taskCode, worker)
}

func (tm *TaskManager) Start(ctx context.Context) error {
	tm.ctx, tm.cancel = context.WithCancel(ctx)
	tm.wg = safe.NewWaitGroupContext(tm.ctx)
	tm.wg.GoCtx(func(ctx context.Context) {
		tm.watchTasks(ctx)
	})

	if !tm.DisableAutoRevert {
		tm.wg.GoCtx(func(ctx context.Context) {
			tm.pollTasks(ctx)
		})
	}

	return nil
}
func (tm *TaskManager) IsRunning(taskID string) bool {
	if _, ok := tm.taskProcessors.Load(taskID); ok {
		return true
	}
	return false
}

func (tm *TaskManager) Stop() {
	if tm.cancel != nil {
		tm.cancel()
	}
}

func (tm *TaskManager) Wait() {
	if tm.wg != nil {
		tm.wg.Wait()
	}
}

// StopMode 停止模式
type StopMode string

const (
	StopModeCancel StopMode = "cancel" // 取消（回滚）
	StopModeStop   StopMode = "stop"   // 停止（不回滚）
	StopModePause  StopMode = "pause"  // 暂停
)

func stopModeToTaskStatus(mode StopMode) Status {
	switch mode {
	case StopModeStop:
		return StatusStopped
	case StopModePause:
		return StatusPaused
	case StopModeCancel:
		fallthrough
	default:
		return StatusCanceled
	}
}

// StopTaskWithMode 按指定模式停止任务
func (tm *TaskManager) StopTaskWithMode(taskID string, mode StopMode) error {
	switch mode {
	case StopModeCancel:
		return tm.cancelTask(taskID)
	case StopModeStop:
		return tm.stopTask(taskID)
	case StopModePause:
		return tm.pauseTask(taskID)
	default:
		return fmt.Errorf("unsupported stop mode: %s", mode)
	}
}

// cancelTask 取消任务（执行回滚）
func (tm *TaskManager) cancelTask(taskID string) error {
	var reportErr error
	tm.taskContexts.Store(taskID+"_stop_mode", StopModeCancel)

	if processorVal, ok := tm.taskProcessors.Load(taskID); ok {
		if processor, ok := processorVal.(*TaskProcessor); ok {
			reportErr = processor.Cancel()
			logger.Info(tm.ctx, "[TaskManager] cancelTask - direct processor update",
				zap.String("taskID", taskID))
		}
	}

	// 然后取消任务上下文
	if cancelFunc, ok := tm.taskContexts.Load(taskID); ok {
		if cancel, ok := cancelFunc.(context.CancelFunc); ok {
			cancel()
			logger.Info(tm.ctx, "[TaskManager] cancelTask - context cancelled",
				zap.String("taskID", taskID))
			return reportErr
		}
		return fmt.Errorf("task %s cancel function invalid", taskID)
	}

	// 如果任务不存在，预先存储停止模式，等待任务启动时使用
	logger.Info(tm.ctx, "[TaskManager] cancelTask pre-stored",
		zap.String("taskID", taskID))
	return reportErr
}

// stopTask 停止任务（不执行回滚）
func (tm *TaskManager) stopTask(taskID string) error {
	var reportErr error
	tm.taskContexts.Store(taskID+"_stop_mode", StopModeStop)

	if processorVal, ok := tm.taskProcessors.Load(taskID); ok {
		if processor, ok := processorVal.(*TaskProcessor); ok {
			reportErr = processor.Stop()
			logger.Info(tm.ctx, "[TaskManager] stopTask - direct processor update",
				zap.String("taskID", taskID))
		}
	}

	// 然后取消任务上下文
	if cancelFunc, ok := tm.taskContexts.Load(taskID); ok {
		if cancel, ok := cancelFunc.(context.CancelFunc); ok {
			cancel()
			logger.Info(tm.ctx, "[TaskManager] stopTask - context cancelled",
				zap.String("taskID", taskID))

			return reportErr
		}
		return fmt.Errorf("task %s cancel function invalid", taskID)
	}

	// 如果任务不存在，预先存储停止模式，等待任务启动时使用
	logger.Info(tm.ctx, "[TaskManager] stopTask pre-stored",
		zap.String("taskID", taskID))
	return reportErr
}

// pauseTask 暂停任务（不执行回滚，可恢复）
func (tm *TaskManager) pauseTask(taskID string) error {
	var reportErr error
	tm.taskContexts.Store(taskID+"_stop_mode", StopModePause)

	if processorVal, ok := tm.taskProcessors.Load(taskID); ok {
		if processor, ok := processorVal.(*TaskProcessor); ok {
			reportErr = processor.Pause()
			logger.Info(tm.ctx, "[TaskManager] pauseTask - direct processor update",
				zap.String("taskID", taskID))
		}
	}

	// 然后取消任务上下文
	if cancelFunc, ok := tm.taskContexts.Load(taskID); ok {
		if cancel, ok := cancelFunc.(context.CancelFunc); ok {
			cancel()
			logger.Info(tm.ctx, "[TaskManager] pauseTask - context cancelled",
				zap.String("taskID", taskID))
			return reportErr
		}
		return fmt.Errorf("task %s cancel function invalid", taskID)
	}

	// 如果任务不存在，预先存储暂停模式，等待任务启动时使用
	logger.Info(tm.ctx, "[TaskManager] pauseTask pre-stored",
		zap.String("taskID", taskID))
	return reportErr
}

// CancelTask 停止指定任务
func (tm *TaskManager) CancelTask(taskID string) error {
	return tm.cancelTask(taskID)
}

// StopTask 停止指定任务
func (tm *TaskManager) StopTask(taskID string) error {
	return tm.stopTask(taskID)
}

// PauseTask 暂停指定任务
func (tm *TaskManager) PauseTask(taskID string) error {
	return tm.pauseTask(taskID)
}

// ResumeTask 恢复指定任务 运行前确保任务状态为running
func (tm *TaskManager) ResumeTask(taskID string) error {
	logger.Info(tm.ctx, "[TaskManager] ResumeTask", zap.String("taskID", taskID))
	// 清理暂停时残留的预存 stop_mode，避免恢复时被旧的 pause 标记再次拦截。
	tm.taskContexts.Delete(taskID + "_stop_mode")
	if tm.isTaskExecutingInProcess(taskID) {
		// 旧执行流可能还在处理 pause 收尾，这时先挂起恢复请求，等退出后自动续跑。
		tm.taskContexts.Store(taskID+"_resume_requested", true)
		logger.Info(tm.ctx, "[TaskManager] ResumeTask deferred until current execution exits",
			zap.String("taskID", taskID))
		return nil
	}
	tm.wg.Go(func() {
		tm.executeTask(tm.ctx, &CallTaskReq{
			TaskID: taskID,
		})
	})
	return nil
}

func (tm *TaskManager) watchTasks(ctx context.Context) {
	taskChan, err := tm.DataProvider.WatchTask()
	if err != nil {
		logger.Error(ctx, "watchTasks", zap.Error(err))
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case taskData, ok := <-taskChan:
			if !ok {
				// channel 关闭，尝试重新建立连接
				taskChan, err = tm.DataProvider.WatchTask()
				if err != nil {
					logger.Error(ctx, "watchTasks", zap.Error(err))
					time.Sleep(5 * time.Second)
				}
				continue
			}

			tm.wg.Go(func() {
				tm.executeTask(ctx, taskData)
			})
		}
	}
}

func (tm *TaskManager) pollTasks(ctx context.Context) {
	ticker := time.NewTicker(tm.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		tasks, err := tm.DataProvider.ListenTask()
		if err != nil {
			logger.Error(ctx, "pollTasks", zap.Error(err))
			continue
		}

		for _, taskData := range tasks {
			tm.wg.GoCtx(func(ctx context.Context) {
				tm.executeTask(ctx, taskData)
			})
		}
	}
}

func (tm *TaskManager) executeTask(ctx context.Context, taskData *CallTaskReq) {
	if taskData == nil {
		return
	}
	if tm.wg == nil {
		logger.Panic(ctx, "TaskManager not start")
	}
	if tm.isTaskExecutingInProcess(taskData.TaskID) {
		logger.Warnf(ctx, "[TaskManager] executeTask taskID: %s, it is already executing in-process", taskData.TaskID)
		return
	}

	owner := uuid.NewString()
	if err := tm.tryLock(taskData.TaskID, owner); err != nil {
		logger.Warnf(ctx, "[TaskManager] executeTask tryRun taskID: %s, it is running now", taskData.TaskID)
		return
	}

	defer func() {
		if err := tm.unlock(taskData.TaskID, owner); err != nil && err != ErrTaskLockNotExists && err != ErrTaskLockOwnerMismatch {
			logger.Error(ctx, "[TaskManager] executeTask unlock", zap.Error(err), zap.String("taskID", taskData.TaskID))
		}
	}()
	err := tm.CanRunTask.CanRunTask(taskData.TaskID)
	if err != nil {
		logger.Error(ctx, "[TaskManager] executeTask can not run taskID: %s", zap.Error(err))
		return
	}

	taskCtx, taskCancel := context.WithCancel(ctx)
	defer taskCancel()
	tm.taskContexts.Store(taskData.TaskID, taskCancel)
	defer func() {
		tm.taskContexts.Delete(taskData.TaskID)
		// 无论任务是正常完成还是被 stop/pause/cancel 打断，退出时都清理辅助 stop_mode 标记。
		tm.taskContexts.Delete(taskData.TaskID + "_stop_mode")
		if shouldResume, ok := tm.taskContexts.LoadAndDelete(taskData.TaskID + "_resume_requested"); ok {
			if resumeRequested, ok := shouldResume.(bool); ok && resumeRequested {
				tm.wg.Go(func() {
					tm.executeTask(tm.ctx, &CallTaskReq{
						TaskID: taskData.TaskID,
					})
				})
			}
		}
	}()

	keepAliveCtx, cancelKeepAlive := context.WithCancel(taskCtx)
	defer cancelKeepAlive()
	tm.wg.GoCtx(func(ctx context.Context) {
		tm.keepAliveLock(keepAliveCtx, taskCancel, taskData.TaskID, owner)
	})

	processor, err := tm.buildProcessor(taskCtx, taskData)
	if err != nil {
		logger.Error(taskCtx, "[TaskManager] executeTask buildProcessor", zap.Error(err))
		if processor != nil {
			processor.TaskRegData.TaskData.Status = StatusFailed
			processor.reportTaskInfo(nil, nil)
		}
		return
	}

	// 存储processor引用，便于后续直接修改
	tm.taskProcessors.Store(taskData.TaskID, processor)
	defer tm.taskProcessors.Delete(taskData.TaskID)

	// 检查是否有预存的停止模式（任务尚未真正进入执行器前就收到了停止/暂停）
	if storedMode, ok := tm.taskContexts.Load(taskData.TaskID + "_stop_mode"); ok {
		if mode, ok := storedMode.(StopMode); ok {
			processor.stopMode = mode
			// 清除已使用的停止模式
			tm.taskContexts.Delete(taskData.TaskID + "_stop_mode")
			processor.TaskRegData.TaskData.Status = stopModeToTaskStatus(mode)
			if err := processor.reportTaskInfo(nil, nil); err != nil {
				logger.Error(taskCtx, "[TaskManager] executeTask report pre-stopped task", zap.Error(err))
			}
			return
		}
	}

	processCtx := taskCtx
	if processor.TaskRegData.TaskData.ExpiredAt != nil {
		if clock.Now().After(*processor.TaskRegData.TaskData.ExpiredAt) {
			logger.Error(taskCtx, "[TaskManager] executeTask task expired", zap.Error(err))
			processor.TaskRegData.TaskData.Status = StatusTimeout
			processor.reportTaskInfo(nil, nil)
			return
		}
		remainingTime := time.Until(*processor.TaskRegData.TaskData.ExpiredAt)
		var cancel context.CancelFunc
		processCtx, cancel = context.WithTimeout(taskCtx, remainingTime)
		defer cancel()
	}

	if err := processor.Process(processCtx); err != nil {
		logger.Error(taskCtx, "[TaskManager] executeTask process",
			zap.String("taskID", taskData.TaskID),
			zap.String("taskCode", taskData.TaskCode),
			zap.Error(err))
		return
	}
}

func (tm *TaskManager) buildProcessor(ctx context.Context, taskData *CallTaskReq) (*TaskProcessor, error) {
	taskInfo, err := tm.engine.StepInfoProvider.GetTaskStepInfo(taskData.TaskID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTaskGetFailed, err)
	}
	// 新任务或等待状态的任务，从头开始构建
	if taskInfo.Status == StatusWaiting {
		processor, err := tm.engine.Build(ctx, taskInfo)
		if err != nil {
			return nil, err
		}
		// 设置默认停止模式
		processor.stopMode = StopModeCancel
		return processor, nil
	}

	// 已完成的任务不再处理
	if isFinishedStatus(taskInfo.Status) {
		return nil, ErrTaskAlreadyFinished
	}

	// 恢复未完成的任务
	processor, err := tm.engine.Revert(ctx, taskInfo)
	if err != nil {
		return nil, err
	}
	// 设置默认停止模式
	processor.stopMode = StopModeCancel
	return processor, nil
}

func (tm *TaskManager) tryLock(taskID string, owner string) error {
	return tm.LockProvider.Lock(taskID, owner)
}

func (tm *TaskManager) unlock(taskID string, owner string) error {
	return tm.LockProvider.Unlock(taskID, owner)
}

func (tm *TaskManager) keepAliveLock(ctx context.Context, cancel context.CancelFunc, taskID string, owner string) {
	ticker := time.NewTicker(tm.LockKeepInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// 续锁失败可能导致任务被其他节点抢占
			if err := tm.LockProvider.KeepAlive(taskID, owner); err != nil {
				logger.Error(ctx, "[TaskManager] keepAliveLock keepAlive", zap.Error(err))
				if err == ErrTaskLockNotExists || err == ErrTaskLockOwnerMismatch {
					cancel()
				}
				return
			}
		}
	}
}

func (tm *TaskManager) isTaskExecutingInProcess(taskID string) bool {
	if _, ok := tm.taskContexts.Load(taskID); ok {
		return true
	}
	if _, ok := tm.taskProcessors.Load(taskID); ok {
		return true
	}
	return false
}
