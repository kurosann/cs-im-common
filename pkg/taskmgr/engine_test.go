package taskmgr

import (
	"context"
	"errors"
	"testing"
)

// mockStepInfoProvider Mock 步骤信息提供者
type mockStepInfoProvider struct {
	taskInfo TaskData
	err      error
}

func (m *mockStepInfoProvider) GetTaskStepInfo(taskId string) (*TaskData, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &m.taskInfo, nil
}

// mockReporter Mock 上报器
type mockReporter struct {
	taskReports []TaskReport
	stepReports []StepReport
}

func (m *mockReporter) ReportTaskInfo(report *TaskReport) error {
	m.taskReports = append(m.taskReports, *report)
	return nil
}

func (m *mockReporter) ReportStepInfo(report *StepReport) error {
	m.stepReports = append(m.stepReports, *report)
	return nil
}

// mockLockProvider Mock 锁提供者
type mockLockProvider struct {
	locks map[string]bool
}

func (m *mockLockProvider) Lock(taskId string, owner string) error {
	if m.locks == nil {
		m.locks = make(map[string]bool)
	}
	if m.locks[taskId] {
		return ErrTaskLocked
	}
	m.locks[taskId] = true
	return nil
}

func (m *mockLockProvider) Unlock(taskId string, owner string) error {
	if m.locks == nil {
		return ErrTaskLockNotExists
	}
	delete(m.locks, taskId)
	return nil
}

func (m *mockLockProvider) KeepAlive(taskId string, owner string) error {
	if m.locks == nil || !m.locks[taskId] {
		return ErrTaskLockNotExists
	}
	return nil
}

// mockCanRunTask Mock 任务可运行检查
type mockCanRunTask struct{}

func (m *mockCanRunTask) CanRunTask(taskId string) error {
	return nil
}

// mockStepWorker Mock 步骤执行器
type mockStepWorker struct {
	executeResult  StepExecResult
	pollResult     StepPollStatusResult
	rollbackErr    error
	executeCalled  bool
	pollCalled     bool
	rollbackCalled bool
}

func (m *mockStepWorker) Execute(ctx context.Context, input StepInput) StepExecResult {
	m.executeCalled = true
	return m.executeResult
}

func (m *mockStepWorker) PollStatus(ctx context.Context, input StepPollInput) StepPollStatusResult {
	m.pollCalled = true
	return m.pollResult
}

func (m *mockStepWorker) Rollback(ctx context.Context, input StepRollbackInput) error {
	m.rollbackCalled = true
	return m.rollbackErr
}

// mockTaskWorker Mock 任务处理器
type mockTaskWorker struct {
	steps []StepRegInfo
}

func (m *mockTaskWorker) Arrange(ctx context.Context, payload []byte) []StepRegInfo {
	return m.steps
}

func TestNewTaskEngine(t *testing.T) {
	stepProvider := &mockStepInfoProvider{}
	reporter := &mockReporter{}
	lockProvider := &mockLockProvider{}

	engine := newTaskEngine(stepProvider, reporter, lockProvider)

	if engine == nil {
		t.Fatal("engine should not be nil")
	}
	if engine.TaskWorkers == nil {
		t.Error("TaskWorkers should be initialized")
	}
	if engine.StepInfoProvider != stepProvider {
		t.Error("StepInfoProvider should be set")
	}
	if engine.Reporter != reporter {
		t.Error("Reporter should be set")
	}
	if engine.LockProvider != lockProvider {
		t.Error("LockProvider should be set")
	}
}

func TestTaskEngine_RegisterTaskWorker(t *testing.T) {
	engine := newTaskEngine(&mockStepInfoProvider{}, &mockReporter{}, &mockLockProvider{})
	worker := &mockTaskWorker{}

	engine.RegisterTaskWorker("test_task", worker)

	if engine.TaskWorkers["test_task"] != worker {
		t.Error("TaskWorker should be registered")
	}
}

func TestTaskEngine_Build(t *testing.T) {
	engine := newTaskEngine(&mockStepInfoProvider{}, &mockReporter{}, &mockLockProvider{})

	stepWorker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result")),
	}
	steps := []StepRegInfo{
		NewStepInfo("step1", stepWorker),
		NewStepInfo("step2", stepWorker),
	}

	taskWorker := &mockTaskWorker{steps: steps}
	engine.RegisterTaskWorker("test_task", taskWorker)

	// 新任务，Steps 为空
	taskInfo := TaskData{
		TaskID:   "task-001",
		TaskCode: "test_task",
		Payload:  []byte("payload"),
		Steps:    nil, // 新任务时 Steps 为空
	}

	processor, err := engine.Build(context.Background(), &taskInfo)
	if err != nil {
		t.Fatalf("Build should not return error: %v", err)
	}

	if processor == nil {
		t.Fatal("Processor should not be nil")
	}
	if len(processor.TaskRegData.Steps) != 2 {
		t.Errorf("Expected 2 steps, got %d", len(processor.TaskRegData.Steps))
	}
	if processor.TaskRegData.Context == nil {
		t.Error("Context should be initialized")
	}
}

func TestTaskEngine_RevertRestoresStepRuntimeFields(t *testing.T) {
	stepWorker := &mockStepWorker{}
	stepErr := errors.New("step failed")
	worker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("step1", stepWorker),
		},
	}

	engine := newTaskEngine(&mockStepInfoProvider{}, &mockReporter{}, &mockLockProvider{})
	if err := engine.RegisterTaskWorker("test_task", worker); err != nil {
		t.Fatalf("RegisterTaskWorker failed: %v", err)
	}

	processor, err := engine.Revert(context.Background(), &TaskData{
		TaskID:   "task-001",
		TaskCode: "test_task",
		Steps: []*StepData{
			{
				TaskID:          "task-001",
				Name:            "step1",
				Status:          StatusFailed,
				Err:             stepErr,
				Message:         "failed message",
				Payload:         []byte("step-payload"),
				PreStepResult:   []byte("pre-result"),
				Result:          []byte("step-result"),
				ProcessPosition: 42,
			},
		},
	})
	if err != nil {
		t.Fatalf("Revert failed: %v", err)
	}

	restored := processor.TaskRegData.Steps[0].StepData
	if restored.Err != stepErr {
		t.Fatalf("Expected reverted step error to be preserved, got %v", restored.Err)
	}
	if restored.Message != "failed message" {
		t.Errorf("Expected message to be preserved, got %q", restored.Message)
	}
	if string(restored.Payload) != "step-payload" {
		t.Errorf("Expected payload to be preserved, got %q", string(restored.Payload))
	}
	if string(restored.PreStepResult) != "pre-result" {
		t.Errorf("Expected pre step result to be preserved, got %q", string(restored.PreStepResult))
	}
	if string(restored.Result) != "step-result" {
		t.Errorf("Expected result to be preserved, got %q", string(restored.Result))
	}
	if restored.ProcessPosition != 42 {
		t.Errorf("Expected process position to be preserved, got %d", restored.ProcessPosition)
	}
}

func TestTaskEngine_Build_WorkerNotFound(t *testing.T) {
	engine := newTaskEngine(&mockStepInfoProvider{}, &mockReporter{}, &mockLockProvider{})

	taskInfo := TaskData{
		TaskID:   "task-001",
		TaskCode: "unknown_task",
		Payload:  []byte("payload"),
	}

	_, err := engine.Build(context.Background(), &taskInfo)
	if err != ErrTaskWorkerNotFound {
		t.Errorf("Expected ErrTaskWorkerNotFound, got %v", err)
	}
}

func TestTaskEngine_Revert(t *testing.T) {
	stepWorker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result")),
	}
	steps := []StepRegInfo{
		NewStepInfo("step1", stepWorker),
		NewStepInfo("step2", stepWorker),
	}

	taskWorker := &mockTaskWorker{steps: steps}

	stepProvider := &mockStepInfoProvider{
		taskInfo: TaskData{
			TaskID:   "task-001",
			TaskCode: "test_task",
			Status:   StatusRunning,
			Steps: []*StepData{
				{TaskID: "task-001", Name: "step1", Status: StatusCompleted},
				{TaskID: "task-001", Name: "step2", Status: StatusPending},
			},
			Context: map[string]any{"key": "value"},
		},
	}

	engine := newTaskEngine(stepProvider, &mockReporter{}, &mockLockProvider{})
	engine.RegisterTaskWorker("test_task", taskWorker)

	taskInfo := TaskData{
		TaskID:   "task-001",
		TaskCode: "test_task",
		Status:   StatusRunning,
		Steps: []*StepData{
			{TaskID: "task-001", Name: "step1", Status: StatusCompleted},
			{TaskID: "task-001", Name: "step2", Status: StatusPending},
		},
		Context: map[string]any{"key": "value"},
	}

	processor, err := engine.Revert(context.Background(), &taskInfo)
	if err != nil {
		t.Fatalf("Revert should not return error: %v", err)
	}

	if processor == nil {
		t.Fatal("Processor should not be nil")
	}
	if len(processor.TaskRegData.Steps) != 2 {
		t.Errorf("Expected 2 steps, got %d", len(processor.TaskRegData.Steps))
	}
	// 检查状态是否恢复
	if processor.TaskRegData.Steps[0].StepData.Status != StatusCompleted {
		t.Errorf("Expected step1 status to be Completed, got %s", processor.TaskRegData.Steps[0].StepData.Status)
	}
	if processor.TaskRegData.Steps[1].StepData.Status != StatusPending {
		t.Errorf("Expected step2 status to be Pending, got %s", processor.TaskRegData.Steps[1].StepData.Status)
	}
	// 检查 Context 是否恢复
	if value, ok := ContextGet[string](processor.TaskRegData.Context, "key"); !ok || value != "value" {
		t.Errorf("Expected value to be 'value', got %s", value)
	}
}

func TestTaskEngine_Revert_WorkerNotFound(t *testing.T) {
	stepProvider := &mockStepInfoProvider{
		taskInfo: TaskData{
			TaskID:   "task-001",
			TaskCode: "unknown_task",
			Status:   StatusRunning,
		},
	}

	engine := newTaskEngine(stepProvider, &mockReporter{}, &mockLockProvider{})

	taskInfo := TaskData{
		TaskID:   "task-001",
		TaskCode: "unknown_task",
		Status:   StatusRunning,
	}

	_, err := engine.Revert(context.Background(), &taskInfo)
	if err != ErrTaskWorkerNotFound {
		t.Errorf("Expected ErrTaskWorkerNotFound, got %v", err)
	}
}

func TestIsFinishedStatus(t *testing.T) {
	tests := []struct {
		name     string
		status   Status
		expected bool
	}{
		{"Completed", StatusCompleted, true},
		{"RolledBack", StatusRolledBack, true},
		{"Failed", StatusFailed, true},
		{"Canceled", StatusCanceled, true},
		{"Running", StatusRunning, false},
		{"Pending", StatusPending, false},
		{"RollingBack", StatusRollingBack, false},
		{"Waiting", StatusWaiting, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isFinishedStatus(tt.status)
			if result != tt.expected {
				t.Errorf("isFinishedStatus(%s) = %v, expected %v", tt.status, result, tt.expected)
			}
		})
	}
}
