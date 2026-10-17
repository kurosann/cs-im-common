package depend

import (
	"errors"
	"sync"

	"github.com/google/uuid"

	"vane-common/pkg/taskmgr"
)

// LocalTaskStore 是 standalone 模式的进程内任务存储与分发器，
// 一体实现 taskmgr 的四个 Provider 接口（ITaskStepInfoProvider / ITaskReporter /
// ITaskDataProvider / ICanRunTask），替代集群模式下的 CenterServer TaskManagerService
// （standalone 下 CenterServer 不可达）。
//
// 任务状态仅存内存，不跨进程持久化：进程重启后丢失，崩溃恢复由业务自身的
// 幂等重推兜底（如 payment 对 paid 未发货订单的周期重推 Job）。
type LocalTaskStore struct {
	mu    sync.RWMutex
	tasks map[string]*taskmgr.TaskData
	watch chan *taskmgr.CallTaskReq
}

func NewLocalTaskStore() *LocalTaskStore {
	return &LocalTaskStore{
		tasks: make(map[string]*taskmgr.TaskData),
		watch: make(chan *taskmgr.CallTaskReq, 100),
	}
}

var (
	// ErrLocalTaskNotFound 任务不存在（可能已被清理或 ID 错误）
	ErrLocalTaskNotFound = errors.New("local task not found")
	// ErrLocalTaskQueueFull 派发 channel 已满（任务仍处 waiting，由 ListenTask 轮询兜底补投）
	ErrLocalTaskQueueFull = errors.New("local task queue full")
)

// SubmitTask 创建任务（waiting）并投入派发 channel。
func (s *LocalTaskStore) SubmitTask(taskCode string, payload []byte) (string, error) {
	t := &taskmgr.TaskData{
		TaskID:   uuid.NewString(),
		TaskCode: taskCode,
		Status:   taskmgr.StatusWaiting,
		Payload:  payload,
		Context:  map[string]any{},
	}
	s.mu.Lock()
	s.tasks[t.TaskID] = t
	s.mu.Unlock()
	select {
	case s.watch <- &taskmgr.CallTaskReq{TaskID: t.TaskID, TaskCode: taskCode}:
	default:
		return t.TaskID, ErrLocalTaskQueueFull
	}
	return t.TaskID, nil
}

// --- ITaskDataProvider ---

// WatchTask 实时派发通道。
func (s *LocalTaskStore) WatchTask() (taskmgr.TaskReceiveChan, error) {
	return s.watch, nil
}

// ListenTask 返回全部未完结任务供轮询补投（channel 溢出兜底）；
// 执行侧有进程内去重与锁，重复投递安全。
func (s *LocalTaskStore) ListenTask() ([]*taskmgr.CallTaskReq, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*taskmgr.CallTaskReq, 0, len(s.tasks))
	for _, t := range s.tasks {
		switch t.Status {
		case taskmgr.StatusWaiting, taskmgr.StatusRunning, taskmgr.StatusPending:
			out = append(out, &taskmgr.CallTaskReq{TaskID: t.TaskID, TaskCode: t.TaskCode})
		}
	}
	return out, nil
}

// --- ITaskStepInfoProvider ---

// GetTaskStepInfo 返回任务快照（含步骤进度），供 buildProcessor 构建/恢复。
func (s *LocalTaskStore) GetTaskStepInfo(taskID string) (*taskmgr.TaskData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[taskID]
	if !ok {
		return nil, ErrLocalTaskNotFound
	}
	return cloneTaskData(t), nil
}

// --- ITaskReporter ---

func (s *LocalTaskStore) ReportTaskInfo(report *taskmgr.TaskReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[report.TaskID]
	if !ok {
		return ErrLocalTaskNotFound
	}
	t.Status = report.Status
	t.RunningStepName = report.RunningStepName
	t.Message = report.Message
	if report.Payload != nil {
		t.Payload = report.Payload
	}
	if report.Result != nil {
		t.Result = report.Result
	}
	if report.Context != nil {
		t.Context = report.Context
	}
	return nil
}

func (s *LocalTaskStore) ReportStepInfo(report *taskmgr.StepReport) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.tasks[report.TaskID]
	if !ok {
		return ErrLocalTaskNotFound
	}
	var step *taskmgr.StepData
	for _, sd := range t.Steps {
		if sd != nil && sd.Name == report.StepName {
			step = sd
			break
		}
	}
	if step == nil {
		step = &taskmgr.StepData{TaskID: report.TaskID, Name: report.StepName}
		t.Steps = append(t.Steps, step)
	}
	step.Status = report.Status
	step.Message = report.Message
	if report.Payload != nil {
		step.Payload = report.Payload
	}
	if report.Result != nil {
		step.Result = report.Result
	}
	if report.PreStepResult != nil {
		step.PreStepResult = report.PreStepResult
	}
	step.PollStepInterval = report.PollStepInterval
	step.ProcessPosition = report.ProcessPosition
	if report.Error != nil {
		step.Err = report.Error
	}
	return nil
}

// --- ICanRunTask ---

// CanRunTask 本地单进程语义：任务存在且未完结即可运行。
func (s *LocalTaskStore) CanRunTask(taskID string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tasks[taskID]
	if !ok {
		return ErrLocalTaskNotFound
	}
	switch t.Status {
	case taskmgr.StatusCompleted, taskmgr.StatusFailed, taskmgr.StatusTimeout,
		taskmgr.StatusCanceled, taskmgr.StatusStopped, taskmgr.StatusRolledBack,
		taskmgr.StatusRollbackFailed:
		return errors.New("local task already finished: " + string(t.Status))
	}
	return nil
}

// cloneTaskData 浅拷贝任务与步骤，避免上报写与构建读之间的数据竞争。
func cloneTaskData(t *taskmgr.TaskData) *taskmgr.TaskData {
	cp := *t
	if t.Steps != nil {
		cp.Steps = make([]*taskmgr.StepData, len(t.Steps))
		for i, sd := range t.Steps {
			if sd == nil {
				continue
			}
			sdc := *sd
			cp.Steps[i] = &sdc
		}
	}
	return &cp
}

var (
	_ taskmgr.ITaskDataProvider     = (*LocalTaskStore)(nil)
	_ taskmgr.ITaskStepInfoProvider = (*LocalTaskStore)(nil)
	_ taskmgr.ITaskReporter         = (*LocalTaskStore)(nil)
	_ taskmgr.ICanRunTask           = (*LocalTaskStore)(nil)
)
