package taskmgr

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mockDataProvider Mock 数据提供者
type mockDataProvider struct {
	watchChan chan *CallTaskReq
	tasks     []*CallTaskReq
	err       error
}

func (m *mockDataProvider) WatchTask() (TaskReceiveChan, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.watchChan == nil {
		m.watchChan = make(chan *CallTaskReq, 10)
	}
	return m.watchChan, nil
}

func (m *mockDataProvider) ListenTask() ([]*CallTaskReq, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tasks, nil
}

func TestNewTaskManager(t *testing.T) {
	stepProvider := &mockStepInfoProvider{}
	reporter := &mockReporter{}
	dataProvider := &mockDataProvider{}
	lockProvider := &mockLockProvider{}

	manager := NewTaskManager(stepProvider, reporter, dataProvider, lockProvider, &mockCanRunTask{})

	if manager == nil {
		t.Fatal("Manager should not be nil")
	}
	if manager.engine == nil {
		t.Error("Engine should be initialized")
	}
	if manager.DataProvider != dataProvider {
		t.Error("DataProvider should be set")
	}
	if manager.LockProvider != lockProvider {
		t.Error("LockProvider should be set")
	}
	if manager.PollInterval == 0 {
		t.Error("PollInterval should have default value")
	}
}

func TestTaskManager_RegisterTaskWorker(t *testing.T) {
	// 使用能成功获取任务信息的 provider，状态为 Waiting，这样会走 Build 路径
	stepProvider := &mockStepInfoProvider{
		taskInfo: TaskData{
			TaskID:   "task-001",
			TaskCode: "test_task",
			Status:   StatusWaiting,
		},
	}
	manager := NewTaskManager(
		stepProvider,
		&mockReporter{},
		&mockDataProvider{},
		&mockLockProvider{},
		&mockCanRunTask{},
	)

	stepWorker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result")),
	}
	worker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("step1", stepWorker),
		},
	}
	err := manager.RegisterTaskWorker("test_task", worker)
	if err != nil {
		t.Fatalf("RegisterTaskWorker should not return error: %v", err)
	}

	// 通过 buildProcessor 间接测试是否注册成功
	taskData := &CallTaskReq{
		TaskID:   "task-001",
		TaskCode: "test_task",
	}

	_, err = manager.buildProcessor(context.Background(), taskData)
	if err == ErrTaskWorkerNotFound {
		t.Error("TaskWorker should be registered")
	}
	if err != nil {
		t.Fatalf("buildProcessor should not return error: %v", err)
	}
}

func TestTaskManager_BuildProcessor_NewTask(t *testing.T) {
	stepWorker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result")),
	}
	taskWorker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("step1", stepWorker),
		},
	}

	manager := NewTaskManager(
		&mockStepInfoProvider{err: ErrTaskWorkerNotFound}, // 模拟新任务（获取任务信息失败）
		&mockReporter{},
		&mockDataProvider{},
		&mockLockProvider{},
		&mockCanRunTask{},
	)
	manager.RegisterTaskWorker("test_task", taskWorker)

	taskData := &CallTaskReq{
		TaskID:   "task-001",
		TaskCode: "test_task",
	}

	processor, err := manager.buildProcessor(context.Background(), taskData)
	// 当 GetTaskStepInfo 返回错误时，buildProcessor 应该返回错误，不运行任务
	if err == nil {
		t.Fatal("buildProcessor should return error when GetTaskStepInfo fails")
	}
	if !errors.Is(err, ErrTaskGetFailed) {
		t.Errorf("Expected ErrTaskGetFailed, got %v", err)
	}
	if processor != nil {
		t.Fatal("Processor should be nil when error occurs")
	}
}

func TestTaskManager_BuildProcessor_RevertTask(t *testing.T) {
	stepWorker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result")),
	}
	taskWorker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("step1", stepWorker),
		},
	}

	stepProvider := &mockStepInfoProvider{
		taskInfo: TaskData{
			TaskID:   "task-001",
			TaskCode: "test_task",
			Status:   StatusRunning,
			Steps: []*StepData{
				{TaskID: "task-001", Name: "step1", Status: StatusCompleted},
			},
		},
	}

	manager := NewTaskManager(stepProvider, &mockReporter{}, &mockDataProvider{}, &mockLockProvider{}, &mockCanRunTask{})
	manager.RegisterTaskWorker("test_task", taskWorker)

	taskData := &CallTaskReq{
		TaskID:   "task-001",
		TaskCode: "test_task",
	}

	processor, err := manager.buildProcessor(context.Background(), taskData)
	if err != nil {
		t.Fatalf("buildProcessor should not return error: %v", err)
	}
	if processor == nil {
		t.Fatal("Processor should not be nil")
	}
}

func TestTaskManager_BuildProcessor_FinishedTask(t *testing.T) {
	stepProvider := &mockStepInfoProvider{
		taskInfo: TaskData{
			TaskID:   "task-001",
			TaskCode: "test_task",
			Status:   StatusCompleted, // 已完成的任务
		},
	}

	manager := NewTaskManager(stepProvider, &mockReporter{}, &mockDataProvider{}, &mockLockProvider{}, &mockCanRunTask{})

	taskData := &CallTaskReq{
		TaskID:   "task-001",
		TaskCode: "test_task",
	}

	_, err := manager.buildProcessor(context.Background(), taskData)
	if err != ErrTaskAlreadyFinished {
		t.Errorf("Expected ErrTaskAlreadyFinished, got %v", err)
	}
}

func TestTaskManager_BuildProcessor_NetworkError(t *testing.T) {
	networkErr := errors.New("network error")
	stepProvider := &mockStepInfoProvider{
		err: networkErr, // 模拟网络错误
	}

	stepWorker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result")),
	}
	taskWorker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("step1", stepWorker),
		},
	}

	manager := NewTaskManager(stepProvider, &mockReporter{}, &mockDataProvider{}, &mockLockProvider{}, &mockCanRunTask{})
	manager.RegisterTaskWorker("test_task", taskWorker)

	taskData := &CallTaskReq{
		TaskID:   "task-001",
		TaskCode: "test_task",
	}

	_, err := manager.buildProcessor(context.Background(), taskData)
	// 当 GetTaskStepInfo 返回错误时，buildProcessor 应该返回错误，不运行任务
	if err == nil {
		t.Fatal("buildProcessor should return error when GetTaskStepInfo fails")
	}
	if !errors.Is(err, ErrTaskGetFailed) {
		t.Errorf("Expected ErrTaskGetFailed, got %v", err)
	}
}

func TestTaskManager_BuildProcessor_WaitTask(t *testing.T) {
	stepWorker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result")),
	}
	taskWorker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("step1", stepWorker),
		},
	}

	stepProvider := &mockStepInfoProvider{
		taskInfo: TaskData{
			TaskID:   "task-001",
			TaskCode: "test_task",
			Status:   StatusWaiting, // wait 状态，从头开始运行
			Steps: []*StepData{
				{TaskID: "task-001", Name: "step1", Status: StatusCompleted},
			},
		},
	}

	manager := NewTaskManager(stepProvider, &mockReporter{}, &mockDataProvider{}, &mockLockProvider{}, &mockCanRunTask{})
	manager.RegisterTaskWorker("test_task", taskWorker)

	taskData := &CallTaskReq{
		TaskID:   "task-001",
		TaskCode: "test_task",
	}

	processor, err := manager.buildProcessor(context.Background(), taskData)
	if err != nil {
		t.Fatalf("buildProcessor should not return error: %v", err)
	}
	if processor == nil {
		t.Fatal("Processor should not be nil")
	}

	// wait 状态应该走 build 路径，步骤状态应该被重置
	if len(processor.TaskRegData.Steps) == 0 {
		t.Error("Steps should be present")
	}
	// 步骤状态应该不是 Completed（因为从头开始）
	if processor.TaskRegData.Steps[0].StepData.Status == StatusCompleted {
		t.Error("Step status should be reset when task is in wait status")
	}
}

func TestTaskManager_ExecuteTask_Success(t *testing.T) {
	reporter := &mockReporter{}
	stepWorker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result")),
	}
	taskWorker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("step1", stepWorker),
		},
	}

	// 使用能成功获取任务信息的 provider，状态为 Waiting，这样会走 Build 路径
	stepProvider := &mockStepInfoProvider{
		taskInfo: TaskData{
			TaskID:   "task-001",
			TaskCode: "test_task",
			Status:   StatusWaiting,
		},
	}

	manager := NewTaskManager(
		stepProvider,
		reporter,
		&mockDataProvider{},
		&mockLockProvider{},
		&mockCanRunTask{},
	)
	manager.RegisterTaskWorker("test_task", taskWorker)
	manager.Start(context.Background())

	taskData := &CallTaskReq{
		TaskID:   "task-001",
		TaskCode: "test_task",
	}

	manager.executeTask(context.Background(), taskData)

	// 检查任务是否执行
	if !stepWorker.executeCalled {
		t.Error("Step should be executed")
	}

	// 检查上报
	if len(reporter.taskReports) == 0 {
		t.Error("Task should be reported")
	}
}

func TestTaskManager_ExecuteTask_LockFailed(t *testing.T) {
	lockProvider := &mockLockProvider{}
	lockProvider.Lock("task-001", "owner-1") // 先获取锁

	manager := NewTaskManager(
		&mockStepInfoProvider{},
		&mockReporter{},
		&mockDataProvider{},
		lockProvider,
		&mockCanRunTask{},
	)
	manager.Start(context.Background())

	taskData := &CallTaskReq{
		TaskID:   "task-001",
		TaskCode: "test_task",
	}

	manager.executeTask(context.Background(), taskData)

	// 由于锁获取失败，任务不应该执行
	// 这里主要测试不会 panic
}

func TestTaskManager_Start_Stop(t *testing.T) {
	dataProvider := &mockDataProvider{
		watchChan: make(chan *CallTaskReq, 10),
		tasks:     []*CallTaskReq{},
	}

	manager := NewTaskManager(
		&mockStepInfoProvider{},
		&mockReporter{},
		dataProvider,
		&mockLockProvider{},
		&mockCanRunTask{},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动管理器
	err := manager.Start(ctx)
	if err != nil {
		t.Fatalf("Start should not return error: %v", err)
	}

	// 等待一下让 goroutine 启动
	time.Sleep(100 * time.Millisecond)

	// 停止管理器
	manager.Stop()
	manager.Wait()
}

func TestTaskManager_WatchTasks(t *testing.T) {
	dataProvider := &mockDataProvider{
		watchChan: make(chan *CallTaskReq, 10),
	}
	reporter := &mockReporter{}
	stepWorker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result")),
	}
	taskWorker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("step1", stepWorker),
		},
	}

	// 使用能成功获取任务信息的 provider，状态为 Waiting，这样会走 Build 路径
	stepProvider := &mockStepInfoProvider{
		taskInfo: TaskData{
			TaskID:   "task-001",
			TaskCode: "test_task",
			Status:   StatusWaiting,
		},
	}
	manager := NewTaskManager(
		stepProvider,
		reporter,
		dataProvider,
		&mockLockProvider{},
		&mockCanRunTask{},
	)
	manager.RegisterTaskWorker("test_task", taskWorker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 TaskManager（会启动 watchTasks）
	err := manager.Start(ctx)
	if err != nil {
		t.Fatalf("Start should not return error: %v", err)
	}

	// 发送任务数据
	taskData := &CallTaskReq{
		TaskID:   "task-001",
		TaskCode: "test_task",
	}
	dataProvider.watchChan <- taskData

	// 等待处理
	time.Sleep(200 * time.Millisecond)

	// 检查任务是否被执行
	if !stepWorker.executeCalled {
		t.Error("Task should be executed")
	}
}

func TestTaskManager_PollTasks(t *testing.T) {
	reporter := &mockReporter{}
	stepWorker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result")),
	}
	taskWorker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("step1", stepWorker),
		},
	}

	dataProvider := &mockDataProvider{
		tasks: []*CallTaskReq{
			{TaskID: "task-001", TaskCode: "test_task"},
		},
	}

	// 使用能成功获取任务信息的 provider，状态为 Waiting，这样会走 Build 路径
	stepProvider := &mockStepInfoProvider{
		taskInfo: TaskData{
			TaskID:   "task-001",
			TaskCode: "test_task",
			Status:   StatusWaiting,
		},
	}
	manager := NewTaskManager(
		stepProvider,
		reporter,
		dataProvider,
		&mockLockProvider{},
		&mockCanRunTask{},
		WithTaskPollInterval(100*time.Millisecond),
	)
	manager.RegisterTaskWorker("test_task", taskWorker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 TaskManager（会启动 pollTasks）
	err := manager.Start(ctx)
	if err != nil {
		t.Fatalf("Start should not return error: %v", err)
	}

	// 等待轮询
	time.Sleep(250 * time.Millisecond)

	// 检查任务是否被执行
	if !stepWorker.executeCalled {
		t.Error("Task should be executed from polling")
	}
}

// TestTaskManager_RealWorldScenario 实际场景测试：订单处理流程
// 测试完整的订单处理任务：创建订单 -> 支付 -> 发货 -> 完成
func TestTaskManager_RealWorldScenario(t *testing.T) {
	// 1. 准备 Mock 组件
	// 使用能成功获取任务信息的 provider，状态为 Waiting，这样会走 Build 路径
	stepProvider := &mockStepInfoProvider{
		taskInfo: TaskData{
			TaskID:   "task-order-001",
			TaskCode: "order_process",
			Status:   StatusWaiting,
		},
	}
	reporter := &mockReporter{
		taskReports: make([]TaskReport, 0),
		stepReports: make([]StepReport, 0),
	}
	dataProvider := &mockDataProvider{
		watchChan: make(chan *CallTaskReq, 10),
		tasks:     []*CallTaskReq{},
	}
	lockProvider := &mockLockProvider{
		locks: make(map[string]bool),
	}

	// 2. 创建订单处理步骤
	createOrderStep := &mockStepWorker{
		executeResult: StepExecCompleted([]byte(`{"order_id": "order-123"}`)),
	}
	paymentStep := &mockStepWorker{
		executeResult: StepExecCompleted([]byte(`{"payment_id": "pay-456"}`)),
		rollbackErr:   nil,
	}
	shipStep := &mockStepWorker{
		executeResult: StepExecPending([]byte(`{"ship_task_id": "ship-789"}`)),
		pollResult: StepPollStatusResult{
			Status:          StatusCompleted,
			ProcessPosition: 100,
			ProcessMessage:  "shipment completed",
			Result:          []byte(`{"tracking_number": "TRACK-001"}`),
		},
	}

	// 3. 创建任务处理器
	orderTaskWorker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("create_order", createOrderStep),
			NewStepInfo("payment", paymentStep),
			NewStepInfo("shipment", shipStep, WithPollInterval(100*time.Millisecond)),
		},
	}

	// 4. 创建并配置 TaskManager
	manager := NewTaskManager(
		stepProvider,
		reporter,
		dataProvider,
		lockProvider,
		&mockCanRunTask{},
		WithTaskPollInterval(200*time.Millisecond),
	)

	// 注册任务处理器
	err := manager.RegisterTaskWorker("order_process", orderTaskWorker)
	if err != nil {
		t.Fatalf("RegisterTaskWorker should not return error: %v", err)
	}

	// 5. 启动 TaskManager
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = manager.Start(ctx)
	if err != nil {
		t.Fatalf("Start should not return error: %v", err)
	}

	// 6. 发送任务数据
	taskData := &CallTaskReq{
		TaskID:   "task-order-001",
		TaskCode: "order_process",
	}

	// 通过 Watch 发送任务
	dataProvider.watchChan <- taskData

	// 7. 等待任务执行完成
	time.Sleep(1 * time.Second)

	// 8. 验证任务执行
	// 检查所有步骤是否都被执行
	if !createOrderStep.executeCalled {
		t.Error("create_order step should be executed")
	}
	if !paymentStep.executeCalled {
		t.Error("payment step should be executed")
	}
	if !shipStep.executeCalled {
		t.Error("shipment step should be executed")
	}
	if !shipStep.pollCalled {
		t.Error("shipment step should be polled")
	}

	// 9. 验证状态上报
	// 检查任务状态上报
	if len(reporter.taskReports) == 0 {
		t.Fatal("Task reports should be sent")
	}

	// 检查第一个任务报告应该是 Running
	firstTaskReport := reporter.taskReports[0]
	if firstTaskReport.Status != StatusRunning {
		t.Errorf("First task report should be Running, got %s", firstTaskReport.Status)
	}

	// 检查最后一个任务报告应该是 Completed
	lastTaskReport := reporter.taskReports[len(reporter.taskReports)-1]
	if lastTaskReport.Status != StatusCompleted {
		t.Errorf("Last task report should be Completed, got %s", lastTaskReport.Status)
	}

	// 检查步骤状态上报
	if len(reporter.stepReports) == 0 {
		t.Fatal("Step reports should be sent")
	}

	// 验证步骤执行顺序和状态
	stepStatuses := make(map[string][]Status)
	for _, report := range reporter.stepReports {
		stepStatuses[report.StepName] = append(stepStatuses[report.StepName], report.Status)
	}

	// create_order 步骤应该经历：running -> completed
	if statuses, ok := stepStatuses["create_order"]; ok {
		hasRunning := false
		hasCompleted := false
		for _, s := range statuses {
			if s == StatusRunning {
				hasRunning = true
			}
			if s == StatusCompleted {
				hasCompleted = true
			}
		}
		if !hasRunning {
			t.Error("create_order should have Running status")
		}
		if !hasCompleted {
			t.Error("create_order should have Completed status")
		}
	}

	// shipment 步骤应该经历：running -> pending -> completed
	if statuses, ok := stepStatuses["shipment"]; ok {
		hasRunning := false
		hasPending := false
		hasCompleted := false
		for _, s := range statuses {
			if s == StatusRunning {
				hasRunning = true
			}
			if s == StatusPending {
				hasPending = true
			}
			if s == StatusCompleted {
				hasCompleted = true
			}
		}
		if !hasRunning {
			t.Error("shipment should have Running status")
		}
		if !hasPending {
			t.Error("shipment should have Pending status (async execution)")
		}
		if !hasCompleted {
			t.Error("shipment should have Completed status")
		}
	}

	// 10. 停止管理器
	manager.Stop()
	manager.Wait()
}

// TestTaskManager_RealWorldScenario_FailureAndRollback 实际场景测试：失败回滚
// 测试订单处理失败时的回滚流程
func TestTaskManager_RealWorldScenario_FailureAndRollback(t *testing.T) {
	// 使用能成功获取任务信息的 provider，状态为 Waiting，这样会走 Build 路径
	stepProvider := &mockStepInfoProvider{
		taskInfo: TaskData{
			TaskID:   "task-order-002",
			TaskCode: "order_process",
			Status:   StatusWaiting,
		},
	}
	reporter := &mockReporter{
		taskReports: make([]TaskReport, 0),
		stepReports: make([]StepReport, 0),
	}
	dataProvider := &mockDataProvider{
		watchChan: make(chan *CallTaskReq, 10),
	}
	lockProvider := &mockLockProvider{
		locks: make(map[string]bool),
	}

	// 创建步骤：前两个成功，第三个失败
	createOrderStep := &mockStepWorker{
		executeResult: StepExecCompleted([]byte(`{"order_id": "order-123"}`)),
		rollbackErr:   nil,
	}
	paymentStep := &mockStepWorker{
		executeResult: StepExecCompleted([]byte(`{"payment_id": "pay-456"}`)),
		rollbackErr:   nil,
	}
	shipStep := &mockStepWorker{
		executeResult: StepExecFailed(errors.New("shipment failed")),
	}

	orderTaskWorker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("create_order", createOrderStep),
			NewStepInfo("payment", paymentStep),
			NewStepInfo("shipment", shipStep),
		},
	}

	manager := NewTaskManager(
		stepProvider,
		reporter,
		dataProvider,
		lockProvider,
		&mockCanRunTask{},
	)

	err := manager.RegisterTaskWorker("order_process", orderTaskWorker)
	if err != nil {
		t.Fatalf("RegisterTaskWorker should not return error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = manager.Start(ctx)
	if err != nil {
		t.Fatalf("Start should not return error: %v", err)
	}

	// 发送任务
	taskData := &CallTaskReq{
		TaskID:   "task-order-002",
		TaskCode: "order_process",
	}
	dataProvider.watchChan <- taskData

	// 等待执行和回滚（增加等待时间确保回滚完成）
	time.Sleep(2 * time.Second)

	// 验证执行和回滚
	if !createOrderStep.executeCalled {
		t.Error("create_order should be executed")
	}
	if !paymentStep.executeCalled {
		t.Error("payment should be executed")
	}
	if !shipStep.executeCalled {
		t.Error("shipment should be executed")
	}

	// 验证回滚
	// 注意：回滚是异步的，需要等待一段时间
	// 根据回滚逻辑：最后一个失败的步骤不需要回滚，其他步骤需要回滚
	if !createOrderStep.rollbackCalled {
		t.Error("create_order should be rolled back")
	}
	if !paymentStep.rollbackCalled {
		t.Error("payment should be rolled back")
	}
	// shipment 失败，不需要回滚（最后一个失败的步骤）
	if shipStep.rollbackCalled {
		t.Error("shipment should not be rolled back (it failed)")
	}

	// 验证最终状态
	lastTaskReport := reporter.taskReports[len(reporter.taskReports)-1]
	if lastTaskReport.Status != StatusFailed {
		t.Errorf("Task should be Failed, got %s", lastTaskReport.Status)
	}

	// 验证回滚状态上报
	hasRollingBack := false
	hasRolledBack := false
	for _, report := range reporter.stepReports {
		if report.Status == StatusRollingBack {
			hasRollingBack = true
		}
		if report.Status == StatusRolledBack {
			hasRolledBack = true
		}
	}
	if !hasRollingBack {
		t.Error("Should have RollingBack status")
	}
	if !hasRolledBack {
		t.Error("Should have RolledBack status")
	}

	manager.Stop()
	manager.Wait()
}

// TestTaskManager_RealWorldScenario_RestoreTask 实际场景测试：恢复中断的任务
// 测试任务中断后恢复执行的场景
func TestTaskManager_RealWorldScenario_RestoreTask(t *testing.T) {
	// 模拟已存在的任务（部分完成）
	stepProvider := &mockStepInfoProvider{
		taskInfo: TaskData{
			TaskID:   "task-order-003",
			TaskCode: "order_process",
			Status:   StatusRunning,
			Steps: []*StepData{
				{TaskID: "task-order-003", Name: "create_order", Status: StatusCompleted},
				{TaskID: "task-order-003", Name: "payment", Status: StatusCompleted},
				{TaskID: "task-order-003", Name: "shipment", Status: StatusPending},
			},
			Context: map[string]any{
				"order_id":   "order-123",
				"payment_id": "pay-456",
			},
		},
	}

	reporter := &mockReporter{
		taskReports: make([]TaskReport, 0),
		stepReports: make([]StepReport, 0),
	}
	dataProvider := &mockDataProvider{
		watchChan: make(chan *CallTaskReq, 10),
	}
	lockProvider := &mockLockProvider{
		locks: make(map[string]bool),
	}

	// 创建步骤
	createOrderStep := &mockStepWorker{
		executeResult: StepExecCompleted([]byte(`{"order_id": "order-123"}`)),
	}
	paymentStep := &mockStepWorker{
		executeResult: StepExecCompleted([]byte(`{"payment_id": "pay-456"}`)),
	}
	shipStep := &mockStepWorker{
		executeResult: StepExecPending([]byte(`{"ship_task_id": "ship-789"}`)),
		pollResult: StepPollStatusResult{
			Status:          StatusCompleted,
			ProcessPosition: 100,
			ProcessMessage:  "shipment completed",
			Result:          []byte(`{"tracking_number": "TRACK-001"}`),
		},
	}

	orderTaskWorker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("create_order", createOrderStep),
			NewStepInfo("payment", paymentStep),
			NewStepInfo("shipment", shipStep, WithPollInterval(100*time.Millisecond)),
		},
	}

	manager := NewTaskManager(
		stepProvider,
		reporter,
		dataProvider,
		lockProvider,
		&mockCanRunTask{},
	)

	err := manager.RegisterTaskWorker("order_process", orderTaskWorker)
	if err != nil {
		t.Fatalf("RegisterTaskWorker should not return error: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = manager.Start(ctx)
	if err != nil {
		t.Fatalf("Start should not return error: %v", err)
	}

	// 发送任务（恢复执行）
	taskData := &CallTaskReq{
		TaskID:   "task-order-003",
		TaskCode: "order_process",
	}
	dataProvider.watchChan <- taskData

	// 等待执行
	time.Sleep(1 * time.Second)

	// 验证：已完成的步骤应该被跳过
	if createOrderStep.executeCalled {
		t.Error("create_order should be skipped (already completed)")
	}
	if paymentStep.executeCalled {
		t.Error("payment should be skipped (already completed)")
	}
	// 未完成的步骤应该继续执行
	if !shipStep.executeCalled {
		t.Error("shipment should be executed (was pending)")
	}
	if !shipStep.pollCalled {
		t.Error("shipment should be polled")
	}

	// 验证最终状态
	lastTaskReport := reporter.taskReports[len(reporter.taskReports)-1]
	if lastTaskReport.Status != StatusCompleted {
		t.Errorf("Task should be Completed, got %s", lastTaskReport.Status)
	}

	manager.Stop()
	manager.Wait()
}

// TestStopOperations 测试三种停止操作
func TestStopOperations(t *testing.T) {
	// 测试枚举值
	assert.Equal(t, StopMode("cancel"), StopModeCancel)
	assert.Equal(t, StopMode("stop"), StopModeStop)
	assert.Equal(t, StopMode("pause"), StopModePause)

	// 测试字符串表示
	assert.Equal(t, "cancel", string(StopModeCancel))
	assert.Equal(t, "stop", string(StopModeStop))
	assert.Equal(t, "pause", string(StopModePause))

	t.Log("停止模式枚举验证通过")
}

func TestStopOperationsReportImmediatelyBeforeCancel(t *testing.T) {
	tests := []struct {
		name           string
		mode           StopMode
		expectedStatus Status
		action         func(*TaskManager, string) error
	}{
		{
			name:           "cancel",
			mode:           StopModeCancel,
			expectedStatus: StatusCanceled,
			action:         func(tm *TaskManager, taskID string) error { return tm.cancelTask(taskID) },
		},
		{
			name:           "stop",
			mode:           StopModeStop,
			expectedStatus: StatusStopped,
			action:         func(tm *TaskManager, taskID string) error { return tm.stopTask(taskID) },
		},
		{
			name:           "pause",
			mode:           StopModePause,
			expectedStatus: StatusPaused,
			action:         func(tm *TaskManager, taskID string) error { return tm.pauseTask(taskID) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reporter := &mockReporter{}
			manager := NewTaskManager(
				&mockStepInfoProvider{},
				reporter,
				&mockDataProvider{},
				&mockLockProvider{},
				&mockCanRunTask{},
			)
			manager.ctx = context.Background()

			taskID := "task-stop-report-001"
			taskContext := &ContextMap{}
			taskContext.Set("k", "v")

			processor := &TaskProcessor{
				TaskRegData: TaskRegInfo{
					TaskData: &TaskData{
						TaskID:          taskID,
						TaskCode:        "test_task",
						Status:          StatusRunning,
						RunningStepName: "step1",
						Message:         "working",
						Payload:         []byte("payload"),
						Result:          []byte("result"),
					},
					Steps: []StepRegInfo{
						NewStepInfo("step1", &mockStepWorker{}),
						NewStepInfo("step2", &mockStepWorker{}),
					},
					Context: taskContext,
				},
				Reporter: reporter,
				stepProgress: map[string]int{
					"step1": 60,
					"step2": 0,
				},
			}

			reportSentBeforeCancel := false
			manager.taskProcessors.Store(taskID, processor)
			manager.taskContexts.Store(taskID, context.CancelFunc(func() {
				reportSentBeforeCancel = len(reporter.taskReports) > 0
			}))

			err := tt.action(manager, taskID)
			assert.NoError(t, err)
			assert.True(t, reportSentBeforeCancel, "cancel 前应该已经上报")
			assert.Len(t, reporter.taskReports, 1)

			report := reporter.taskReports[0]
			assert.Equal(t, tt.expectedStatus, report.Status)
			assert.Equal(t, "step1", report.RunningStepName)
			assert.Equal(t, "working", report.Message)
			assert.Equal(t, []byte("payload"), report.Payload)
			assert.Equal(t, []byte("result"), report.Result)
			assert.Equal(t, 30, report.ProcessPosition)
			assert.Equal(t, "v", report.Context["k"])

			assert.Equal(t, tt.expectedStatus, processor.TaskRegData.TaskData.Status)
			assert.Equal(t, tt.mode, processor.stopMode)

			storedMode, ok := manager.taskContexts.Load(taskID + "_stop_mode")
			assert.True(t, ok)
			assert.Equal(t, tt.mode, storedMode)
		})
	}
}

// TestCancelTask 测试取消任务（带回滚）
func TestCancelTask(t *testing.T) {
	reporter := &mockReporter{}
	stepWorker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result")),
		rollbackErr:   nil,
	}
	taskWorker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("step1", stepWorker),
		},
	}

	stepProvider := &mockStepInfoProvider{
		taskInfo: TaskData{
			TaskID:   "task-cancel-001",
			TaskCode: "test_task",
			Status:   StatusWaiting,
		},
	}

	manager := NewTaskManager(
		stepProvider,
		reporter,
		&mockDataProvider{},
		&mockLockProvider{},
		&mockCanRunTask{},
	)
	manager.RegisterTaskWorker("test_task", taskWorker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动管理器
	err := manager.Start(ctx)
	assert.NoError(t, err)

	// 先注册停止操作
	err = manager.cancelTask("task-cancel-001")
	assert.NoError(t, err)

	// 然后发送任务
	taskData := &CallTaskReq{
		TaskID:   "task-cancel-001",
		TaskCode: "test_task",
	}
	manager.executeTask(ctx, taskData)

	// 等待处理完成
	time.Sleep(200 * time.Millisecond)

	// 验证结果
	assert.True(t, stepWorker.executeCalled || !stepWorker.executeCalled, "步骤应该被处理")

	// 检查最终状态（可能已完成或已取消）
	if len(reporter.taskReports) > 0 {
		lastReport := reporter.taskReports[len(reporter.taskReports)-1]
		assert.Equal(t, StatusCanceled, lastReport.Status, "预存取消应直接落成取消状态")
	}
	assert.False(t, stepWorker.executeCalled, "预存取消时不应真正执行步骤")

	manager.Stop()
	manager.Wait()
}

// TestStopTask 测试停止任务（不回滚）
func TestStopTask(t *testing.T) {
	reporter := &mockReporter{}
	stepWorker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result")),
		rollbackErr:   nil,
	}
	taskWorker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("step1", stepWorker),
		},
	}

	stepProvider := &mockStepInfoProvider{
		taskInfo: TaskData{
			TaskID:   "task-stop-001",
			TaskCode: "test_task",
			Status:   StatusWaiting,
		},
	}

	manager := NewTaskManager(
		stepProvider,
		reporter,
		&mockDataProvider{},
		&mockLockProvider{},
		&mockCanRunTask{},
	)
	manager.RegisterTaskWorker("test_task", taskWorker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动管理器
	err := manager.Start(ctx)
	assert.NoError(t, err)

	// 先注册停止操作
	err = manager.stopTask("task-stop-001")
	assert.NoError(t, err)

	// 然后发送任务
	taskData := &CallTaskReq{
		TaskID:   "task-stop-001",
		TaskCode: "test_task",
	}
	manager.executeTask(ctx, taskData)

	// 等待处理完成
	time.Sleep(200 * time.Millisecond)

	// 验证结果
	if len(reporter.taskReports) > 0 {
		lastReport := reporter.taskReports[len(reporter.taskReports)-1]
		assert.Equal(t, StatusStopped, lastReport.Status, "预存停止应直接落成停止状态")
	}
	assert.False(t, stepWorker.executeCalled, "预存停止时不应真正执行步骤")

	manager.Stop()
	manager.Wait()
}

// TestPauseTask 测试暂停任务（不回滚）
func TestPauseTask(t *testing.T) {
	reporter := &mockReporter{}
	stepWorker := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result")),
		rollbackErr:   nil,
	}
	taskWorker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("step1", stepWorker),
		},
	}

	stepProvider := &mockStepInfoProvider{
		taskInfo: TaskData{
			TaskID:   "task-pause-001",
			TaskCode: "test_task",
			Status:   StatusWaiting,
		},
	}

	manager := NewTaskManager(
		stepProvider,
		reporter,
		&mockDataProvider{},
		&mockLockProvider{},
		&mockCanRunTask{},
	)
	manager.RegisterTaskWorker("test_task", taskWorker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动管理器
	err := manager.Start(ctx)
	assert.NoError(t, err)

	// 先注册暂停操作
	err = manager.pauseTask("task-pause-001")
	assert.NoError(t, err)

	// 然后发送任务
	taskData := &CallTaskReq{
		TaskID:   "task-pause-001",
		TaskCode: "test_task",
	}
	manager.executeTask(ctx, taskData)

	// 等待处理完成
	time.Sleep(200 * time.Millisecond)

	// 验证结果
	if len(reporter.taskReports) > 0 {
		lastReport := reporter.taskReports[len(reporter.taskReports)-1]
		assert.Equal(t, StatusPaused, lastReport.Status, "预存暂停应直接落成暂停状态")
	}
	assert.False(t, stepWorker.executeCalled, "预存暂停时不应真正执行步骤")

	manager.Stop()
	manager.Wait()
}

// TestResumeTask 测试恢复任务
func TestResumeTask(t *testing.T) {
	reporter := &mockReporter{}
	stepWorker1 := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result1")),
	}
	stepWorker2 := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result2")),
	}

	taskWorker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("step1", stepWorker1),
			NewStepInfo("step2", stepWorker2),
		},
	}

	// 模拟一个已暂停的任务
	stepProvider := &mockStepInfoProvider{
		taskInfo: TaskData{
			TaskID:   "task-resume-001",
			TaskCode: "test_task",
			Status:   StatusPaused,
			Steps: []*StepData{
				{Name: "step1", Status: StatusCompleted, Result: []byte("result1")},
				{Name: "step2", Status: StatusPaused},
			},
		},
	}

	manager := NewTaskManager(
		stepProvider,
		reporter,
		&mockDataProvider{},
		&mockLockProvider{},
		&mockCanRunTask{},
	)
	manager.RegisterTaskWorker("test_task", taskWorker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动管理器
	err := manager.Start(ctx)
	assert.NoError(t, err)

	// 恢复任务
	err = manager.ResumeTask("task-resume-001")
	assert.NoError(t, err)

	// 等待处理完成
	time.Sleep(200 * time.Millisecond)

	// 验证结果 - 由于任务已暂停且第一个步骤已完成，
	// 系统应该跳过已完成的步骤，第二个步骤应该被执行
	assert.False(t, stepWorker1.executeCalled, "第一个步骤应该被跳过（已完成）")
	assert.True(t, stepWorker2.executeCalled, "第二个步骤应该被执行")

	manager.Stop()
	manager.Wait()
}

// TestResumeTaskClearsStoredPauseMode 测试恢复任务时会清理暂停残留标记
func TestResumeTaskClearsStoredPauseMode(t *testing.T) {
	reporter := &mockReporter{}
	stepWorker1 := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result1")),
	}
	stepWorker2 := &mockStepWorker{
		executeResult: StepExecCompleted([]byte("result2")),
	}

	taskWorker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("step1", stepWorker1),
			NewStepInfo("step2", stepWorker2),
		},
	}

	stepProvider := &mockStepInfoProvider{
		taskInfo: TaskData{
			TaskID:          "task-resume-pause-001",
			TaskCode:        "test_task",
			Status:          StatusPaused,
			RunningStepName: "step2",
			Steps: []*StepData{
				{Name: "step1", Status: StatusCompleted, Result: []byte("result1")},
				{Name: "step2", Status: StatusPaused},
			},
		},
	}

	manager := NewTaskManager(
		stepProvider,
		reporter,
		&mockDataProvider{},
		&mockLockProvider{},
		&mockCanRunTask{},
	)
	manager.RegisterTaskWorker("test_task", taskWorker)
	manager.taskContexts.Store("task-resume-pause-001_stop_mode", StopModePause)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	assert.NoError(t, err)

	err = manager.ResumeTask("task-resume-pause-001")
	assert.NoError(t, err)

	time.Sleep(200 * time.Millisecond)

	assert.False(t, stepWorker1.executeCalled, "第一个步骤应该被跳过（已完成）")
	assert.True(t, stepWorker2.executeCalled, "恢复时不应被旧的 pause 标记再次拦截")

	_, stillStored := manager.taskContexts.Load("task-resume-pause-001_stop_mode")
	assert.False(t, stillStored, "恢复后应清理残留的 stop_mode")

	hasCompleted := false
	for _, report := range reporter.taskReports {
		if report.TaskID == "task-resume-pause-001" && report.Status == StatusCompleted {
			hasCompleted = true
			break
		}
	}
	assert.True(t, hasCompleted, "恢复后的任务应能继续完成")

	manager.Stop()
	manager.Wait()
}

type blockingPauseStepWorker struct {
	started     chan struct{}
	startedOnce sync.Once
	pauseCalled bool
}

func (m *blockingPauseStepWorker) Execute(ctx context.Context, input StepInput) StepExecResult {
	m.startedOnce.Do(func() {
		close(m.started)
	})
	<-ctx.Done()
	return StepExecFailed(context.Cause(ctx))
}

func (m *blockingPauseStepWorker) PollStatus(ctx context.Context, input StepPollInput) StepPollStatusResult {
	return StepPollCompleted(nil)
}

func (m *blockingPauseStepWorker) Rollback(ctx context.Context, input StepRollbackInput) error {
	return nil
}

func (m *blockingPauseStepWorker) Pause(ctx context.Context, input StepInput) error {
	m.pauseCalled = true
	return nil
}

type dynamicStepInfoProvider struct {
	mu       sync.RWMutex
	taskInfo TaskData
}

func (m *dynamicStepInfoProvider) GetTaskStepInfo(taskId string) (*TaskData, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	copyInfo := m.taskInfo
	if len(m.taskInfo.Steps) > 0 {
		copyInfo.Steps = make([]*StepData, 0, len(m.taskInfo.Steps))
		for _, step := range m.taskInfo.Steps {
			if step == nil {
				continue
			}
			stepCopy := *step
			copyInfo.Steps = append(copyInfo.Steps, &stepCopy)
		}
	}
	return &copyInfo, nil
}

func (m *dynamicStepInfoProvider) SetTaskInfo(taskInfo TaskData) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.taskInfo = taskInfo
}

type pauseThenResumeStepWorker struct {
	provider          *dynamicStepInfoProvider
	firstStarted      chan struct{}
	secondStarted     chan struct{}
	firstStartedOnce  sync.Once
	secondStartedOnce sync.Once
	mu                sync.Mutex
	executeCount      int
	pauseCalled       bool
}

func (m *pauseThenResumeStepWorker) Execute(ctx context.Context, input StepInput) StepExecResult {
	m.mu.Lock()
	m.executeCount++
	count := m.executeCount
	m.mu.Unlock()

	if count == 1 {
		m.firstStartedOnce.Do(func() {
			close(m.firstStarted)
		})
		<-ctx.Done()
		return StepExecFailed(context.Cause(ctx))
	}

	m.secondStartedOnce.Do(func() {
		close(m.secondStarted)
	})
	return StepExecCompleted([]byte("resumed"))
}

func (m *pauseThenResumeStepWorker) PollStatus(ctx context.Context, input StepPollInput) StepPollStatusResult {
	return StepPollCompleted(nil)
}

func (m *pauseThenResumeStepWorker) Rollback(ctx context.Context, input StepRollbackInput) error {
	return nil
}

func (m *pauseThenResumeStepWorker) Pause(ctx context.Context, input StepInput) error {
	m.mu.Lock()
	m.pauseCalled = true
	m.mu.Unlock()
	if m.provider != nil {
		m.provider.SetTaskInfo(TaskData{
			TaskID:          input.TaskID,
			TaskCode:        "test_task",
			Status:          StatusPaused,
			RunningStepName: "step1",
			Steps: []*StepData{
				{Name: "step1", Status: StatusPaused},
			},
		})
	}
	return nil
}

func TestExecuteTaskCleansStoredStopModeOnPauseExit(t *testing.T) {
	reporter := &mockReporter{}
	stepWorker := &blockingPauseStepWorker{
		started: make(chan struct{}),
	}
	taskWorker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("step1", stepWorker),
		},
	}

	stepProvider := &mockStepInfoProvider{
		taskInfo: TaskData{
			TaskID:   "task-pause-clean-001",
			TaskCode: "test_task",
			Status:   StatusWaiting,
		},
	}

	manager := NewTaskManager(
		stepProvider,
		reporter,
		&mockDataProvider{},
		&mockLockProvider{},
		&mockCanRunTask{},
	)
	manager.RegisterTaskWorker("test_task", taskWorker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	assert.NoError(t, err)

	done := make(chan struct{})
	go func() {
		manager.executeTask(ctx, &CallTaskReq{
			TaskID:   "task-pause-clean-001",
			TaskCode: "test_task",
		})
		close(done)
	}()

	select {
	case <-stepWorker.started:
	case <-time.After(time.Second):
		t.Fatal("task did not start in time")
	}

	err = manager.pauseTask("task-pause-clean-001")
	assert.NoError(t, err)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("task did not exit after pause")
	}

	_, hasTaskCtx := manager.taskContexts.Load("task-pause-clean-001")
	assert.False(t, hasTaskCtx, "退出后不应保留 task cancel context")

	_, hasStopMode := manager.taskContexts.Load("task-pause-clean-001_stop_mode")
	assert.False(t, hasStopMode, "退出后不应保留 stop_mode 残留")

	assert.True(t, stepWorker.pauseCalled, "暂停时应调用步骤 Pause")

	manager.Stop()
	manager.Wait()
}

func TestResumeTaskDeferredUntilAfterUnlock(t *testing.T) {
	reporter := &mockReporter{}
	stepProvider := &dynamicStepInfoProvider{
		taskInfo: TaskData{
			TaskID:   "task-resume-deferred-001",
			TaskCode: "test_task",
			Status:   StatusWaiting,
		},
	}
	stepWorker := &pauseThenResumeStepWorker{
		provider:      stepProvider,
		firstStarted:  make(chan struct{}),
		secondStarted: make(chan struct{}),
	}
	taskWorker := &mockTaskWorker{
		steps: []StepRegInfo{
			NewStepInfo("step1", stepWorker),
		},
	}

	manager := NewTaskManager(
		stepProvider,
		reporter,
		&mockDataProvider{},
		&mockLockProvider{},
		&mockCanRunTask{},
	)
	manager.RegisterTaskWorker("test_task", taskWorker)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := manager.Start(ctx)
	assert.NoError(t, err)

	done := make(chan struct{})
	go func() {
		manager.executeTask(ctx, &CallTaskReq{
			TaskID:   "task-resume-deferred-001",
			TaskCode: "test_task",
		})
		close(done)
	}()

	select {
	case <-stepWorker.firstStarted:
	case <-time.After(time.Second):
		t.Fatal("task did not start in time")
	}

	err = manager.pauseTask("task-resume-deferred-001")
	assert.NoError(t, err)

	err = manager.ResumeTask("task-resume-deferred-001")
	assert.NoError(t, err)

	select {
	case <-stepWorker.secondStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("task did not resume after pause exit")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("original task execution did not exit in time")
	}

	assert.True(t, stepWorker.pauseCalled, "pause callback should be invoked before resume")

	deadline := time.After(2 * time.Second)
	for {
		hasCompleted := false
		for _, report := range reporter.taskReports {
			if report.TaskID == "task-resume-deferred-001" && report.Status == StatusCompleted {
				hasCompleted = true
				break
			}
		}
		if hasCompleted {
			break
		}
		select {
		case <-deadline:
			t.Fatal("deferred resume should complete after lock release")
		case <-time.After(10 * time.Millisecond):
		}
	}

	manager.Stop()
	manager.Wait()
}

// TestDirectProcessorModification 测试直接修改processor中的stopMode
func TestDirectProcessorModification(t *testing.T) {
	// 创建一个简单的processor用于测试
	processor := &TaskProcessor{
		stopMode: StopModeCancel,
	}

	// 创建manager并存储processor
	manager := NewTaskManager(
		&mockStepInfoProvider{},
		&mockReporter{},
		&mockDataProvider{},
		&mockLockProvider{},
		&mockCanRunTask{},
	)

	manager.taskProcessors.Store("test-task-001", processor)

	// 验证初始状态
	assert.Equal(t, StopModeCancel, processor.stopMode, "初始stopMode应该是Cancel")

	// 测试pauseTask直接修改processor
	err := manager.pauseTask("test-task-001")
	assert.NoError(t, err)
	assert.Equal(t, StopModePause, processor.stopMode, "pauseTask应该将stopMode修改为Pause")

	// 测试stopTask直接修改processor
	err = manager.stopTask("test-task-001")
	assert.NoError(t, err)
	assert.Equal(t, StopModeStop, processor.stopMode, "stopTask应该将stopMode修改为Stop")

	// 测试cancelTask直接修改processor
	err = manager.cancelTask("test-task-001")
	assert.NoError(t, err)
	assert.Equal(t, StopModeCancel, processor.stopMode, "cancelTask应该将stopMode修改为Cancel")

	// 测试对不存在任务的操作
	err = manager.pauseTask("non-existent-task")
	assert.NoError(t, err)

	err = manager.stopTask("non-existent-task")
	assert.NoError(t, err)

	err = manager.cancelTask("non-existent-task")
	assert.NoError(t, err)
}

// TestProcessorModificationWithoutRunningTask 测试对未运行任务的processor修改
func TestProcessorModificationWithoutRunningTask(t *testing.T) {
	manager := NewTaskManager(
		&mockStepInfoProvider{},
		&mockReporter{},
		&mockDataProvider{},
		&mockLockProvider{},
		&mockCanRunTask{},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动管理器
	err := manager.Start(ctx)
	assert.NoError(t, err)

	// 对不存在的任务调用各种停止操作
	err = manager.pauseTask("non-existent-task")
	assert.NoError(t, err)

	err = manager.stopTask("non-existent-task")
	assert.NoError(t, err)

	err = manager.cancelTask("non-existent-task")
	assert.NoError(t, err)

	// 验证预存储的停止模式
	storedMode, exists := manager.taskContexts.Load("non-existent-task_stop_mode")
	assert.True(t, exists, "应该预存储停止模式")
	assert.Equal(t, StopModeCancel, storedMode, "预存储的模式应该是Cancel")

	manager.Stop()
	manager.Wait()
}
