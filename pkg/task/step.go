package task

import (
	"context"
	"time"
)

// StepExecResult 步骤执行结果
type StepExecResult struct {
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

// StepExecCompleted 步骤执行完成
func StepExecCompleted(result []byte) StepExecResult {
	return StepExecResult{
		Status:          StatusCompleted,
		ProcessPosition: 100,
		Result:          result,
	}
}

// StepExecPending 步骤执行中
func StepExecPending(result []byte) StepExecResult {
	return StepExecResult{
		Status: StatusPending,
		Result: result,
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

// StepExecFailedWithResult 步骤执行失败
func StepExecFailedWithResult(err error, result []byte) StepExecResult {
	return StepExecResult{
		Status: StatusFailed,
		Result: result,
		Err:    err,
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
		ProcessMessage:  "completed",
		Result:          result,
	}
}

// StepPollFailed 步骤轮询失败
func StepPollFailed(err error) StepPollStatusResult {
	return StepPollStatusResult{
		Status:         StatusFailed,
		ProcessMessage: "failed",
		Err:            err,
	}
}

// StepPollPending 步骤轮询中
// processPosition 进度百分比,范围0-100
func StepPollPending(processPosition int, processMessage string) StepPollStatusResult {
	return StepPollStatusResult{
		Status:          StatusPending,
		ProcessPosition: processPosition,
		ProcessMessage:  processMessage,
	}
}

// StepWorker 步骤执行器
type StepWorker interface {
	// Execute 执行步骤
	Execute(ctx context.Context, payload, preStepResult []byte) (execResult StepExecResult)
	// PollStatus 轮询步骤状态
	PollStatus(ctx context.Context, payload, stepResult []byte) (execResult StepPollStatusResult)
	// Rollback	回滚步骤
	Rollback(ctx context.Context, payload, stepResult []byte) (err error)
}

// StepInfo 步骤信息
type StepInfo struct {
	TaskID           string
	StepID           string
	Order            int
	Name             string
	StepWorker       StepWorker
	pollStepInterval time.Duration
}
type Option interface {
	apply(stepInfo *StepInfo)
}

type stepInfoOption struct {
	pollStepInterval time.Duration
}

func (s stepInfoOption) apply(stepInfo *StepInfo) {
	stepInfo.pollStepInterval = s.pollStepInterval
}

func WithPollInterval(pollStepInterval time.Duration) Option {
	return stepInfoOption{
		pollStepInterval: pollStepInterval,
	}
}

func NewStepInfo(name string, stepWorker StepWorker, options ...Option) StepInfo {
	stepInfo := StepInfo{
		Name:             name,
		StepWorker:       stepWorker,
		pollStepInterval: 500 * time.Millisecond,
	}
	for _, option := range options {
		option.apply(&stepInfo)
	}
	return stepInfo
}

// StepRecorder 步骤记录器
type StepRecorder interface {
	// RecordStep 记录步骤执行结果
	// stepID 为0时创建并生成步骤ID,不允许失败
	RecordStep(stepID string, result StepExecResult) (id string, err error)
	QueryStep(stepID string) (result StepExecResult, err error)
}
