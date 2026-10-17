package taskmgr

import (
	"context"
	"time"
)

// StepRegInfo 注册时的步骤信息
type StepRegInfo struct {
	StepWorker StepWorker
	StepData   *StepData // 步骤数据
}

// StepExecResult 步骤执行结果
type StepExecResult struct {
	Status          Status
	ProcessPosition int
	Message         string // 执行消息
	Err             error
	Payload         []byte
	PreStepResult   []byte
	Result          []byte
}

// StepExecCompleted 步骤执行完成
func StepExecCompleted(result []byte) StepExecResult {
	return StepExecResult{
		Status:          StatusCompleted,
		ProcessPosition: 100,
		Result:          result,
	}
}

func StepExecPending(result []byte) StepExecResult {
	return StepExecResult{
		Status: StatusPending,
		Result: result,
	}
}

// StepExecCompletedWithError 步骤执行完成但有错误
func StepExecCompletedWithError(err error, result []byte) StepExecResult {
	return StepExecResult{
		Status:          StatusCompleted,
		ProcessPosition: 100,
		Err:             err,
		Result:          result,
	}
}

// StepExecFailed 步骤执行失败
func StepExecFailed(err error) StepExecResult {
	return StepExecResult{
		Status: StatusFailed,
		Err:    err,
		Result: make([]byte, 0),
	}
}

// StepPollStatusResult 步骤轮询状态结果
type StepPollStatusResult struct {
	Status          Status
	ProcessPosition int
	ProcessMessage  string
	Err             error
	Result          []byte
}

// StepPollCompleted 步骤轮询完成
func StepPollCompleted(result []byte) StepPollStatusResult {
	return StepPollStatusResult{
		Status:          StatusCompleted,
		ProcessPosition: 100,
		Result:          result,
	}
}

// StepPollCompletedWithError 步骤轮询完成但有错误，不回滚，错误传递给上层任务
func StepPollCompletedWithError(err error, result []byte) StepPollStatusResult {
	return StepPollStatusResult{
		Status:          StatusCompleted,
		ProcessPosition: 100,
		Err:             err,
		Result:          result,
	}
}

// StepPollFailed 步骤轮询失败
func StepPollFailed(err error) StepPollStatusResult {
	return StepPollStatusResult{
		Status: StatusFailed,
		Err:    err,
	}
}

// StepPollPending 步骤轮询中
// processPosition 进度百分比,范围0-100
func StepPollPending(processPosition int, result []byte) StepPollStatusResult {
	return StepPollStatusResult{
		Status:          StatusPending,
		ProcessPosition: processPosition,
		Result:          result,
	}
}

// StepInput 步骤执行输入
type StepInput struct {
	TaskID        string      // 任务ID
	Payload       []byte      // 任务原始载荷
	PreStepResult []byte      // 上一步骤的结果
	Context       *ContextMap // 扩展上下文信息
}

// StepPollInput 步骤轮询输入
type StepPollInput struct {
	TaskID     string      // 任务ID
	Payload    []byte      // 任务原始载荷
	StepResult []byte      // 步骤执行时返回的结果
	Context    *ContextMap // 扩展上下文信息
}
type StepPollAfterInput struct {
	StepPollInput
	AfterResult StepExecResult
}

// StepRollbackInput 步骤回滚输入
type StepRollbackInput struct {
	TaskID     string      // 任务ID
	Payload    []byte      // 任务原始载荷
	StepResult []byte      // 步骤执行时的结果
	Context    *ContextMap // 扩展上下文信息
}

// StepWorker 步骤执行器接口
type StepWorker interface {
	// Execute 执行步骤
	Execute(ctx context.Context, input StepInput) (execResult StepExecResult)
	// PollStatus 轮询步骤状态
	PollStatus(ctx context.Context, input StepPollInput) (execResult StepPollStatusResult)
	// Rollback	回滚步骤
	Rollback(ctx context.Context, input StepRollbackInput) (err error)
}

type StepWorkerPause interface {
	// Pause 暂停步骤
	Pause(ctx context.Context, input StepInput) error
}
type StepWorkerStop interface {
	// Stop 停止步骤
	Stop(ctx context.Context, input StepInput) error
}
type StepWorkerCancel interface {
	// Cancel 停止步骤
	Cancel(ctx context.Context, input StepInput) error
}

type BaseStepWorker struct {
	StepWorker
	StepWorkerPause
	StepWorkerStop
}

type StepPullStatusAfter interface {
	After(ctx context.Context, input StepPollAfterInput) error
}

// Option 步骤选项接口
type Option interface {
	apply(stepInfo *StepRegInfo)
}

type stepInfoOption struct {
	pollStepInterval time.Duration
}

func (s stepInfoOption) apply(stepInfo *StepRegInfo) {
	stepInfo.StepData.PollStepInterval = s.pollStepInterval
}

// WithPollInterval 设置轮询间隔
func WithPollInterval(pollStepInterval time.Duration) Option {
	return stepInfoOption{
		pollStepInterval: pollStepInterval,
	}
}

// NewStepInfo 创建步骤信息
func NewStepInfo(name string, stepWorker StepWorker, options ...Option) StepRegInfo {
	stepInfo := StepRegInfo{
		StepWorker: stepWorker,
		StepData: &StepData{
			Name:             name,
			Status:           StatusWaiting,
			PollStepInterval: 500 * time.Millisecond,
		},
	}
	for _, option := range options {
		option.apply(&stepInfo)
	}
	return stepInfo
}
