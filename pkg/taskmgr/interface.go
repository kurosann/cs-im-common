package taskmgr

import "errors"

var (
	ErrTaskAlreadyRunning    = errors.New("task already running")
	ErrTaskLocked            = errors.New("task is locked")
	ErrTaskLockNotExists     = errors.New("task lock not exists")
	ErrTaskLockOwnerMismatch = errors.New("task lock owner mismatch")
)

type ITaskDataProvider interface {
	WatchTask() (TaskReceiveChan, error)
	ListenTask() ([]*CallTaskReq, error)
}

type ITaskReporter interface {
	ReportTaskInfo(report *TaskReport) error
	ReportStepInfo(report *StepReport) error
}

type ITaskStepInfoProvider interface {
	GetTaskStepInfo(taskId string) (*TaskData, error)
}

type ITaskLockProvider interface {
	Lock(taskId string, owner string) error
	Unlock(taskId string, owner string) error
	KeepAlive(taskId string, owner string) error
}

type ICanRunTask interface {
	CanRunTask(taskId string) error
}
