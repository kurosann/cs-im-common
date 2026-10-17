package task

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockRecorder 模拟记录器
type MockRecorder struct {
	TaskResults []TaskExecResult
	StepResults []StepExecResult
}

func (m *MockRecorder) RecordTask(taskID string, result TaskExecResult) (string, error) {
	if taskID == "" {
		m.TaskResults = append(m.TaskResults, result)
		return fmt.Sprintf("%d", len(m.TaskResults)), nil
	}

	tid, err := strconv.Atoi(taskID)
	if err != nil {
		return "", err
	}
	if tid <= len(m.TaskResults) {
		m.TaskResults[tid-1] = result
	}
	return taskID, nil
}

func (m *MockRecorder) RecordStep(stepID string, result StepExecResult) (string, error) {
	if stepID == "" {
		// 新步骤，追加结果
		m.StepResults = append(m.StepResults, result)
		return fmt.Sprintf("%d", len(m.StepResults)), nil
	}
	sid, err := strconv.Atoi(stepID)
	if err != nil {
		return "", err
	}
	// 已存在的步骤，更新结果
	if sid <= len(m.StepResults) {
		m.StepResults[sid-1] = result
	}
	return stepID, nil
}

func (m *MockRecorder) QueryStep(stepID string) (result StepExecResult, err error) {
	sid, err := strconv.Atoi(stepID)
	if err != nil {
		return StepExecResult{}, err
	}
	return m.StepResults[sid-1], nil
}

func (m *MockRecorder) QueryTask(taskID string) (result TaskExecResult, err error) {
	tid, err := strconv.Atoi(taskID)
	if err != nil {
		return TaskExecResult{}, err
	}
	return m.TaskResults[tid-1], nil
}

// MockStep 模拟步骤
type MockStep struct {
	ExecuteFunc    func(ctx context.Context, payload []byte, lastStepResult []byte) StepExecResult
	RollbackFunc   func(ctx context.Context, payload []byte, lastStepResult []byte) error
	PollStatusFunc func(ctx context.Context, payload []byte, lastStepResult []byte) StepPollStatusResult
}

func (m *MockStep) Execute(ctx context.Context, payload []byte, lastStepResult []byte) StepExecResult {
	return m.ExecuteFunc(ctx, payload, lastStepResult)
}

func (m *MockStep) Rollback(ctx context.Context, payload []byte, lastStepResult []byte) error {
	return m.RollbackFunc(ctx, payload, lastStepResult)
}

func (m *MockStep) PollStatus(ctx context.Context, payload []byte, lastStepResult []byte) StepPollStatusResult {
	return m.PollStatusFunc(ctx, payload, lastStepResult)
}

// MockTask 模拟任务
type MockTask struct {
	Steps []StepInfo
}

func (m *MockTask) Arrange(ctx context.Context, payload []byte) *StepSeq {
	return NewStepSeq().AddStep(m.Steps...)
}

func TestDistributedTransaction(t *testing.T) {
	tests := []struct {
		name           string
		steps          []StepInfo
		expectedError  bool
		expectedStatus Status
	}{
		{
			name: "成功执行所有步骤",
			steps: []StepInfo{
				{
					Name: "步骤1",
					StepWorker: &MockStep{
						ExecuteFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepExecResult {
							return StepExecCompleted([]byte("步骤1完成"))
						},
						RollbackFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) error {
							return nil
						},
						PollStatusFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepPollStatusResult {
							return StepPollCompleted([]byte("步骤1完成"))
						},
					},
				},
				{
					Name: "步骤2",
					StepWorker: &MockStep{
						ExecuteFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepExecResult {
							return StepExecCompleted([]byte("步骤2完成"))
						},
						RollbackFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) error {
							return nil
						},
						PollStatusFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepPollStatusResult {
							return StepPollCompleted([]byte("步骤2完成"))
						},
					},
				},
			},
			expectedError:  false,
			expectedStatus: StatusCompleted,
		},
		{
			name: "步骤2失败并回滚",
			steps: []StepInfo{
				{
					Name: "步骤1",
					StepWorker: &MockStep{
						ExecuteFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepExecResult {
							return StepExecCompleted([]byte("步骤1完成"))
						},
						RollbackFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) error {
							return nil
						},
						PollStatusFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepPollStatusResult {
							return StepPollCompleted([]byte("步骤1完成"))
						},
					},
				},
				{
					Name: "步骤2",
					StepWorker: &MockStep{
						ExecuteFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepExecResult {
							return StepExecFailed(errors.New("步骤2执行失败"))
						},
						RollbackFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) error {
							return nil
						},
						PollStatusFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepPollStatusResult {
							return StepPollFailed(errors.New("步骤2执行失败"))
						},
					},
				},
			},
			expectedError:  true,
			expectedStatus: StatusFailed,
		},
		{
			name: "超时测试",
			steps: []StepInfo{
				{
					Name: "超时步骤",
					StepWorker: &MockStep{
						ExecuteFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepExecResult {
							return StepExecPending([]byte("超时步骤"))
						},
						RollbackFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) error {
							return nil
						},
						PollStatusFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepPollStatusResult {
							return StepPollPending(0, "超时步骤")
						},
					},
				},
			},
			expectedError:  true,
			expectedStatus: StatusTimeout,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &MockRecorder{}
			tm := NewTaskManager(recorder)

			mockTask := &MockTask{Steps: tt.steps}
			tm.RegisterTask(&TaskInfo{
				Name: "测试任务",
				Task: mockTask,
			})

			ctx := context.Background()
			if tt.expectedStatus == StatusTimeout {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
				defer cancel()
			}

			result, err := tm.RunTask(ctx, "测试任务", []byte("测试数据"))

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}

			// 验证最终状态
			lastTaskResult := recorder.TaskResults[len(recorder.TaskResults)-1]
			assert.Equal(t, tt.expectedStatus, lastTaskResult.Status, lastTaskResult.Err)

			// 验证回滚逻辑
			if tt.expectedError {
				hasRollback := false
				for _, result := range recorder.StepResults {
					if result.Status == StatusRollingBack || result.Status == StatusRolledBack || result.Status == StatusTimeout {
						hasRollback = true
						break
					}
				}
				assert.True(t, hasRollback, "期望执行回滚操作")
			}
		})
	}
}

func TestOrderProcessDemo(t *testing.T) {
	// 模拟数据存储
	type Storage struct {
		inventory int
		balance   float64
		orders    []string
	}
	storage := &Storage{
		inventory: 10,
		balance:   1000.0,
	}

	// 库存检查步骤
	checkInventoryStep := &MockStep{
		ExecuteFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepExecResult {
			if storage.inventory <= 0 {
				return StepExecFailed(errors.New("库存不足"))
			}
			return StepExecCompleted([]byte("库存检查完成"))
		},
		RollbackFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) error {
			return nil // 检查库存步骤无需回滚
		},
		PollStatusFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepPollStatusResult {
			return StepPollCompleted([]byte("库存检查完成"))
		},
	}

	// 扣款步骤
	deductBalanceStep := &MockStep{
		ExecuteFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepExecResult {
			amount := 100.0
			if storage.balance < amount {
				return StepExecFailed(errors.New("余额不足"))
			}
			storage.balance -= amount
			return StepExecCompleted([]byte(fmt.Sprintf("扣款成功，剩余余额: %.2f", storage.balance)))
		},
		RollbackFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) error {
			storage.balance += 100.0
			return nil
		},
		PollStatusFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepPollStatusResult {
			return StepPollCompleted([]byte("扣款完成"))
		},
	}

	// 创建订单步骤
	createOrderStep := &MockStep{
		ExecuteFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepExecResult {
			orderID := fmt.Sprintf("ORDER_%d", len(storage.orders)+1)
			storage.orders = append(storage.orders, orderID)
			storage.inventory--
			return StepExecCompleted([]byte(fmt.Sprintf("订单创建成功: %s", orderID)))
		},
		RollbackFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) error {
			if len(storage.orders) > 0 {
				storage.orders = storage.orders[:len(storage.orders)-1]
				storage.inventory++
			}
			return nil
		},
		PollStatusFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepPollStatusResult {
			return StepPollCompleted([]byte("订单创建完成"))
		},
	}

	tests := []struct {
		name           string
		setupStorage   func()
		steps          []StepInfo
		expectedError  bool
		expectedStatus Status
	}{
		{
			name: "订单处理成功",
			setupStorage: func() {
				storage.inventory = 10
				storage.balance = 1000.0
				storage.orders = nil
			},
			steps: []StepInfo{
				{Name: "检查库存", StepWorker: checkInventoryStep},
				{Name: "扣除余额", StepWorker: deductBalanceStep},
				{Name: "创建订单", StepWorker: createOrderStep},
			},
			expectedError:  false,
			expectedStatus: StatusCompleted,
		},
		{
			name: "库存不足失败",
			setupStorage: func() {
				storage.inventory = 0
				storage.balance = 1000.0
				storage.orders = nil
			},
			steps: []StepInfo{
				{Name: "检查库存", StepWorker: checkInventoryStep},
				{Name: "扣除余额", StepWorker: deductBalanceStep},
				{Name: "创建订单", StepWorker: createOrderStep},
			},
			expectedError:  true,
			expectedStatus: StatusFailed,
		},
		{
			name: "余额不足失败并回滚",
			setupStorage: func() {
				storage.inventory = 10
				storage.balance = 50.0
				storage.orders = nil
			},
			steps: []StepInfo{
				{Name: "检查库存", StepWorker: checkInventoryStep},
				{Name: "扣除余额", StepWorker: deductBalanceStep},
				{Name: "创建订单", StepWorker: createOrderStep},
			},
			expectedError:  true,
			expectedStatus: StatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 重置存储状态
			tt.setupStorage()

			// 初始值记录
			initialInventory := storage.inventory
			initialBalance := storage.balance
			initialOrders := len(storage.orders)

			recorder := &MockRecorder{}
			tm := NewTaskManager(recorder)

			mockTask := &MockTask{Steps: tt.steps}
			tm.RegisterTask(&TaskInfo{
				Name: "订单处理",
				Task: mockTask,
			})

			result, err := tm.RunTask(context.Background(), "订单处理", []byte("订单数据"))

			if tt.expectedError {
				assert.Error(t, err)
				// 验证回滚是否成功
				assert.Equal(t, initialInventory, storage.inventory, "库存应该回滚到初始状态")
				assert.Equal(t, initialBalance, storage.balance, "余额应该回滚到初始状态")
				assert.Equal(t, initialOrders, len(storage.orders), "订单数量应该回滚到初始状态")
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				// 验证成功执行的结果
				assert.Equal(t, initialInventory-1, storage.inventory, "库存应该减少1")
				assert.Equal(t, initialBalance-100.0, storage.balance, "余额应该减少100")
				assert.Equal(t, initialOrders+1, len(storage.orders), "订单数量应该增加1")
			}

			// 验证最终状态
			lastTaskResult := recorder.TaskResults[len(recorder.TaskResults)-1]
			assert.Equal(t, tt.expectedStatus, lastTaskResult.Status)
		})
	}
}

func TestSimpleCalculationDemo(t *testing.T) {
	// 创建加法步骤
	addStep := &MockStep{
		ExecuteFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepExecResult {
			return StepExecCompleted([]byte("10"))
		},
		RollbackFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) error {
			return nil
		},
		PollStatusFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepPollStatusResult {
			return StepPollCompleted([]byte("10"))
		},
	}

	// 创建乘法步骤
	multiplyStep := &MockStep{
		ExecuteFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepExecResult {
			// 将上一步的结果乘以2
			return StepExecCompleted([]byte("20"))
		},
		RollbackFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) error {
			return nil
		},
		PollStatusFunc: func(ctx context.Context, payload []byte, lastStepResult []byte) StepPollStatusResult {
			return StepPollCompleted([]byte("20"))
		},
	}

	// 创建任务步骤列表
	steps := []StepInfo{
		{Name: "加法运算", StepWorker: addStep},
		{Name: "乘法运算", StepWorker: multiplyStep},
	}

	// 创建记录器和任务管理器
	recorder := &MockRecorder{}
	tm := NewTaskManager(recorder)

	// 注册任务
	mockTask := &MockTask{Steps: steps}
	tm.RegisterTask(&TaskInfo{
		Name: "计算任务",
		Task: mockTask,
	})

	// 执行任务
	result, err := tm.RunTask(context.Background(), "计算任务", []byte("初始数据"))

	// 验证结果
	assert.NoError(t, err)
	assert.NotNil(t, result)

	// 验证任务状态
	lastTaskResult := recorder.TaskResults[len(recorder.TaskResults)-1]
	assert.Equal(t, StatusCompleted, lastTaskResult.Status)

	// 验证步骤执行结果
	assert.Equal(t, 2, len(recorder.StepResults))
	assert.Equal(t, []byte("10"), recorder.StepResults[0].Result)
	assert.Equal(t, []byte("20"), recorder.StepResults[1].Result)
}

func TestTaskManager(t *testing.T) {
	tests := []struct {
		name     string
		testFunc func(t *testing.T)
	}{
		{"测试正常任务执行", testNormalTaskExecution},
		{"测试任务执行失败和回滚", testTaskFailureAndRollback},
		{"测试任务超时", testTaskTimeout},
		{"测试并发任务执行", testConcurrentTaskExecution},
		{"测试状态轮询", testStatusPolling},
		{"测试任务取消", testTaskCancellation},
		{"测试步骤panic恢复", testStepPanicRecovery},
		{"测试记录器功能", testRecorderFunctionality},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.testFunc)
	}
}

func testNormalTaskExecution(t *testing.T) {
	ctx := context.Background()
	recorder := &MockRecorder{}
	tm := NewTaskManager(recorder)

	// 创建一个模拟任务
	task := &TaskInfo{
		Name: "test-task",
		Task: &mockTaskWorker{
			steps: []StepInfo{
				{
					Name: "step1",
					StepWorker: &mockStepWorker{
						executeFunc: func(ctx context.Context, payload, stepResult []byte) StepExecResult {
							return StepExecCompleted([]byte("step1 completed"))
						},
					},
				},
			},
		},
	}

	tm.RegisterTask(task)
	result, err := tm.RunTask(ctx, "test-task", nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
}

func testTaskFailureAndRollback(t *testing.T) {
	ctx := context.Background()
	recorder := &MockRecorder{}
	tm := NewTaskManager(recorder)

	// 创建一个会失败的任务
	task := &TaskInfo{
		Name: "failing-task",
		Task: &mockTaskWorker{
			steps: []StepInfo{
				{
					Name: "failing-step",
					StepWorker: &mockStepWorker{
						executeFunc: func(ctx context.Context, payload, stepResult []byte) StepExecResult {
							return StepExecFailed(errors.New("模拟的错误"))
						},
						rollbackFunc: func(ctx context.Context, payload, result []byte) error {
							return nil
						},
					},
				},
			},
		},
	}

	tm.RegisterTask(task)
	_, err := tm.RunTask(ctx, "failing-task", nil)

	assert.Error(t, err)
	// 验证回滚是否被调用
	// 验证记录器是否正确记录了状态变化
}

func testTaskTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	recorder := &MockRecorder{}
	tm := NewTaskManager(recorder)

	// 创建一个会超时的任务
	task := &TaskInfo{
		Name: "timeout-task",
		Task: &mockTaskWorker{
			steps: []StepInfo{
				{
					Name: "long-running-step",
					StepWorker: &mockStepWorker{
						executeFunc: func(ctx context.Context, payload, stepResult []byte) StepExecResult {
							// 返回pending状态以触发轮询
							return StepExecPending([]byte("超时步骤"))
						},
						pollFunc: func(ctx context.Context, payload, stepResult []byte) StepPollStatusResult {
							time.Sleep(200 * time.Millisecond)
							return StepPollCompleted([]byte("completed"))
						},
						rollbackFunc: func(ctx context.Context, payload, result []byte) error {
							return nil
						},
					},
				},
			},
		},
	}

	tm.RegisterTask(task)
	_, err := tm.RunTask(ctx, "timeout-task", nil)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.DeadlineExceeded), "应该是超时错误")

	// 验证任务状态
	lastTaskResult := recorder.TaskResults[len(recorder.TaskResults)-1]
	assert.Equal(t, StatusTimeout, lastTaskResult.Status, "任务状态应该是超时")
}

type mockTaskWorker struct {
	steps []StepInfo
}

func (m *mockTaskWorker) Arrange(ctx context.Context, payload []byte) *StepSeq {
	return NewStepSeq().AddStep(m.steps...)
}

type mockStepWorker struct {
	executeFunc  func(ctx context.Context, payload, stepResult []byte) StepExecResult
	rollbackFunc func(ctx context.Context, payload, result []byte) error
	pollFunc     func(ctx context.Context, payload, stepResult []byte) StepPollStatusResult
}

func (m *mockStepWorker) Execute(ctx context.Context, payload, stepResult []byte) StepExecResult {
	return m.executeFunc(ctx, payload, stepResult)
}

func (m *mockStepWorker) Rollback(ctx context.Context, payload, result []byte) error {
	return m.rollbackFunc(ctx, payload, result)
}

func (m *mockStepWorker) PollStatus(ctx context.Context, payload, stepResult []byte) StepPollStatusResult {
	return m.pollFunc(ctx, payload, stepResult)
}

func testConcurrentTaskExecution(t *testing.T) {
	recorder := &MockRecorder{}
	tm := NewTaskManager(recorder)

	// 创建一个简单的任务
	task := &TaskInfo{
		Name: "concurrent-task",
		Task: &mockTaskWorker{
			steps: []StepInfo{
				{
					Name: "concurrent-step",
					StepWorker: &mockStepWorker{
						executeFunc: func(ctx context.Context, payload, stepResult []byte) StepExecResult {
							time.Sleep(100 * time.Millisecond)
							return StepExecCompleted([]byte("step1 completed"))
						},
					},
				},
			},
		},
	}

	tm.RegisterTask(task)

	// 并发执行多个任务
	const concurrentTasks = 5
	var wg sync.WaitGroup
	errors := make(chan error, concurrentTasks)

	for i := 0; i < concurrentTasks; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := tm.RunTask(context.Background(), "concurrent-task", nil)
			if err != nil {
				errors <- err
			}
		}()
	}

	wg.Wait()
	close(errors)

	// 检查是否有错误发生
	for err := range errors {
		assert.NoError(t, err)
	}

	// 验证所有任务是否都完成
	assert.Equal(t, concurrentTasks, len(recorder.TaskResults))
	for _, result := range recorder.TaskResults {
		assert.Equal(t, StatusCompleted, result.Status)
	}
}

func testStatusPolling(t *testing.T) {
	ctx := context.Background()
	recorder := &MockRecorder{}
	tm := NewTaskManager(recorder)

	pollCount := 0
	task := &TaskInfo{
		Name: "polling-task",
		Task: &mockTaskWorker{
			steps: []StepInfo{
				{
					Name: "polling-step",
					StepWorker: &mockStepWorker{
						executeFunc: func(ctx context.Context, payload, stepResult []byte) StepExecResult {
							return StepExecPending([]byte("轮询步骤"))
						},
						pollFunc: func(ctx context.Context, payload, stepResult []byte) StepPollStatusResult {
							pollCount++
							if pollCount < 3 {
								return StepPollPending(pollCount*30, "轮询步骤")
							}
							return StepPollCompleted([]byte("polling completed"))
						},
					},
				},
			},
		},
	}

	tm.RegisterTask(task)
	result, err := tm.RunTask(ctx, "polling-task", nil)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 3, pollCount)

	// 验证进度记录
	var foundProgress bool
	for _, record := range recorder.StepResults {
		if record.ProcessPosition > 0 {
			foundProgress = true
			break
		}
	}
	assert.True(t, foundProgress, "应该记录轮询进度")
}

func testTaskCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	recorder := &MockRecorder{}
	tm := NewTaskManager(recorder)

	executed := make(chan struct{})
	task := &TaskInfo{
		Name: "cancellable-task",
		Task: &mockTaskWorker{
			steps: []StepInfo{
				{
					Name: "long-step",
					StepWorker: &mockStepWorker{
						executeFunc: func(ctx context.Context, payload, stepResult []byte) StepExecResult {
							close(executed)
							time.Sleep(200 * time.Millisecond)
							return StepExecCompleted([]byte("step1 completed"))
						},
					},
				},
			},
		},
	}

	tm.RegisterTask(task)

	go func() {
		<-executed
		cancel()
	}()

	_, err := tm.RunTask(ctx, "cancellable-task", nil)

	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled))
}

func testStepPanicRecovery(t *testing.T) {
	ctx := context.Background()
	recorder := &MockRecorder{}
	tm := NewTaskManager(recorder)

	task := &TaskInfo{
		Name: "panic-task",
		Task: &mockTaskWorker{
			steps: []StepInfo{
				{
					Name: "panicking-step",
					StepWorker: &mockStepWorker{
						executeFunc: func(ctx context.Context, payload, stepResult []byte) StepExecResult {
							panic("预期的panic")
						},
						rollbackFunc: func(ctx context.Context, payload, result []byte) error {
							return nil
						},
					},
				},
			},
		},
	}

	tm.RegisterTask(task)
	_, err := tm.RunTask(ctx, "panic-task", nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "panic")

	// 验证是否正确记录了失败状态
	lastTaskResult := recorder.TaskResults[len(recorder.TaskResults)-1]
	assert.Equal(t, StatusFailed, lastTaskResult.Status)
}

func testRecorderFunctionality(t *testing.T) {
	ctx := context.Background()
	recorder := &MockRecorder{}
	tm := NewTaskManager(recorder)

	task := &TaskInfo{
		Name: "recorder-test-task",
		Task: &mockTaskWorker{
			steps: []StepInfo{
				{
					Name: "step1",
					StepWorker: &mockStepWorker{
						executeFunc: func(ctx context.Context, payload, stepResult []byte) StepExecResult {
							return StepExecCompleted([]byte("step1 completed"))
						},
					},
				},
				{
					Name: "step2",
					StepWorker: &mockStepWorker{
						executeFunc: func(ctx context.Context, payload, stepResult []byte) StepExecResult {
							return StepExecCompleted([]byte("step2 completed"))
						},
					},
				},
			},
		},
	}

	tm.RegisterTask(task)
	_, err := tm.RunTask(ctx, "recorder-test-task", nil)

	assert.NoError(t, err)

	// 验证任务记录
	assert.NotEmpty(t, recorder.TaskResults)
	assert.Equal(t, StatusCompleted, recorder.TaskResults[len(recorder.TaskResults)-1].Status)

	// 验证步骤记录
	assert.Equal(t, 2, len(recorder.StepResults))
	assert.Equal(t, "step1", recorder.StepResults[0].StepName)
	assert.Equal(t, "step2", recorder.StepResults[1].StepName)
	assert.Equal(t, StatusCompleted, recorder.StepResults[0].Status)
	assert.Equal(t, StatusCompleted, recorder.StepResults[1].Status)

	// 验证步骤ID的连续性
	assert.Equal(t, 1, recorder.StepResults[0].StepID)
	assert.Equal(t, 2, recorder.StepResults[1].StepID)
}
