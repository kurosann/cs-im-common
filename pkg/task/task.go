package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"vane-common/pkg/safe"
)

type Status string

const (
	StatusPending     Status = "pending"
	StatusRunning     Status = "running"
	StatusCompleted   Status = "completed"
	StatusRollingBack Status = "rollingBack"
	StatusRolledBack  Status = "rolledBack"
	StatusFailed      Status = "failed"
	StatusTimeout     Status = "timeout"
	StatusCanceled    Status = "canceled"
)

type TaskInfo struct {
	Name string
	Task TaskWorker
}

func NewTaskInfo(name string, task TaskWorker) *TaskInfo {
	return &TaskInfo{Name: name, Task: task}
}

type TaskRecorder interface {
	// RecordTask 记录任务执行结果
	// taskID 为0时创建并生成任务ID,不允许失败
	RecordTask(taskID string, result TaskExecResult) (id string, err error)
	QueryTask(taskID string) (result TaskExecResult, err error)
}
type TaskExecResult struct {
	TaskCode        string
	Status          Status
	Order           int
	StepName        string
	TaskID          string
	StepID          string
	ProcessPosition int
	Err             error
	Payload         []byte
	Result          []byte
}
type Recorder interface {
	StepRecorder
	TaskRecorder
}

type TaskManager struct {
	TaskWorkers map[string]*TaskInfo
	mu          sync.RWMutex
	Recorder    Recorder
}

var _ Recorder = &DefaultRecorder{}

type DefaultRecorder struct {
}

func (d *DefaultRecorder) QueryStep(stepID string) (result StepExecResult, err error) {
	return StepExecResult{}, nil
}

func (d *DefaultRecorder) QueryTask(taskID string) (result TaskExecResult, err error) {
	return TaskExecResult{}, nil
}

func (d *DefaultRecorder) RecordTask(taskID string, result TaskExecResult) (id string, err error) {
	return "", nil
}
func (d *DefaultRecorder) RecordStep(stepID string, result StepExecResult) (id string, err error) {
	return "", nil
}

var (
	globalTaskManager *TaskManager
	once              sync.Once
)

// Deprecated
func InitGlobalTaskManager(recorder Recorder) {
	once.Do(func() {
		globalTaskManager = NewTaskManager(recorder)
	})
}

// Deprecated
func GetGlobalTaskManager() *TaskManager {
	return globalTaskManager
}

// NewTaskManager 创建任务管理器
// Deprecated
func NewTaskManager(recorder Recorder) *TaskManager {
	if recorder == nil {
		recorder = &DefaultRecorder{}
	}
	return &TaskManager{
		TaskWorkers: make(map[string]*TaskInfo),
		Recorder:    recorder,
	}
}

// RegisterTask 注册任务
func (tm *TaskManager) RegisterTask(task *TaskInfo) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tm.TaskWorkers[task.Name] = task
}

var (
	ErrTaskNotFound = errors.New("task not found")
)

// RunTask 执行任务
func (tm *TaskManager) RunTask(ctx context.Context, taskCode string, payload []byte) (result []byte, err error) {
	result, _, err = tm.RunTask2(ctx, taskCode, payload)
	return result, err
}

// RunTask 执行任务 并返回id
func (tm *TaskManager) RunTask2(ctx context.Context, taskCode string, payload []byte) (result []byte, taskId string, err error) {
	tm.mu.RLock()
	task, ok := tm.TaskWorkers[taskCode]
	defer tm.mu.RUnlock()
	if !ok {
		return nil, "", fmt.Errorf("%s %w", taskCode, ErrTaskNotFound)
	}
	steps := task.Task.Arrange(ctx, payload)
	if steps.NewPayload != nil {
		payload = steps.NewPayload
	}
	taskWorker := &TaskWorkerProcessor{
		TaskCode: taskCode,
		Steps:    steps.Steps,
		Recorder: tm.Recorder,
	}
	taskID, err := taskWorker.Recorder.RecordTask("", TaskExecResult{Status: StatusRunning, Payload: payload})
	if err != nil {
		return nil, "", err
	}
	taskResult := taskWorker.Process(ctx, payload, taskID)
	if taskResult.Err != nil {
		return nil, taskID, taskResult.Err
	}
	return taskResult.Result, taskID, nil
}

func (tm *TaskManager) AsyncRunTask(ctx context.Context, taskCode string, payload []byte) (taskID string, err error) {
	tm.mu.RLock()
	task, ok := tm.TaskWorkers[taskCode]
	defer tm.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("%s %w", taskCode, ErrTaskNotFound)
	}

	taskWorker := &TaskWorkerProcessor{
		TaskCode: taskCode,
		Recorder: tm.Recorder,
	}
	taskID, err = taskWorker.Recorder.RecordTask("", TaskExecResult{Status: StatusRunning, Payload: payload})
	if err != nil {
		return "", err
	}
	steps := task.Task.Arrange(ctx, payload)
	if steps.NewPayload != nil {
		payload = steps.NewPayload
	}
	taskWorker.Steps = steps.Steps
	safe.Go(func() {
		_ = taskWorker.Process(ctx, payload, taskID)
	})
	return taskID, nil
}

func (tm *TaskManager) AsyncReRunTask(ctx context.Context, taskID string) (err error) {
	result, err := tm.Recorder.QueryTask(taskID)
	if err != nil {
		return err
	}
	tm.mu.RLock()
	task, ok := tm.TaskWorkers[result.TaskCode]
	defer tm.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%s %w", result.TaskCode, ErrTaskNotFound)
	}

	taskWorker := &TaskWorkerProcessor{
		TaskCode: result.TaskCode,
		Recorder: tm.Recorder,
	}
	taskID, err = taskWorker.Recorder.RecordTask(taskID, TaskExecResult{Status: StatusRunning, Payload: result.Payload})
	if err != nil {
		return err
	}
	steps := task.Task.Arrange(ctx, result.Payload)
	if steps.NewPayload != nil {
		result.Payload = steps.NewPayload
	}
	taskWorker.Steps = steps.Steps
	safe.Go(func() {
		_ = taskWorker.Process(ctx, result.Payload, taskID)
	})
	return nil
}

type TaskIdCtx string

const (
	TaskIdCtxKey TaskIdCtx = "taskWorker"
)

func GetTaskId(ctx context.Context) string {
	return ctx.Value(TaskIdCtxKey).(string)
}

func (tm *TaskManager) QueryTask(taskID string) (result TaskExecResult, err error) {
	return tm.Recorder.QueryTask(taskID)
}

// TaskWorkerProcessor 任务执行器
type TaskWorkerProcessor struct {
	TaskCode string
	Steps    []StepInfo
	Recorder Recorder
}

// Process 执行任务
func (tp *TaskWorkerProcessor) Process(ctx context.Context, payload []byte, taskID string) (result TaskExecResult) {
	ctx = context.WithValue(ctx, TaskIdCtxKey, taskID)
	result.TaskID = taskID

	// 记录已执行的步骤和它们的结果
	type executedStep struct {
		Step       StepInfo
		ExecResult StepExecResult
	}
	executedSteps := make([]executedStep, 0)
	stepResult := make([]byte, 0)
	defer func() {
		if result.Err != nil {
			rollbackErrs := make([]error, 0)
			// 若task的最后一个步骤是pending，则进行回滚
			// 如果执行失败，则逆向回滚已执行的步骤
			for i := len(executedSteps) - 1; i >= 0; i-- {
				step := executedSteps[i]
				if i == len(executedSteps)-1 && step.ExecResult.Status != StatusPending {
					continue
				}
				step.ExecResult.Status = StatusRollingBack
				step.ExecResult.Payload = payload
				tp.Record(taskID, step.Step, step.ExecResult)
				// 全部执行过的步骤进行rollback
				if err := safe.TryE(func() error {
					return step.Step.StepWorker.Rollback(ctx, step.ExecResult.Payload, step.ExecResult.Result)
				}); err != nil {
					rollbackErrs = append(rollbackErrs, fmt.Errorf("rollback %d failed: %w", i+1, err))
				}
				step.ExecResult.Status = StatusRolledBack
				step.ExecResult.Payload = payload
				tp.Record(taskID, step.Step, step.ExecResult)
			}
			tp.Recorder.RecordTask(taskID, TaskExecResult{Status: result.Status, Payload: payload, Result: stepResult})
			result.Err = errors.Join(append([]error{result.Err}, rollbackErrs...)...)
		}
		tp.Recorder.RecordTask(taskID, result)
	}()

	for i, step := range tp.Steps {
		step.TaskID = taskID
		select {
		case <-ctx.Done():
			result.Err = context.Cause(ctx)
			result.Status = StatusTimeout
			return result
		default:
			stepExecResult := StepExecResult{Status: StatusRunning, TaskID: taskID, Payload: payload}
			step.Order = i + 1
			step.StepID, result = tp.Record(taskID, step, stepExecResult)

			// 使用channel来处理Execute的超时
			execDone := make(chan StepExecResult, 1)

			wg := safe.NewWaitGroup()
			wg.Go(func() {
				defer close(execDone)
				execDone <- step.StepWorker.Execute(ctx, payload, stepResult)
			})

			// 等待执行完成或context取消
			select {
			case <-ctx.Done():
				stepExecResult.Status = StatusTimeout
				stepExecResult.Err = ctx.Err()
				stepExecResult.Payload = payload
			case stepExecResult = <-execDone:
				if err := wg.WaitAndRecover(); err != nil {
					stepExecResult.Status = StatusFailed
					stepExecResult.Err = fmt.Errorf("步骤执行panic: %w", err)
					stepExecResult.Payload = payload
				}
				stepResult = stepExecResult.Result
			}

			if stepExecResult.Status == StatusFailed ||
				stepExecResult.Status == StatusTimeout {
				step.StepID, result = tp.Record(taskID, step, stepExecResult)
				result.Err = stepExecResult.Err
				result.Status = stepExecResult.Status
			}
			// 只有pending状态需要轮询
			if stepExecResult.Status == StatusPending {
				done := make(chan struct{})
				wg := safe.NewWaitGroup()
				wg.GoE(func() error {
					defer close(done)
					stepExecResult = tp.PollStatus(ctx, taskID, step, payload, stepResult)
					if stepExecResult.Result != nil {
						stepResult = stepExecResult.Result
					}
					if stepExecResult.Err != nil {
						return stepExecResult.Err
					}
					return nil
				})

				select {
				case <-ctx.Done():
					// done优先级高于ctx.Done()
					select {
					case <-done:
					default:
						stepExecResult.Payload = payload
						stepExecResult.Err = context.Cause(ctx)
						stepExecResult.Status = StatusTimeout
						step.StepID, result = tp.Record(taskID, step, stepExecResult)
						result.Err = context.Cause(ctx)
						result.Status = StatusTimeout
						goto skipPollStatus
					}
				case <-done:
				}
				if err := wg.WaitAndRecover(); err != nil {
					stepExecResult.Err = err
					stepExecResult.Payload = payload
					step.StepID, result = tp.Record(taskID, step, stepExecResult)
					result.Err = err
					result.Status = stepExecResult.Status
				}
			skipPollStatus:
			}
			// 记录完整的执行结果
			executedSteps = append(executedSteps, executedStep{
				Step:       step,
				ExecResult: stepExecResult,
			})
			if result.Status == StatusFailed ||
				result.Status == StatusTimeout {
				return
			}
			step.StepID, result = tp.Record(taskID, step, stepExecResult)
			result.Result = stepResult
		}
	}
	return result
}

// PollStatus 轮询步骤状态
func (tp *TaskWorkerProcessor) PollStatus(ctx context.Context, taskID string, step StepInfo, payload []byte, stepResult []byte) (result StepExecResult) {
	ticker := time.NewTicker(step.pollStepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return StepExecResult{Status: StatusTimeout, Err: context.Cause(ctx)}
		case <-ticker.C:
			var monitorResult StepPollStatusResult
			if err := safe.Try(func() {
				monitorResult = step.StepWorker.PollStatus(ctx, payload, stepResult)
			}); err != nil {
				monitorResult = StepPollStatusResult{
					Status: StatusFailed,
					Err:    fmt.Errorf("状态轮询panic: %w", err),
				}
			}

			stepExecResult := StepExecResult{
				StepID:          step.StepID,
				Order:           step.Order,
				StepName:        step.Name,
				TaskID:          step.TaskID,
				Status:          monitorResult.Status,
				ProcessPosition: monitorResult.ProcessPosition,
				Result:          monitorResult.Result,
				Err:             monitorResult.Err,
			}

			if monitorResult.Status == StatusPending {
				tp.Record(taskID, step, stepExecResult)
				continue
			}
			return stepExecResult
		}
	}
}

// Record 记录步骤执行结果
func (tp *TaskWorkerProcessor) Record(taskID string, step StepInfo, result StepExecResult) (stepID string, taskResult TaskExecResult) {
	result.StepID = step.StepID
	result.Order = step.Order
	result.StepName = step.Name
	stepID, err := tp.Recorder.RecordStep(step.StepID, result)
	if err != nil {
		result.Err = err
		result.Status = StatusFailed
		return "", TaskExecResult{Status: result.Status, Err: result.Err}
	}
	taskResult = TaskExecResult{
		TaskCode:        tp.TaskCode,
		Status:          result.Status,
		Order:           step.Order,
		StepName:        step.Name,
		TaskID:          taskID,
		StepID:          stepID,
		ProcessPosition: result.ProcessPosition,
		Err:             result.Err,
		Result:          result.Result,
	}
	tp.Recorder.RecordTask(taskID, taskResult)
	return stepID, taskResult
}

type StepSeq struct {
	NewPayload []byte // 新的payload 为空使用入参payload
	Steps      []StepInfo
}

func NewStepSeq() *StepSeq {
	return &StepSeq{}
}

func (s *StepSeq) AddStep(step ...StepInfo) *StepSeq {
	s.Steps = append(s.Steps, step...)
	return s
}

func (s *StepSeq) SetNewPayload(newPayload []byte) *StepSeq {
	s.NewPayload = newPayload
	return s
}

// TaskWorker 任务执行器
type TaskWorker interface {
	// Arrange 步骤编排
	Arrange(ctx context.Context, payload []byte) (steps *StepSeq)
}
