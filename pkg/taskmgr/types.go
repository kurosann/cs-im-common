package taskmgr

import (
	"sync"
	"time"
)

// Status 任务/步骤状态
type Status string

const (
	StatusWaiting        Status = "waiting"        // 等待从头开始运行
	StatusRunning        Status = "running"        // 正在执行（同步执行阶段）
	StatusPending        Status = "pending"        // 异步执行中，需要轮询状态
	StatusCompleted      Status = "completed"      // 执行成功
	StatusRollingBack    Status = "rollingBack"    // 回滚中
	StatusRolledBack     Status = "rolledBack"     // 回滚完成
	StatusRollbackFailed Status = "rollbackFailed" // 回滚失败
	StatusFailed         Status = "failed"         // 执行失败
	StatusTimeout        Status = "timeout"        // 执行超时
	StatusCanceled       Status = "canceled"       // 已取消（中断并回滚）
	StatusStopped        Status = "stopped"        // 已停止（中断不回滚）
	StatusPaused         Status = "paused"         // 已暂停（中断保持状态）
)

func (s Status) IsCompleted() bool {
	return s == StatusCompleted
}
func (s Status) IsFailed() bool {
	return s == StatusFailed || s == StatusTimeout || s == StatusCanceled || s == StatusStopped || s == StatusRollbackFailed || s == StatusRollingBack || s == StatusRolledBack
}

// CallTaskReq 对外的任务数据
type CallTaskReq struct {
	TaskID   string
	TaskCode string // 任务代码，用于标识任务类型
}

// TaskRegInfo 对内的任务数据
type TaskRegInfo struct {
	TaskData *TaskData     // 任务基础数据
	Steps    []StepRegInfo // 可执行的步骤信息
	Context  *ContextMap   // 任务执行上下文
}

// StepData 运行时的步骤信息
type StepData struct {
	TaskID           string        // 所属任务ID
	Name             string        // 步骤名称（TaskID + Name 唯一确定一个步骤）
	Status           Status        // 步骤状态
	Err              error         // 步骤执行错误
	Message          string        // 步骤执行消息
	Payload          []byte        // 步骤执行输入
	PreStepResult    []byte        // 上一步骤的执行结果
	Result           []byte        // 步骤执行结果
	PollStepInterval time.Duration // 轮询间隔
	ProcessPosition  int
}

// TaskData 任务信息
type TaskData struct {
	TaskID          string
	TaskCode        string
	RunningStepName string // 正在执行的步骤编码
	Message         string // 任务执行消息
	Payload         []byte
	Result          []byte
	Status          Status
	Steps           []*StepData
	Context         map[string]any // 任务执行上下文
	ExpiredAt       *time.Time     // 任务过期时间
}

// TaskReceiveChan 任务数据接收通道
type TaskReceiveChan chan *CallTaskReq

// isFinishedStatus 判断是否为结束状态
func isFinishedStatus(status Status) bool {
	return status == StatusCompleted ||
		status == StatusRolledBack ||
		status == StatusFailed ||
		status == StatusCanceled ||
		status == StatusTimeout ||
		status == StatusStopped
	// 注意：StatusPaused 不应该被视为结束状态，因为可以恢复执行
}

type ContextMap struct {
	m sync.Map
}

func (c *ContextMap) ParseMap(m map[string]any) {
	for key, value := range m {
		c.m.Store(key, value)
	}
}

func (c *ContextMap) Get(key string) (any, bool) {
	return c.m.Load(key)
}
func (c *ContextMap) Set(key string, value any) {
	c.m.Store(key, value)
}
func (c *ContextMap) Delete(key string) {
	c.m.Delete(key)
}
func (c *ContextMap) Range(f func(key string, value any) bool) {
	c.m.Range(func(key, value any) bool {
		return f(key.(string), value)
	})
}
