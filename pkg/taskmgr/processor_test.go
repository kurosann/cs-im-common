package taskmgr

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestTaskProcessor_Process_Success(t *testing.T) {
	reporter := &mockReporter{}
	stepWorker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result")),
	}

	processor := &TaskProcessor{
		TaskRegData: TaskRegInfo{
			TaskData: &TaskData{
				TaskID:   "task-001",
				TaskCode: "test_task",
				Payload:  []byte("payload"),
			},
			Steps: []StepRegInfo{
				NewStepInfo("step1", stepWorker),
				NewStepInfo("step2", stepWorker),
			},
			Context: &ContextMap{},
		},
		Reporter: reporter,
	}

	err := processor.Process(context.Background())
	if err != nil {
		t.Fatalf("Process should not return error: %v", err)
	}

	// 检查步骤是否被调用
	if !stepWorker.executeCalled {
		t.Error("Step Execute should be called")
	}

	// 检查上报
	if len(reporter.taskReports) == 0 {
		t.Error("Task reports should be sent")
	}
	if len(reporter.stepReports) == 0 {
		t.Error("Step reports should be sent")
	}

	// 检查最终状态
	lastReport := reporter.taskReports[len(reporter.taskReports)-1]
	if lastReport.Status != StatusCompleted {
		t.Errorf("Expected final status to be Completed, got %s", lastReport.Status)
	}
}

func TestTaskProcessor_Process_StepFailed(t *testing.T) {
	reporter := &mockReporter{}
	testErr := errors.New("step failed")

	stepWorker1 := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result1")),
	}
	stepWorker2 := &mockStepWorker{
		executeResult: StepExecFailed(testErr),
		rollbackErr:   nil,
	}

	processor := &TaskProcessor{
		TaskRegData: TaskRegInfo{
			TaskData: &TaskData{
				TaskID:   "task-001",
				TaskCode: "test_task",
				Payload:  []byte("payload"),
			},
			Steps: []StepRegInfo{
				NewStepInfo("step1", stepWorker1),
				NewStepInfo("step2", stepWorker2),
			},
			Context: &ContextMap{},
		},
		Reporter: reporter,
	}

	err := processor.Process(context.Background())
	if err == nil {
		t.Error("Process should return error when step fails")
	}

	// 检查回滚是否被调用
	if !stepWorker1.rollbackCalled {
		t.Error("Rollback should be called for step1")
	}
	if stepWorker2.rollbackCalled {
		t.Error("Rollback should not be called for failed step")
	}

	// 检查最终状态
	lastReport := reporter.taskReports[len(reporter.taskReports)-1]
	if lastReport.Status != StatusFailed {
		t.Errorf("Expected final status to be Failed, got %s", lastReport.Status)
	}
}

func TestTaskProcessor_Process_RevertFailedRunningStepDoesNotContinue(t *testing.T) {
	reporter := &mockReporter{}
	testErr := errors.New("step failed before restart")

	stepWorker1 := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("should not execute")),
	}
	stepWorker2 := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("should not execute")),
	}

	steps := []StepRegInfo{
		NewStepInfo("step1", stepWorker1),
		NewStepInfo("step2", stepWorker2),
	}
	steps[0].StepData.Status = StatusFailed
	steps[0].StepData.Err = testErr
	steps[0].StepData.Result = []byte("failed-result")

	processor := &TaskProcessor{
		TaskRegData: TaskRegInfo{
			TaskData: &TaskData{
				TaskID:          "task-revert-failed-001",
				TaskCode:        "test_task",
				Payload:         []byte("payload"),
				Status:          StatusRunning,
				RunningStepName: "step1",
			},
			Steps:   steps,
			Context: &ContextMap{},
		},
		Reporter: reporter,
	}

	err := processor.Process(context.Background())
	if !errors.Is(err, testErr) {
		t.Fatalf("Expected reverted failure error, got %v", err)
	}
	if stepWorker1.executeCalled {
		t.Error("Failed running step should not execute again")
	}
	if stepWorker2.executeCalled {
		t.Error("Next step should not execute after reverted failed step")
	}
	if processor.TaskRegData.TaskData.Status != StatusFailed {
		t.Errorf("Expected task status to be Failed, got %s", processor.TaskRegData.TaskData.Status)
	}
}

func TestTaskProcessor_Process_RevertFailedRunningStepRollsBackCompletedSteps(t *testing.T) {
	reporter := &mockReporter{}
	testErr := errors.New("step failed before restart")

	stepWorker1 := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("should not execute")),
	}
	stepWorker2 := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("should not execute")),
	}
	stepWorker3 := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("should not execute")),
	}

	steps := []StepRegInfo{
		NewStepInfo("step1", stepWorker1),
		NewStepInfo("step2", stepWorker2),
		NewStepInfo("step3", stepWorker3),
	}
	steps[0].StepData.Status = StatusCompleted
	steps[0].StepData.Result = []byte("result1")
	steps[1].StepData.Status = StatusFailed
	steps[1].StepData.Err = testErr
	steps[1].StepData.Result = []byte("failed-result")

	processor := &TaskProcessor{
		TaskRegData: TaskRegInfo{
			TaskData: &TaskData{
				TaskID:          "task-revert-failed-002",
				TaskCode:        "test_task",
				Payload:         []byte("payload"),
				Status:          StatusRunning,
				RunningStepName: "step2",
			},
			Steps:   steps,
			Context: &ContextMap{},
		},
		Reporter: reporter,
	}

	err := processor.Process(context.Background())
	if !errors.Is(err, testErr) {
		t.Fatalf("Expected reverted failure error, got %v", err)
	}
	if stepWorker1.executeCalled {
		t.Error("Completed step should not execute again")
	}
	if !stepWorker1.rollbackCalled {
		t.Error("Completed step should rollback after reverted failed step")
	}
	if stepWorker2.executeCalled {
		t.Error("Failed running step should not execute again")
	}
	if stepWorker2.rollbackCalled {
		t.Error("Failed running step should not rollback itself")
	}
	if stepWorker3.executeCalled {
		t.Error("Next step should not execute after reverted failed step")
	}
	if processor.TaskRegData.TaskData.Status != StatusFailed {
		t.Errorf("Expected task status to be Failed, got %s", processor.TaskRegData.TaskData.Status)
	}
}

// TestTaskProcessor_Process_CompletedWithError 测试步骤完成但有错误：不回滚，错误传递给上层
func TestTaskProcessor_Process_CompletedWithError(t *testing.T) {
	reporter := &mockReporter{}
	completedWithErr := errors.New("completed but has warnings")

	stepWorker1 := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result1")),
	}
	stepWorker2 := &mockStepWorker{
		executeResult: StepExecCompletedWithError(completedWithErr, []byte("result2")),
	}

	processor := &TaskProcessor{
		TaskRegData: TaskRegInfo{
			TaskData: &TaskData{
				TaskID:   "task-001",
				TaskCode: "test_task",
				Payload:  []byte("payload"),
			},
			Steps: []StepRegInfo{
				NewStepInfo("step1", stepWorker1),
				NewStepInfo("step2", stepWorker2),
			},
			Context: &ContextMap{},
		},
		Reporter: reporter,
	}

	err := processor.Process(context.Background())
	if err == nil {
		t.Error("Process should return error when step completes with error")
	}
	if !errors.Is(err, completedWithErr) {
		t.Errorf("Expected error to wrap completedWithErr, got %v", err)
	}

	// 完成但有错误时不应回滚
	if stepWorker1.rollbackCalled || stepWorker2.rollbackCalled {
		t.Error("Rollback should not be called when step completes with error")
	}

	// 最终状态应为 Completed
	lastReport := reporter.taskReports[len(reporter.taskReports)-1]
	if lastReport.Status != StatusCompleted {
		t.Errorf("Expected final status to be Completed, got %s", lastReport.Status)
	}
	// Reporter 应收到错误信息
	if lastReport.Error == nil {
		t.Error("Task report should include error for completed-with-error case")
	}
}

func TestTaskProcessor_Process_RollbackFailed(t *testing.T) {
	reporter := &mockReporter{}
	testErr := errors.New("step failed")
	rollbackErr := errors.New("rollback failed")

	stepWorker1 := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result1")),
		rollbackErr:   rollbackErr,
	}
	stepWorker2 := &mockStepWorker{
		executeResult: StepExecFailed(testErr),
	}

	processor := &TaskProcessor{
		TaskRegData: TaskRegInfo{
			TaskData: &TaskData{
				TaskID:   "task-001",
				TaskCode: "test_task",
				Payload:  []byte("payload"),
			},
			Steps: []StepRegInfo{
				NewStepInfo("step1", stepWorker1),
				NewStepInfo("step2", stepWorker2),
			},
			Context: &ContextMap{},
		},
		Reporter: reporter,
	}

	err := processor.Process(context.Background())
	if err == nil {
		t.Error("Process should return error")
	}

	// 检查回滚失败是否上报
	hasRollbackFailed := false
	for _, report := range reporter.stepReports {
		if report.Status == StatusRollbackFailed {
			hasRollbackFailed = true
			break
		}
	}
	if !hasRollbackFailed {
		t.Error("Rollback failed should be reported")
	}
}

func TestTaskProcessor_Process_PendingStep(t *testing.T) {
	reporter := &mockReporter{}
	stepWorker := &mockStepWorker{
		executeResult: StepExecPending([]byte("pending")),
		pollResult: StepPollStatusResult{
			Status:          StatusCompleted,
			ProcessPosition: 100,
			ProcessMessage:  "completed",
			Result:          []byte("final result"),
		},
	}

	processor := &TaskProcessor{
		TaskRegData: TaskRegInfo{
			TaskData: &TaskData{
				TaskID:   "task-001",
				TaskCode: "test_task",
				Payload:  []byte("payload"),
			},
			Steps: []StepRegInfo{
				NewStepInfo("step1", stepWorker, WithPollInterval(100*time.Millisecond)),
			},
			Context: &ContextMap{},
		},
		Reporter: reporter,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := processor.Process(ctx)
	if err != nil {
		t.Fatalf("Process should not return error: %v", err)
	}

	// 检查轮询是否被调用
	if !stepWorker.pollCalled {
		t.Error("PollStatus should be called for pending step")
	}

	// 检查最终状态
	lastReport := reporter.taskReports[len(reporter.taskReports)-1]
	if lastReport.Status != StatusCompleted {
		t.Errorf("Expected final status to be Completed, got %s", lastReport.Status)
	}
}

func TestTaskProcessor_Process_Timeout(t *testing.T) {
	reporter := &mockReporter{}
	stepWorker := &mockStepWorker{
		executeResult: StepExecPending([]byte("pending")),
		pollResult: StepPollStatusResult{
			Status: StatusPending,
		},
	}

	processor := &TaskProcessor{
		TaskRegData: TaskRegInfo{
			TaskData: &TaskData{
				TaskID:   "task-001",
				TaskCode: "test_task",
				Payload:  []byte("payload"),
			},
			Steps: []StepRegInfo{
				NewStepInfo("step1", stepWorker, WithPollInterval(100*time.Millisecond)),
			},
			Context: &ContextMap{},
		},
		Reporter: reporter,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err := processor.Process(ctx)
	if err == nil {
		t.Error("Process should return error on timeout")
	}

	// 检查最终状态
	lastReport := reporter.taskReports[len(reporter.taskReports)-1]
	if lastReport.Status != StatusTimeout {
		t.Errorf("Expected final status to be Timeout, got %s", lastReport.Status)
	}
}

func TestTaskProcessor_Process_SkipCompletedStep(t *testing.T) {
	reporter := &mockReporter{}
	stepWorker1 := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result1")),
	}
	stepWorker2 := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result2")),
	}

	steps := []StepRegInfo{
		NewStepInfo("step1", stepWorker1),
		NewStepInfo("step2", stepWorker2),
	}
	steps[0].StepData.Status = StatusCompleted // 标记为已完成

	processor := &TaskProcessor{
		TaskRegData: TaskRegInfo{
			TaskData: &TaskData{
				TaskID:   "task-001",
				TaskCode: "test_task",
				Payload:  []byte("payload"),
			},
			Steps:   steps,
			Context: &ContextMap{},
		},
		Reporter: reporter,
	}

	err := processor.Process(context.Background())
	if err != nil {
		t.Fatalf("Process should not return error: %v", err)
	}

	// step1 应该被跳过
	if stepWorker1.executeCalled {
		t.Error("Step1 should be skipped")
	}
	// step2 应该被执行
	if !stepWorker2.executeCalled {
		t.Error("Step2 should be executed")
	}
}

func TestTaskProcessor_Process_WaitStep(t *testing.T) {
	reporter := &mockReporter{}
	stepWorker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result")),
	}

	steps := []StepRegInfo{
		NewStepInfo("step1", stepWorker),
	}
	steps[0].StepData.Status = StatusWaiting // 标记为 wait 状态

	processor := &TaskProcessor{
		TaskRegData: TaskRegInfo{
			TaskData: &TaskData{
				TaskID:   "task-001",
				TaskCode: "test_task",
				Payload:  []byte("payload"),
			},
			Steps:   steps,
			Context: &ContextMap{},
		},
		Reporter: reporter,
	}

	err := processor.Process(context.Background())
	if err != nil {
		t.Fatalf("Process should not return error: %v", err)
	}

	// wait 状态的步骤应该被执行（从头开始）
	if !stepWorker.executeCalled {
		t.Error("Wait step should be executed from beginning")
	}
}

func TestTaskProcessor_Process_ContextPassing(t *testing.T) {
	reporter := &mockReporter{}
	stepWorker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result")),
	}

	taskContext := &ContextMap{}
	taskContext.ParseMap(map[string]any{
		"key1": "value1",
		"key2": 123,
	})

	processor := &TaskProcessor{
		TaskRegData: TaskRegInfo{
			TaskData: &TaskData{
				TaskID:   "task-001",
				TaskCode: "test_task",
				Payload:  []byte("payload"),
			},
			Steps: []StepRegInfo{
				NewStepInfo("step1", stepWorker),
			},
			Context: taskContext,
		},
		Reporter: reporter,
	}

	ctx := context.Background()
	err := processor.Process(ctx)
	if err != nil {
		t.Fatalf("Process should not return error: %v", err)
	}

	// 检查 Context 是否在任务上报中传递，步骤上报不携带 Context。
	for _, report := range reporter.taskReports {
		if report.Context == nil {
			t.Error("Context should be included in task report")
		}
	}
}

type mockInterruptDuringPollStepWorker struct {
	cancel         context.CancelCauseFunc
	pendingResult  []byte
	stopCalled     bool
	pauseCalled    bool
	pauseCtxActive bool
	stopCtxActive  bool
}

func (m *mockInterruptDuringPollStepWorker) Execute(ctx context.Context, input StepInput) StepExecResult {
	return StepExecPending(m.pendingResult)
}

func (m *mockInterruptDuringPollStepWorker) PollStatus(ctx context.Context, input StepPollInput) StepPollStatusResult {
	m.cancel(context.Canceled)
	return StepPollFailed(context.Canceled)
}

func (m *mockInterruptDuringPollStepWorker) Rollback(ctx context.Context, input StepRollbackInput) error {
	return nil
}

func (m *mockInterruptDuringPollStepWorker) Pause(ctx context.Context, input StepInput) error {
	m.pauseCalled = true
	m.pauseCtxActive = ctx.Err() == nil
	return nil
}

func (m *mockInterruptDuringPollStepWorker) Stop(ctx context.Context, input StepInput) error {
	m.stopCalled = true
	m.stopCtxActive = ctx.Err() == nil
	return nil
}

func TestTaskProcessor_Process_PauseDuringPollRace(t *testing.T) {
	reporter := &mockReporter{}
	ctx, cancel := context.WithCancelCause(context.Background())
	stepWorker := &mockInterruptDuringPollStepWorker{
		cancel:        cancel,
		pendingResult: []byte("pause-snapshot"),
	}

	processor := &TaskProcessor{
		TaskRegData: TaskRegInfo{
			TaskData: &TaskData{
				TaskID:   "task-pause-race-001",
				TaskCode: "test_task",
				Payload:  []byte("payload"),
			},
			Steps: []StepRegInfo{
				NewStepInfo("step1", stepWorker, WithPollInterval(10*time.Millisecond)),
			},
			Context: &ContextMap{},
		},
		Reporter: reporter,
		stopMode: StopModePause,
	}

	err := processor.Process(ctx)
	if err != nil {
		t.Fatalf("Process should not return error when paused, got %v", err)
	}

	if !stepWorker.pauseCalled {
		t.Fatal("Pause should be called when context is canceled during poll")
	}
	if !stepWorker.pauseCtxActive {
		t.Fatal("Pause should receive a non-canceled context")
	}

	lastReport := reporter.taskReports[len(reporter.taskReports)-1]
	if lastReport.Status != StatusPaused {
		t.Fatalf("Expected final status to be Paused, got %s", lastReport.Status)
	}
	if string(lastReport.Result) != "pause-snapshot" {
		t.Fatalf("Expected paused task result to be preserved, got %q", string(lastReport.Result))
	}
}

func TestTaskProcessor_Process_StopDuringPollRace(t *testing.T) {
	reporter := &mockReporter{}
	ctx, cancel := context.WithCancelCause(context.Background())
	stepWorker := &mockInterruptDuringPollStepWorker{
		cancel:        cancel,
		pendingResult: []byte("stop-snapshot"),
	}

	processor := &TaskProcessor{
		TaskRegData: TaskRegInfo{
			TaskData: &TaskData{
				TaskID:   "task-stop-race-001",
				TaskCode: "test_task",
				Payload:  []byte("payload"),
			},
			Steps: []StepRegInfo{
				NewStepInfo("step1", stepWorker, WithPollInterval(10*time.Millisecond)),
			},
			Context: &ContextMap{},
		},
		Reporter: reporter,
		stopMode: StopModeStop,
	}

	err := processor.Process(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Process should return context canceled when stopped, got %v", err)
	}

	if !stepWorker.stopCalled {
		t.Fatal("Stop should be called when context is canceled during poll")
	}
	if !stepWorker.stopCtxActive {
		t.Fatal("Stop should receive a non-canceled context")
	}

	lastReport := reporter.taskReports[len(reporter.taskReports)-1]
	if lastReport.Status != StatusStopped {
		t.Fatalf("Expected final status to be Stopped, got %s", lastReport.Status)
	}
	if string(lastReport.Result) != "stop-snapshot" {
		t.Fatalf("Expected stopped task result to be preserved, got %q", string(lastReport.Result))
	}
}

func TestTaskProcessor_ReportTaskInfo_PreservesLastResult(t *testing.T) {
	reporter := &mockReporter{}
	processor := &TaskProcessor{
		ctx: context.Background(),
		TaskRegData: TaskRegInfo{
			TaskData: &TaskData{
				TaskID:   "task-result-001",
				TaskCode: "test_task",
				Status:   StatusPaused,
				Result:   []byte("saved-result"),
			},
			Context: &ContextMap{},
		},
		Reporter:     reporter,
		stepProgress: map[string]int{},
	}

	if err := processor.reportTaskInfo(nil, nil); err != nil {
		t.Fatalf("reportTaskInfo should not return error: %v", err)
	}

	lastReport := reporter.taskReports[len(reporter.taskReports)-1]
	if string(lastReport.Result) != "saved-result" {
		t.Fatalf("Expected report to keep last result, got %q", string(lastReport.Result))
	}
}

// 集成测试：验证完整的任务执行流程
type mockIntegrationReporter struct {
	reports []*TaskReport
}

func (r *mockIntegrationReporter) ReportTaskInfo(report *TaskReport) error {
	r.reports = append(r.reports, report)
	return nil
}

func (r *mockIntegrationReporter) ReportStepInfo(report *StepReport) error {
	return nil
}

func TestTaskProcessor_Integration_FullWorkflow(t *testing.T) {
	reporter := &mockIntegrationReporter{}

	// 创建多个步骤的模拟工作者
	step1Worker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("step1_result")),
	}
	step2Worker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("step2_result")),
	}
	step3Worker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("step3_result")),
	}

	processor := &TaskProcessor{
		TaskRegData: TaskRegInfo{
			TaskData: &TaskData{
				TaskID:   "integration-task-001",
				TaskCode: "integration_test_task",
				Payload:  []byte("integration_payload"),
			},
			Steps: []StepRegInfo{
				NewStepInfo("integration_step1", step1Worker),
				NewStepInfo("integration_step2", step2Worker),
				NewStepInfo("integration_step3", step3Worker),
			},
			Context: &ContextMap{},
		},
		Reporter: reporter,
	}

	ctx := context.Background()
	err := processor.Process(ctx)
	if err != nil {
		t.Fatalf("Integration test Process should not return error: %v", err)
	}

	// 验证所有步骤都被执行
	if !step1Worker.executeCalled {
		t.Error("Integration step1 should be called")
	}
	if !step2Worker.executeCalled {
		t.Error("Integration step2 should be called")
	}
	if !step3Worker.executeCalled {
		t.Error("Integration step3 should be called")
	}

	// 验证报告生成
	if len(reporter.reports) == 0 {
		t.Error("Integration reports should be generated")
	}

	// 验证最终状态是完成
	var finalReport *TaskReport
	// 获取最后一个相关的报告
	for i := len(reporter.reports) - 1; i >= 0; i-- {
		report := reporter.reports[i]
		if report.TaskID == "integration-task-001" {
			finalReport = report
			break
		}
	}

	if finalReport.TaskID == "" {
		t.Fatal("Should find final task report")
	}

	if finalReport.Status != StatusCompleted {
		t.Errorf("Expected final status to be Completed, got %s", finalReport.Status)
	}
}

func TestTaskProcessor_Integration_ErrorHandling(t *testing.T) {
	reporter := &mockIntegrationReporter{}
	testErr := errors.New("integration test error")

	step1Worker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("step1_result")),
	}
	step2Worker := &mockStepWorker{
		executeResult: StepExecFailed(testErr),
		rollbackErr:   nil,
	}
	step3Worker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("step3_result")),
	}

	processor := &TaskProcessor{
		TaskRegData: TaskRegInfo{
			TaskData: &TaskData{
				TaskID:   "integration-error-task-001",
				TaskCode: "integration_error_test_task",
				Payload:  []byte("error_payload"),
			},
			Steps: []StepRegInfo{
				NewStepInfo("error_step1", step1Worker),
				NewStepInfo("error_step2", step2Worker),
				NewStepInfo("error_step3", step3Worker),
			},
			Context: &ContextMap{},
		},
		Reporter: reporter,
	}

	ctx := context.Background()
	err := processor.Process(ctx)
	if err == nil {
		t.Error("Integration test should return error when step fails")
	}

	// 验证错误步骤之前的步骤被回滚
	if !step1Worker.rollbackCalled {
		t.Error("Step1 should be rolled back")
	}
	if step2Worker.rollbackCalled {
		t.Error("Failed step should not be rolled back")
	}
	if step3Worker.executeCalled {
		t.Error("Step3 should not be executed")
	}

	// 验证错误报告
	var errorReport *TaskReport
	// 获取最后一个相关的报告
	for i := len(reporter.reports) - 1; i >= 0; i-- {
		report := reporter.reports[i]
		if report.TaskID == "integration-error-task-001" {
			errorReport = report
			break
		}
	}

	if errorReport.TaskID == "" {
		t.Fatal("Should find error task report")
	}

	if errorReport.Status != StatusFailed {
		t.Errorf("Expected final status to be Failed, got %s", errorReport.Status)
	}
}

// TestShouldRollback 测试回滚判断逻辑
func TestShouldRollback(t *testing.T) {
	processor := &TaskProcessor{}

	testCases := []struct {
		name           string
		status         Status
		shouldRollback bool
	}{
		{
			name:           "Completed - 不应回滚",
			status:         StatusCompleted,
			shouldRollback: false,
		},
		{
			name:           "Stopped - 不应回滚",
			status:         StatusStopped,
			shouldRollback: false,
		},
		{
			name:           "Paused - 不应回滚",
			status:         StatusPaused,
			shouldRollback: false,
		},
		{
			name:           "Timeout - 不应回滚",
			status:         StatusTimeout,
			shouldRollback: false,
		},
		{
			name:           "Failed - 应该回滚",
			status:         StatusFailed,
			shouldRollback: true,
		},
		{
			name:           "Canceled - 应该回滚",
			status:         StatusCanceled,
			shouldRollback: true,
		},
		{
			name:           "RollbackFailed - 应该回滚",
			status:         StatusRollbackFailed,
			shouldRollback: true,
		},
		{
			name:           "Running - 应该回滚",
			status:         StatusRunning,
			shouldRollback: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := processor.shouldRollback(tc.status)
			if result != tc.shouldRollback {
				t.Errorf("回滚判断不符合预期: status=%s, expected=%v, got=%v", tc.status, tc.shouldRollback, result)
			}
		})
	}
}

// TestCalculateTaskProgress 测试任务进度计算
func TestCalculateTaskProgress(t *testing.T) {
	t.Run("正常进度计算", func(t *testing.T) {
		processor := &TaskProcessor{
			TaskRegData: TaskRegInfo{
				Steps: []StepRegInfo{
					{StepData: &StepData{Name: "step1"}},
					{StepData: &StepData{Name: "step2"}},
					{StepData: &StepData{Name: "step3"}},
				},
			},
			stepProgress: map[string]int{
				"step1": 50,
				"step2": 100,
				"step3": 25,
			},
		}

		progress := processor.calculateTaskProgress()
		// 当前实现使用整数计算: (50*33/100) + (100*33/100) + (25*33/100) = 16 + 33 + 8 = 57
		expected := 57
		if progress != expected {
			t.Errorf("进度计算不正确: expected=%d, got=%d", expected, progress)
		}
	})

	t.Run("无步骤", func(t *testing.T) {
		processor := &TaskProcessor{
			TaskRegData: TaskRegInfo{
				Steps: []StepRegInfo{},
			},
			stepProgress: map[string]int{},
		}

		progress := processor.calculateTaskProgress()
		if progress != 0 {
			t.Errorf("无步骤时进度应该为0: got=%d", progress)
		}
	})

	t.Run("进度边界处理", func(t *testing.T) {
		processor := &TaskProcessor{
			TaskRegData: TaskRegInfo{
				Steps: []StepRegInfo{
					{StepData: &StepData{Name: "step1"}},
				},
			},
			stepProgress: map[string]int{
				"step1": 150, // 超出范围的进度
			},
		}

		progress := processor.calculateTaskProgress()
		// 即使单个步骤进度超过100，整体进度也不应超过100
		if progress > 100 {
			t.Errorf("整体进度不应超过100: got=%d", progress)
		}
	})
}

// TestUpdateStepProgress 测试步骤进度更新
func TestUpdateStepProgress(t *testing.T) {
	t.Run("正常进度更新", func(t *testing.T) {
		processor := &TaskProcessor{
			stepProgress: make(map[string]int),
		}

		processor.updateStepProgress("test-step", 50)

		processor.stepProgressMu.RLock()
		progress, exists := processor.stepProgress["test-step"]
		processor.stepProgressMu.RUnlock()

		if !exists {
			t.Error("步骤进度应该存在")
		}
		if progress != 50 {
			t.Errorf("进度应该被正确设置: expected=50, got=%d", progress)
		}
	})

	t.Run("进度边界处理", func(t *testing.T) {
		processor := &TaskProcessor{
			stepProgress: make(map[string]int),
		}

		// 测试负数进度
		processor.updateStepProgress("step1", -10)
		processor.stepProgressMu.RLock()
		progress1 := processor.stepProgress["step1"]
		processor.stepProgressMu.RUnlock()
		if progress1 != 0 {
			t.Errorf("负数进度应该被限制为0: got=%d", progress1)
		}

		// 测试超大进度
		processor.updateStepProgress("step2", 150)
		processor.stepProgressMu.RLock()
		progress2 := processor.stepProgress["step2"]
		processor.stepProgressMu.RUnlock()
		if progress2 != 100 {
			t.Errorf("超大进度应该被限制为100: got=%d", progress2)
		}
	})
}
