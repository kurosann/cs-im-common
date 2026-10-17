# TaskMgr 分布式任务框架使用指南

## 一、框架概述

`taskmgr` 是一个**分布式步骤编排任务框架**，核心设计思想：

- **任务(Task)** 由多个有序 **步骤(Step)** 组成，按顺序执行
- 每个步骤可以是**同步**（Execute 直接返回 Completed）或**异步**（Execute 返回 Pending，然后通过 PollStatus 轮询直到完成）
- 步骤失败时，已完成的步骤会按**逆序回滚(Rollback)**
- 支持**暂停/恢复、停止、取消**三种中断模式
- 内置**分布式锁**和**锁续期**，支持多节点部署
- 通过**外部持久化接口**存储任务状态，断电可恢复

> 注意：`pkg/task` 是旧版框架（已标记 Deprecated），`pkg/taskmgr` 是新版，推荐使用新版。下文全部基于 `pkg/taskmgr`。

---

## 二、架构总览

```
┌──────────────────────────────────────────────────────┐
│                    TaskManager                        │
│  ┌─────────────┐  ┌─────────────┐                    │
│  │ watchTasks  │  │ pollTasks   │  任务获取（推+拉）  │
│  └──────┬──────┘  └──────┬──────┘                    │
│         └────────┬───────┘                           │
│                  ▼                                    │
│           executeTask()                               │
│         ┌────────┴────────┐                          │
│         │  distributed    │  分布式锁 + 续期          │
│         │  lock + keepAlive│                          │
│         └────────┬────────┘                          │
│                  ▼                                    │
│         buildProcessor()                              │
│         ┌────────┴────────┐                          │
│         │   TaskEngine    │                          │
│         │  Build / Revert │  新建 / 恢复任务         │
│         └────────┬────────┘                          │
│                  ▼                                    │
│         TaskProcessor.Process()                       │
│         ┌────────────────────────────────┐           │
│         │ Step1 → Step2 → ... → StepN    │ 顺序执行  │
│         │  ↕ ctx.Done() 时:              │           │
│         │   Pause / Stop / Cancel        │           │
│         │  ↕ 失败时:                     │           │
│         │   Rollback (逆序)              │           │
│         └────────────────────────────────┘           │
└──────────────────────────────────────────────────────┘
```

---

## 三、核心接口一览

### 3.1 你需要实现的接口

| 接口 | 方法 | 必须？ | 说明 |
|------|------|--------|------|
| `TaskWorker` | `Arrange(ctx, payload) []StepRegInfo` | **必须** | 编排步骤，返回步骤列表 |
| `StepWorker` | `Execute(ctx, StepInput) StepExecResult` | **必须** | 执行步骤逻辑 |
| `StepWorker` | `PollStatus(ctx, StepPollInput) StepPollStatusResult` | **必须** | 轮询异步步骤状态 |
| `StepWorker` | `Rollback(ctx, StepRollbackInput) error` | **必须** | 回滚步骤 |
| `StepWorkerPause` | `Pause(ctx, StepInput) error` | 可选 | 暂停时回调，用于保存状态 |
| `StepWorkerStop` | `Stop(ctx, StepInput) error` | 可选 | 停止时回调，用于清理资源 |
| `StepWorkerCancel` | `Cancel(ctx, StepInput) error` | 可选 | 取消时回调 |
| `StepPullStatusAfter` | `After(ctx, StepPollAfterInput) error` | 可选 | 异步轮询完成后的回调 |
| `BaseStepWorker` | 嵌入即可，集合了 Pause + Stop | 可选 | 便捷组合接口 |

### 3.2 外部依赖接口（需要你提供实现）

| 接口 | 方法 | 说明 |
|------|------|------|
| `ITaskDataProvider` | `WatchTask() (chan *CallTaskReq, error)` | 监听新任务（推送模式） |
| `ITaskDataProvider` | `ListenTask() ([]*CallTaskReq, error)` | 拉取待处理任务（轮询模式） |
| `ITaskReporter` | `ReportTaskInfo(*TaskReport) error` | 上报任务状态变更 |
| `ITaskReporter` | `ReportStepInfo(*StepReport) error` | 上报步骤状态变更 |
| `ITaskStepInfoProvider` | `GetTaskStepInfo(taskID) (*TaskData, error)` | 获取任务持久化信息（用于恢复） |
| `ITaskLockProvider` | `Lock/Unlock/KeepAlive(taskID, owner)` | 分布式锁 |
| `ICanRunTask` | `CanRunTask(taskID) error` | 判断任务是否可运行 |

### 3.3 任务状态机

```
  waiting ──→ running ──→ completed
                │              │
                ├─→ pending ──→┤ (异步轮询)
                │              │
                ├─→ failed ───→┼─→ rollingBack ──→ rolledBack
                │              │                  └─→ rollbackFailed
                ├─→ timeout    │
                ├─→ canceled ──┘ (回滚)
                ├─→ stopped    (不回滚)
                └─→ paused     (不回滚, 可恢复)
                     │
                     └──→ ResumeTask ──→ running (恢复执行)
```

---

## 四、完整使用示例

### 4.1 定义步骤 Worker

```go
package mytask

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "vane-common/pkg/taskmgr"
)

// ========== Step1: 创建订单 ==========

type CreateOrderStep struct{}

func (s *CreateOrderStep) Execute(ctx context.Context, input taskmgr.StepInput) taskmgr.StepExecResult {
    // 1. 从 payload 解析入参
    var req OrderRequest
    if err := json.Unmarshal(input.Payload, &req); err != nil {
        return taskmgr.StepExecFailed(err)
    }

    // 2. 使用 UpdatePosition 上报进度
    taskmgr.UpdatePosition(ctx, 30)

    // 3. 执行业务逻辑
    orderID, err := createOrderInDB(req)
    if err != nil {
        return taskmgr.StepExecFailed(err)
    }
    taskmgr.UpdatePosition(ctx, 80)

    // 4. 把结果存到 Context，供后续步骤使用
    taskmgr.ContextSet(input.Context, "order_id", orderID)

    // 5. 返回完成
    result, _ := json.Marshal(map[string]string{"order_id": orderID})
    return taskmgr.StepExecCompleted(result)
}

func (s *CreateOrderStep) PollStatus(ctx context.Context, input taskmgr.StepPollInput) taskmgr.StepPollStatusResult {
    // 同步步骤不需要轮询，直接返回完成
    return taskmgr.StepPollCompleted(input.StepResult)
}

func (s *CreateOrderStep) Rollback(ctx context.Context, input taskmgr.StepRollbackInput) error {
    // 从 Context 获取 order_id 并删除订单
    orderID, _ := taskmgr.ContextGet[string](input.Context, "order_id")
    if orderID != "" {
        return deleteOrderFromDB(orderID)
    }
    return nil
}

// ========== Step2: 异步调用支付（演示暂停保存状态 + 停止） ==========

type PaymentStep struct {
    // 可以持有外部依赖，如 http client
}

func (s *PaymentStep) Execute(ctx context.Context, input taskmgr.StepInput) taskmgr.StepExecResult {
    // 解析上一步结果
    var prevResult map[string]string
    json.Unmarshal(input.PreStepResult, &prevResult)
    orderID := prevResult["order_id"]

    // 调用第三方支付接口（异步）
    payReqID, err := submitPayment(orderID)
    if err != nil {
        return taskmgr.StepExecFailed(err)
    }

    // ★ 关键：把异步请求ID存到 Context，暂停恢复时可以用来继续轮询
    taskmgr.ContextSet(input.Context, "pay_request_id", payReqID)

    // 返回 Pending，框架会自动进入 PollStatus 轮询
    result, _ := json.Marshal(map[string]string{"pay_request_id": payReqID})
    return taskmgr.StepExecPending(result)
}

func (s *PaymentStep) PollStatus(ctx context.Context, input taskmgr.StepPollInput) taskmgr.StepPollInput {
    // 从 StepResult 中获取 pay_request_id
    var data map[string]string
    json.Unmarshal(input.StepResult, &data)
    payReqID := data["pay_request_id"]

    // 查询支付状态
    status, err := queryPaymentStatus(payReqID)
    if err != nil {
        return taskmgr.StepPollFailed(err)
    }

    switch status {
    case "success":
        return taskmgr.StepPollCompleted(input.StepResult)
    case "failed":
        return taskmgr.StepPollFailed(fmt.Errorf("payment failed"))
    default:
        // 还在处理中，上报进度
        taskmgr.UpdatePosition(ctx, 50)
        return taskmgr.StepPollPending(50, input.StepResult)
    }
}

func (s *PaymentStep) Rollback(ctx context.Context, input taskmgr.StepRollbackInput) error {
    var data map[string]string
    json.Unmarshal(input.StepResult, &data)
    return refundPayment(data["pay_request_id"])
}

// ★★★ Pause — 暂停时保存状态 ★★★
func (s *PaymentStep) Pause(ctx context.Context, input taskmgr.StepInput) error {
    // 暂停时，关键信息已经通过 ContextSet 保存在 Context 中
    // 框架会自动把 Context 通过 ReportTaskInfo 上报给持久化层
    // 你只需要清理临时资源（如关闭连接等）
    log.Info("payment step paused, context will be persisted automatically")
    return nil
}

// ★★★ Stop — 停止时清理资源 ★★★
func (s *PaymentStep) Stop(ctx context.Context, input taskmgr.StepInput) error {
    // 停止与暂停不同：停止是终态，不再恢复
    // 需要主动取消第三方请求
    payReqID, _ := taskmgr.ContextGet[string](input.Context, "pay_request_id")
    if payReqID != "" {
        cancelPayment(payReqID) // 取消第三方支付请求
    }
    log.Info("payment step stopped, resources cleaned up")
    return nil
}

// 使用 BaseStepWorker 的简写方式（如果不需要 Cancel 回调）
// type PaymentStep struct {
//     taskmgr.BaseStepWorker // 嵌入即可拥有 Pause + Stop 默认实现
// }
// 然后只重写你需要的方法
```

### 4.2 定义任务 Worker（编排步骤）

```go
type OrderTaskWorker struct{}

func (w *OrderTaskWorker) Arrange(ctx context.Context, payload []byte) []taskmgr.StepRegInfo {
    return []taskmgr.StepRegInfo{
        taskmgr.NewStepInfo("create_order", &CreateOrderStep{}),
        taskmgr.NewStepInfo("payment", &PaymentStep{},
            taskmgr.WithPollInterval(3*time.Second), // 自定义轮询间隔
        ),
        taskmgr.NewStepInfo("send_notification", &NotifyStep{}),
    }
}
```

### 4.3 实现外部依赖接口

```go
// ---- 数据提供者：从数据库/消息队列获取待执行任务 ----

type MyDataProvider struct {
    db *sql.DB
}

func (p *MyDataProvider) WatchTask() (taskmgr.TaskReceiveChan, error) {
    ch := make(taskmgr.TaskReceiveChan, 100)
    // 启动 goroutine 监听数据库变更/RabbitMQ/Redis Stream 等
    go func() {
        for {
            task := listenForNewTasks(p.db)
            ch <- &taskmgr.CallTaskReq{
                TaskID:   task.ID,
                TaskCode: task.Code,
            }
        }
    }()
    return ch, nil
}

func (p *MyDataProvider) ListenTask() ([]*taskmgr.CallTaskReq, error) {
    // 拉取模式：定时查询数据库中 waiting 状态的任务
    return queryPendingTasks(p.db)
}

// ---- 状态上报：把任务/步骤状态写入数据库 ----

type MyReporter struct {
    db *sql.DB
}

func (r *MyReporter) ReportTaskInfo(report *taskmgr.TaskReport) error {
    // 将 report 持久化到数据库
    return saveTaskReport(r.db, report)
}

func (r *MyReporter) ReportStepInfo(report *taskmgr.StepReport) error {
    // 将步骤报告持久化到数据库
    return saveStepReport(r.db, report)
}

// ---- 任务信息查询：从数据库恢复任务状态 ----

type MyStepInfoProvider struct {
    db *sql.DB
}

func (p *MyStepInfoProvider) GetTaskStepInfo(taskID string) (*taskmgr.TaskData, error) {
    // 从数据库查询任务及其步骤信息
    return queryTaskData(p.db, taskID)
}

// ---- 分布式锁 ----

type MyLockProvider struct {
    redis *redis.Client
}

func (p *MyLockProvider) Lock(taskID, owner string) error {
    ok, err := p.redis.SetNX(context.Background(),
        "task_lock:"+taskID, owner, 5*time.Minute).Result()
    if !ok {
        return taskmgr.ErrTaskLocked
    }
    return err
}

func (p *MyLockProvider) Unlock(taskID, owner string) error {
    // Lua 脚本保证只有持有者才能解锁
    return p.redis.Eval(context.Background(),
        `if redis.call("get",KEYS[1]) == ARGV[1] then return redis.call("del",KEYS[1]) else return 0 end`,
        []string{"task_lock:" + taskID}, owner).Err()
}

func (p *MyLockProvider) KeepAlive(taskID, owner string) error {
    return p.redis.Expire(context.Background(),
        "task_lock:"+taskID, 5*time.Minute).Err()
}

// ---- 可运行检查 ----

type MyCanRunTask struct{}

func (c *MyCanRunTask) CanRunTask(taskID string) error {
    // 可以检查任务是否被标记为不可运行等
    return nil // 返回 nil 表示可以运行
}
```

### 4.4 启动框架

```go
func main() {
    // 1. 创建外部依赖
    dataProvider := &MyDataProvider{db: db}
    reporter := &MyReporter{db: db}
    stepInfoProvider := &MyStepInfoProvider{db: db}
    lockProvider := &MyLockProvider{redis: rdb}
    canRunTask := &MyCanRunTask{}

    // 2. 创建 TaskManager
    tm := taskmgr.NewTaskManager(
        stepInfoProvider,
        reporter,
        dataProvider,
        lockProvider,
        canRunTask,
        taskmgr.WithTaskPollInterval(5*time.Second),     // 轮询拉取间隔
        taskmgr.WithTaskLockKeepInterval(20*time.Second), // 锁续期间隔
        taskmgr.WithNeedAutoRevert(false),                // 是否禁用轮询拉取
    )

    // 3. 注册任务类型
    tm.RegisterTaskWorker("order_process", &OrderTaskWorker{})

    // 4. 启动
    ctx := context.Background()
    tm.Start(ctx)

    // 5. 优雅关闭
    defer func() {
        tm.Stop()
        tm.Wait()
    }()

    select {} // 阻塞主 goroutine
}
```

### 4.5 控制任务生命周期

```go
// 暂停任务（不回滚，可恢复）
tm.PauseTask("task-123")

// 恢复任务（从暂停点继续执行）
tm.ResumeTask("task-123")

// 停止任务（不回滚，终态）
tm.StopTask("task-123")

// 取消任务（触发回滚）
tm.CancelTask("task-123")

// 通用接口
tm.StopTaskWithMode("task-123", taskmgr.StopModePause)
```

---

## 五、暂停时如何保存状态

### 5.1 核心机制

框架的暂停状态保存依赖三个层面：

```
┌──────────────────────────────────────────────┐
│ 1. ContextMap（内存，自动上报）               │
│    你的 Step 通过 ContextSet/Set 存入的数据   │
│    框架每次 ReportTaskInfo 时会自动带上       │
│    Context → 你的 Reporter 持久化到数据库     │
│                                               │
│ 2. StepData（内存，自动上报）                 │
│    框架在 Process() 中维护每个步骤的:         │
│    - Status, Result, PreStepResult            │
│    - ProcessPosition, Message                 │
│    暂停时通过 ReportStepInfo 上报              │
│                                               │
│ 3. Pause() 回调（你控制）                     │
│    在此方法中保存额外的业务状态               │
│    例如：关闭连接、记录断点位置等              │
└──────────────────────────────────────────────┘
```

### 5.2 最佳实践

```go
type MyStep struct {
    externalConn *SomeConnection // 外部连接
    checkpoint   int             // 业务处理断点
}

func (s *MyStep) Execute(ctx context.Context, input taskmgr.StepInput) taskmgr.StepExecResult {
    for i := s.checkpoint; i < totalItems; i++ {
        // ★ 检查 ctx 是否已被取消（暂停信号）
        select {
        case <-ctx.Done():
            // 虽然框架会处理 ctx 取消，但你可以提前优雅退出
            return taskmgr.StepExecPending(nil)
        default:
        }

        // 处理业务...
        processItem(i)

        // ★ 定期把进度存入 Context
        taskmgr.ContextSet(input.Context, "checkpoint", i)
        taskmgr.UpdatePosition(ctx, (i+1)*100/totalItems)
    }
    return taskmgr.StepExecCompleted(nil)
}

func (s *MyStep) Pause(ctx context.Context, input taskmgr.StepInput) error {
    // 1. Context 中的数据会自动持久化（通过 Reporter）
    //    但你也可以在这里做额外处理

    // 2. 关闭临时资源
    if s.externalConn != nil {
        s.externalConn.Close()
        s.externalConn = nil
    }

    // 3. 确保关键状态已存入 Context
    taskmgr.ContextSet(input.Context, "paused_at", time.Now().Unix())

    return nil
}

func (s *MyStep) PollStatus(ctx context.Context, input taskmgr.StepPollInput) taskmgr.StepPollStatusResult {
    // 恢复时，从 Context 读取断点
    checkpoint, _ := taskmgr.ContextGet[int](input.Context, "checkpoint")
    s.checkpoint = checkpoint

    // 继续处理...
    return taskmgr.StepPollCompleted(nil)
}
```

### 5.3 恢复流程

```
ResumeTask(taskID)
    │
    ├─ 1. 清理 _stop_mode 标记
    ├─ 2. 调用 executeTask()
    ├─ 3. buildProcessor()
    │      ├─ GetTaskStepInfo(taskID)  ← 从数据库获取持久化的 TaskData
    │      └─ engine.Revert()          ← 用 TaskData 恢复步骤状态
    │           ├─ Arrange()           ← 重新编排（获取步骤定义）
    │           ├─ 恢复每个已执行步骤的 Status/Result
    │           └─ 恢复 Context (ContextMap.ParseMap)
    ├─ 4. processor.Process()
    │      ├─ 跳过已完成的步骤
    │      ├─ 从 RunningStepName 对应的步骤继续
    │      └─ 未完成的步骤重新执行
    └─ 5. 完成后上报最终状态
```

---

## 六、实现自定义任务的停止执行

### 6.1 三种停止模式的区别

| 模式 | 方法 | 是否回滚 | 是否可恢复 | 触发回调 |
|------|------|----------|-----------|----------|
| Cancel | `CancelTask()` | 是 | 否 | `StepWorkerCancel` |
| Stop | `StopTask()` | 否 | 否（终态） | `StepWorkerStop` |
| Pause | `PauseTask()` | 否 | 是 | `StepWorkerPause` |

### 6.2 停止流程详解

```
StopTask(taskID)
    │
    ├─ 1. 设置 processor.stopMode = StopModeStop
    ├─ 2. 取消 taskCtx → 触发 ctx.Done()
    │
    └─ TaskProcessor.Process() 中:
         ├─ select { case <-ctx.Done(): ... }
         ├─ handleInterruptedStep()
         │    └─ stopMode == StopModeStop
         │         └─ stopTask() → 调用 StepWorker.Stop() 回调
         ├─ 设置 Status = stopped
         ├─ ReportStepInfo() 上报步骤状态
         ├─ shouldRollback(stopped) → false  ← 不回滚
         └─ ReportTaskInfo() 上报最终状态
```

### 6.3 实现示例

```go
// 方式一：实现 StepWorkerStop 接口
type MyStep struct{}

func (s *MyStep) Execute(ctx context.Context, input taskmgr.StepInput) taskmgr.StepExecResult {
    // 你的执行逻辑...
    return taskmgr.StepExecCompleted(nil)
}

func (s *MyStep) PollStatus(ctx context.Context, input taskmgr.StepPollInput) taskmgr.StepPollStatusResult {
    return taskmgr.StepPollCompleted(input.StepResult)
}

func (s *MyStep) Rollback(ctx context.Context, input taskmgr.StepRollbackInput) error {
    return nil
}

// ★ 实现停止回调
func (s *MyStep) Stop(ctx context.Context, input taskmgr.StepInput) error {
    // 注意：这里的 ctx 是 context.WithoutCancel 的，不会被取消
    // 你可以安全地执行清理操作

    // 清理外部资源
    cleanupExternalResources(input.TaskID)

    // 记录停止原因
    taskmgr.ContextSet(input.Context, "stop_reason", "manual_stop")

    return nil
}

// 方式二：嵌入 BaseStepWorker
type MyStep2 struct {
    taskmgr.BaseStepWorker // 自动包含 Pause + Stop
}

// 只需实现核心的 Execute/PollStatus/Rollback
// Pause 和 Stop 有默认的空实现，可以按需重写
```

### 6.4 在 Execute 中优雅处理停止

```go
func (s *MyStep) Execute(ctx context.Context, input taskmgr.StepInput) taskmgr.StepExecResult {
    for _, item := range items {
        // ★ 每次迭代前检查 ctx
        select {
        case <-ctx.Done():
            // 框架会自动处理，这里不需要返回特殊状态
            // 但你可以先保存中间结果到 Context
            taskmgr.ContextSet(input.Context, "processed_count", processedCount)
            // 返回什么不重要，框架会通过 handleInterruptedStep 接管
            return taskmgr.StepExecFailed(ctx.Err())
        default:
        }

        // 处理业务...
        processedCount++
        taskmgr.UpdatePosition(ctx, processedCount*100/len(items))
    }
    return taskmgr.StepExecCompleted(nil)
}
```

---

## 七、辅助工具函数

### 7.1 进度上报

```go
// 在 Execute 或 PollStatus 中调用，上报 0-100 的进度
taskmgr.UpdatePosition(ctx, 50)

// 上报中间结果
taskmgr.UpdateResult(ctx, []byte("partial result"))
```

### 7.2 Context 操作

```go
// 设置值（线程安全）
taskmgr.ContextSet(input.Context, "key", "value")
taskmgr.ContextSet(input.Context, "count", 42)
taskmgr.ContextSet(input.Context, "config", MyConfig{...})

// 获取值（支持类型自动转换）
str, ok := taskmgr.ContextGet[string](input.Context, "key")
num, ok := taskmgr.ContextGet[int](input.Context, "count")
cfg, ok := taskmgr.ContextGet[MyConfig](input.Context, "config")

// 删除值
taskmgr.ContextDelete(input.Context, "key")

// 转为普通 map（用于序列化）
m := taskmgr.ContextToMap(input.Context)
```

`ContextGet` 支持自动类型转换：
- `string` ↔ `[]byte` 互转
- `[]byte` / `string` → struct（JSON 反序列化）
- 数值类型之间互转（int/int64/uint/float64）
- `map[string]any` → struct（JSON 中转）

---

## 八、步骤结果工厂函数

### Execute 结果

| 函数 | 含义 | 是否触发回滚 |
|------|------|-------------|
| `StepExecCompleted(result)` | 同步完成 | 否 |
| `StepExecCompletedWithError(err, result)` | 完成但有警告错误，不回滚 | 否 |
| `StepExecPending(result)` | 异步执行中，需轮询 | — |
| `StepExecFailed(err)` | 执行失败 | 是（触发回滚） |

### PollStatus 结果

| 函数 | 含义 |
|------|------|
| `StepPollCompleted(result)` | 异步操作完成 |
| `StepPollCompletedWithError(err, result)` | 完成但有错误，不触发回滚 |
| `StepPollPending(position, result)` | 仍在执行中，position 0-100 |
| `StepPollFailed(err)` | 轮询失败，触发回滚 |

---

## 九、步骤选项

```go
// 自定义轮询间隔（默认 500ms）
taskmgr.NewStepInfo("my_step", &MyStep{},
    taskmgr.WithPollInterval(3*time.Second),
)
```

---

## 十、TaskManager 选项

```go
tm := taskmgr.NewTaskManager(
    stepInfoProvider,
    reporter,
    dataProvider,
    lockProvider,
    canRunTask,
    // 任务拉取间隔（默认 5s）
    taskmgr.WithTaskPollInterval(10*time.Second),
    // 锁续期间隔（默认 20s）
    taskmgr.WithTaskLockKeepInterval(30*time.Second),
    // 是否禁用轮询拉取（设为 true 则只用 Watch 推送）
    taskmgr.WithNeedAutoRevert(true),
)
```

---

## 十一、完整生命周期时序图

```
外部系统          TaskManager         TaskEngine        TaskProcessor        StepWorker
  │                  │                   │                   │                   │
  │── WatchTask() ──→│                   │                   │                   │
  │◄── taskChan ─────│                   │                   │                   │
  │                  │                   │                   │                   │
  │              executeTask()           │                   │                   │
  │                  │── Lock() ────────→│                   │                   │
  │                  │── CanRunTask() ──→│                   │                   │
  │                  │── Build() ───────→│                   │                   │
  │                  │                   │── Arrange() ──────│──────────────────→│
  │                  │                   │◄── StepRegInfo[] ─│───────────────────│
  │                  │◄── processor ─────│                   │                   │
  │                  │                   │                   │                   │
  │                  │────────── Process() ─────────────────→│                   │
  │                  │                   │                   │── Execute() ─────→│
  │                  │                   │                   │◄── Result ────────│
  │                  │                   │                   │                   │
  │                  │                   │          (如果 Pending)              │
  │                  │                   │                   │── PollStatus() ──→│
  │                  │                   │                   │◄── PollResult ────│
  │                  │                   │                   │     ...loop...    │
  │                  │                   │                   │                   │
  │                  │◄── ReportTaskInfo() ─────────────────│                   │
  │                  │◄── ReportStepInfo() ─────────────────│                   │
  │                  │                   │                   │                   │
  │                  │            (所有步骤完成)             │                   │
  │                  │◄── final report ────────────────────│                   │
  │                  │── Unlock() ─────→│                   │                   │
```

---

## 十二、注意事项

1. **Arrange 必须幂等**：`Arrange()` 在 Build 和 Revert 时都会被调用，必须根据相同的 payload 返回相同的步骤结构
2. **Context 是自动持久化的**：通过 `ContextSet` 设置的值会在每次 Report 时被上报，你的 Reporter 实现需要将其持久化
3. **Pause 的 ctx**：`Pause()` 接收的 ctx 是 `context.WithoutCancel()` 包装的，可以安全执行清理操作
4. **Stop 的 ctx**：`Stop()` 同样使用 `context.WithoutCancel()` 包装
5. **Cancel 的 ctx**：`Cancel()` 使用原始 ctx（已被取消），注意不要在此做耗时操作
6. **分布式锁超时**：锁续期失败（锁不存在或 owner 不匹配）会自动取消任务
7. **任务过期**：`TaskData.ExpiredAt` 字段可设置任务超时时间，超时自动标记为 timeout
8. **goroutine 安全**：`Execute` 在独立 goroutine 中运行，框架通过 `safe.Try` 捕获 panic
9. **`CreateTaskLog` 必须调用**：调用 `AsyncRunTask` 时必须通过 `task.CreateTaskLog(ctx)` 包装 context，否则任务记录在列表 API 中不可见。详见下方「十三、CreateTaskLog 与任务记录可见性」

---

## 十三、CreateTaskLog 与任务记录可见性

### 13.1 问题描述

调用 `AsyncRunTask` 后，数据库中已创建 TaskRecord，但通过任务列表 API 查询不到该记录，表现为「运行后不生成任务详情」。

### 13.2 根因分析

任务列表接口（`TaskDBService.ListTask`）在查询时固定过滤 `log_tag = 1`：

```go
// task_db.go
func (t *TaskDBService) ListTask(...) {
    conditions := []dataaccess.QueryCondition{
        dataaccess.Eq("log_tag", 1),  // 只返回 log_tag=1 的记录
    }
}
```

而 `log_tag` 的值来源于 `CreateTask` 时从 context 中读取的 `createTaskLog` 值：

```go
// task.go
func CreateTask(ctx context.Context, taskCode string, payload []byte, ...) {
    ctl, _ := ctx.Value(createTaskLog).(int)  // 从 context 取值
    record := &model.TaskRecord{
        LogTag: ctl,  // 未设置则默认为 0
    }
}
```

**只有调用 `task.CreateTaskLog(ctx)` 包装 context，`log_tag` 才为 1，任务记录才对列表 API 可见。**

### 13.3 正确用法 vs 错误用法

```go
import systemtask "tlg-pilot/app/module/system/pkg/task"

// ✅ 正确：包装 context，log_tag=1，任务记录可见
func (a *SomeApi) Run(ctx context.Context) {
    ctx = systemtask.CreateTaskLog(ctx)
    taskmgr.AsyncRunTask(ctx, taskCode, payload)
}

// ❌ 错误：直接传入原始 context，log_tag=0，任务记录被列表 API 过滤掉
func (a *SomeApi) Run(ctx context.Context) {
    taskmgr.AsyncRunTask(ctx, taskCode, payload)
}
```

### 13.4 完整调用示例

参照 `msgpublish` 模块的写法：

```go
// API 层：包装 context
func (a *TgPublishMsgApi) PublishMsgPlan(req *dto.PublishMsgReq) (taskId string, err error) {
    return a.publishMsgService.RunPublishTask(
        task.CreateTaskLog(appruntime.GetCtx()),  // ← 关键：设置 createTaskLog
        req.SrcUser, req.Target, req.Msg,
    )
}

// Service 层：将包装后的 context 透传给 AsyncRunTask
func (s *PublishMsgService) RunPublishTask(ctx context.Context, ...) (taskId string, err error) {
    return taskmgr.AsyncRunTask(ctx, taskcode.TaskCodePublishMsg, reqBytes)
}
```

### 13.5 注意事项

- `CreateTaskLog` 的调用位置应在 **API 层**（或调用 `AsyncRunTask` 之前），确保 context 透传到 `CreateTask`
- 任务的实际执行不受 `log_tag` 影响（DB 监听器会触发所有新建任务的执行），`log_tag` 仅控制 **列表 API 的可见性**
- 如果任务需要在前端展示执行记录，**必须**调用 `CreateTaskLog`
