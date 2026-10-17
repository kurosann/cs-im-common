package taskmgr

import "time"

// TaskReport 任务上报信息
type TaskReport struct {
	TaskID          string
	TaskCode        string
	Status          Status
	ProcessPosition int            // 任务整体进度 0-100
	RunningStepName string         // 正在执行的步骤编码
	Message         string         // 进度消息
	Error           error          // 错误信息
	Context         map[string]any // 任务执行上下文（用于恢复）
	Payload         []byte         // 任务原始载荷
	Result          []byte         // 任务执行时返回的结果
}

// StepReport 步骤上报信息
type StepReport struct {
	TaskID           string
	StepName         string // 步骤名称（TaskID + StepName 唯一确定一个步骤）
	Status           Status
	PollStepInterval time.Duration // 轮询间隔
	ProcessPosition  int           // 步骤进度 0-100
	Message          string        // 进度消息
	Error            error         // 错误信息
	Payload          []byte        // 任务原始载荷
	Result           []byte        // 步骤执行时返回的结果
	PreStepResult    []byte        // 上一步骤的结果
}
