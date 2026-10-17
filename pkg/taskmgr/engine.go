package taskmgr

import (
	"context"
	"errors"
	"fmt"
)

var (
	ErrTaskWorkerNotFound  = errors.New("task worker not found")
	ErrTaskAlreadyFinished = errors.New("task already finished")
	ErrTaskGetFailed       = errors.New("task get failed")
)

type TaskEngine struct {
	TaskWorkers      map[string]TaskWorker
	StepInfoProvider ITaskStepInfoProvider
	Reporter         ITaskReporter
	LockProvider     ITaskLockProvider
}

func newTaskEngine(
	stepConfigProvider ITaskStepInfoProvider,
	reporter ITaskReporter,
	lockProvider ITaskLockProvider,
) *TaskEngine {
	return &TaskEngine{
		TaskWorkers:      make(map[string]TaskWorker),
		StepInfoProvider: stepConfigProvider,
		Reporter:         reporter,
		LockProvider:     lockProvider,
	}
}

func (te *TaskEngine) RegisterTaskWorker(taskCode string, worker TaskWorker) error {
	if _, ok := te.TaskWorkers[taskCode]; ok {
		return fmt.Errorf("task worker [%s] already registered", taskCode)
	}
	te.TaskWorkers[taskCode] = worker
	return nil
}

// GetTaskWorker 获取任务处理器
func (te *TaskEngine) GetTaskWorker(taskCode string) (TaskWorker, bool) {
	worker, ok := te.TaskWorkers[taskCode]
	return worker, ok
}

func (te *TaskEngine) Build(ctx context.Context, taskData *TaskData) (*TaskProcessor, error) {
	worker, ok := te.TaskWorkers[taskData.TaskCode]
	if !ok {
		return nil, ErrTaskWorkerNotFound
	}

	steps := worker.Arrange(ctx, taskData.Payload)

	taskContext := &ContextMap{}
	if taskData.Context != nil {
		taskContext.ParseMap(taskData.Context)
	}

	return &TaskProcessor{
		ctx: ctx,
		TaskRegData: TaskRegInfo{
			TaskData: taskData,
			Steps:    steps,
			Context:  taskContext,
		},
		Reporter:     te.Reporter,
		TaskWorker:   worker,
		stepProgress: map[string]int{},
	}, nil
}

func (te *TaskEngine) Revert(ctx context.Context, taskData *TaskData) (*TaskProcessor, error) {
	worker, ok := te.TaskWorkers[taskData.TaskCode]
	if !ok {
		return nil, ErrTaskWorkerNotFound
	}

	stepRegInfos := worker.Arrange(ctx, taskData.Payload)

	// 从外部提供的步骤信息中获取已存在的步骤数据（执行过并上报的步骤）
	stepInfoMap := make(map[string]*StepData)
	for _, stepInfo := range taskData.Steps {
		if stepInfo != nil {
			stepInfoMap[stepInfo.Name] = stepInfo
		}
	}

	for i := range stepRegInfos {
		// 如果外部提供了该步骤的信息（已执行过），恢复其状态
		if stepInfo, ok := stepInfoMap[stepRegInfos[i].StepData.Name]; ok {
			// 恢复步骤状态，但保留注册时设置的轮询间隔
			pollInterval := stepRegInfos[i].StepData.PollStepInterval
			// 使用外部提供的 TaskID，恢复需要恢复的字段
			stepRegInfos[i].StepData.TaskID = stepInfo.TaskID
			stepRegInfos[i].StepData.Status = stepInfo.Status
			stepRegInfos[i].StepData.Err = stepInfo.Err
			stepRegInfos[i].StepData.Message = stepInfo.Message
			stepRegInfos[i].StepData.Payload = stepInfo.Payload
			stepRegInfos[i].StepData.PreStepResult = stepInfo.PreStepResult
			stepRegInfos[i].StepData.Result = stepInfo.Result
			stepRegInfos[i].StepData.ProcessPosition = stepInfo.ProcessPosition
			stepRegInfos[i].StepData.PollStepInterval = pollInterval
		} else {
			stepRegInfos[i].StepData.TaskID = taskData.TaskID
		}
	}

	taskContext := &ContextMap{}
	if taskData.Context != nil {
		taskContext.ParseMap(taskData.Context)
	}

	return &TaskProcessor{
		ctx: ctx,
		TaskRegData: TaskRegInfo{
			TaskData: taskData,
			Steps:    stepRegInfos,
			Context:  taskContext,
		},
		Reporter:     te.Reporter,
		TaskWorker:   worker,
		stepProgress: map[string]int{},
	}, nil
}
