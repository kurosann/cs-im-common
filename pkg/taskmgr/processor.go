package taskmgr

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"vane-common/pkg/logger"
	"vane-common/pkg/safe"

	"go.uber.org/zap"
)

type taskReportMsg struct {
	report *TaskReport
}

type stepReportMsg struct {
	stepData *StepData
}

type TaskProcessor struct {
	ctx            context.Context
	TaskRegData    TaskRegInfo
	Reporter       ITaskReporter
	TaskWorker     TaskWorker         // 保存 TaskWorker 引用，用于恢复上下文
	stepProgress   map[string]int     // 跟踪每个step的进度 0-100
	stepProgressMu sync.RWMutex       // 保护stepProgress的并发访问
	stopMode       StopMode           // 停止模式
	taskReportChan chan taskReportMsg // 任务报告channel
	stepReportChan chan stepReportMsg // 步骤报告channel
	reportDone     chan struct{}      // 报告协程退出通知
}

func (tp *TaskProcessor) updateStopStatus(mode StopMode) error {
	if tp == nil {
		return nil
	}

	tp.stopMode = mode
	if tp.TaskRegData.TaskData == nil {
		return nil
	}

	tp.TaskRegData.TaskData.Status = stopModeToTaskStatus(mode)
	if tp.Reporter == nil {
		return nil
	}

	return tp.reportTaskInfo(nil, nil)
}

func (tp *TaskProcessor) Cancel() error {
	return tp.updateStopStatus(StopModeCancel)
}

func (tp *TaskProcessor) Stop() error {
	return tp.updateStopStatus(StopModeStop)
}

func (tp *TaskProcessor) Pause() error {
	return tp.updateStopStatus(StopModePause)
}

// calculateTaskProgress 计算task的整体进度
func (tp *TaskProcessor) calculateTaskProgress() int {
	tp.stepProgressMu.RLock()
	defer tp.stepProgressMu.RUnlock()

	totalSteps := len(tp.TaskRegData.Steps)
	if totalSteps == 0 {
		return 0
	}

	stepWeight := 100 / totalSteps // 每个step的权重
	totalProgress := 0

	// 计算所有step的进度总和
	for _, step := range tp.TaskRegData.Steps {
		stepName := step.StepData.Name
		progress, exists := tp.stepProgress[stepName]
		if !exists {
			progress = 0
		}
		totalProgress += progress * stepWeight / 100
	}

	// 如果大于100，则返回100
	if totalProgress > 100 {
		return 100
	}
	return totalProgress
}

type executedStep struct {
	StepRegInfo StepRegInfo
	ExecResult  StepExecResult
}

// updateStepProgress 更新指定step的进度
func (tp *TaskProcessor) updateStepProgress(stepName string, progress int) {
	tp.stepProgressMu.Lock()
	defer tp.stepProgressMu.Unlock()

	// 确保进度在0-100范围内
	if progress < 0 {
		progress = 0
	} else if progress > 100 {
		progress = 100
	}
	tp.stepProgress[stepName] = progress
}

func (tp *TaskProcessor) handleInterruptedStep(ctx context.Context, step StepRegInfo, stepResult []byte) (Status, error) {
	finalErr := context.Cause(ctx)
	ctx = context.WithoutCancel(ctx)
	stepInput := StepInput{
		TaskID:        tp.TaskRegData.TaskData.TaskID,
		Payload:       tp.TaskRegData.TaskData.Payload,
		PreStepResult: stepResult,
		Context:       tp.TaskRegData.Context,
	}

	switch tp.stopMode {
	case StopModeStop:
		tp.stopTask(ctx, step.StepWorker, stepInput)
		return StatusStopped, finalErr
	case StopModePause:
		tp.pauseTask(ctx, step.StepWorker, stepInput)
		return StatusPaused, finalErr
	case StopModeCancel:
		fallthrough
	default:
		status := StatusCanceled
		if errors.Is(finalErr, context.DeadlineExceeded) {
			status = StatusTimeout
		}
		tp.cancelTask(ctx, step.StepWorker, stepInput)
		return status, finalErr
	}
}

func (tp *TaskProcessor) Process(ctx context.Context) error {
	tp.ctx = ctx
	// 初始化停止模式为默认值
	if tp.stopMode == "" {
		tp.stopMode = StopModeCancel
	}
	// 初始化step进度跟踪
	tp.stepProgressMu.Lock()
	if tp.stepProgress == nil {
		tp.stepProgress = make(map[string]int, len(tp.TaskRegData.Steps))
	}
	// 初始化所有step的进度为0
	for _, step := range tp.TaskRegData.Steps {
		tp.stepProgress[step.StepData.Name] = 0
	}
	tp.stepProgressMu.Unlock()

	// 初始化异步报告channel
	tp.taskReportChan = make(chan taskReportMsg, 50)
	tp.stepReportChan = make(chan stepReportMsg, 100)
	tp.reportDone = make(chan struct{})

	safe.Go(func() {
		defer close(tp.reportDone)
		tp.reportWorker()
	})

	tp.TaskRegData.TaskData.Status = StatusRunning
	// 异步发送报告
	select {
	case tp.taskReportChan <- taskReportMsg{report: tp.buildTaskReport(nil, nil)}:
	default:
		logger.Warn(ctx, "[TaskProcessor] Task report channel full")
	}

	executedSteps := make([]executedStep, 0)
	stepResult := make([]byte, 0)
	var finalErr error
	finalStatus := StatusCompleted

	defer func() {
		// 根据停止模式决定是否执行回滚
		shouldRollback := tp.shouldRollback(finalStatus)

		if shouldRollback {
			tp.executeRollback(ctx, executedSteps, &finalErr)
		}

		// 根据上下文错误和停止模式设置最终状态
		if ctx.Err() != nil {
			switch tp.stopMode {
			case StopModeStop:
				finalStatus = StatusStopped
			case StopModePause:
				finalStatus = StatusPaused
			case StopModeCancel:
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					finalStatus = StatusTimeout
				} else {
					finalStatus = StatusCanceled
				}
			default:
				if errors.Is(ctx.Err(), context.DeadlineExceeded) {
					finalStatus = StatusTimeout
				} else {
					finalStatus = StatusCanceled
				}
			}
		}

		// 报告最终任务状态
		var taskResult []byte
		if len(executedSteps) > 0 {
			taskResult = executedSteps[len(executedSteps)-1].ExecResult.Result
		}

		tp.TaskRegData.TaskData.Status = finalStatus
		switch finalStatus {
		case StatusCompleted:
			// 完成时，确保所有已完成的step进度都是100
			for _, step := range executedSteps {
				if step.ExecResult.Status == StatusCompleted {
					tp.updateStepProgress(step.StepRegInfo.StepData.Name, 100)
				}
			}
		}

		// 异步发送最终报告
		tp.asyncReportTask(taskResult, finalErr)
		close(tp.taskReportChan)
		close(tp.stepReportChan)
		<-tp.reportDone
	}()
	isRevert := tp.TaskRegData.TaskData.RunningStepName != ""
	steps := tp.TaskRegData.Steps

	// 判断是否为恢复流程需要跳过已经执行过的步骤
	if isRevert {
	loop:
		for i, step := range steps {
			if step.StepData.Name == tp.TaskRegData.TaskData.RunningStepName {
				var remainStep []StepRegInfo
				switch step.StepData.Status {
				case StatusRunning, StatusPending:
					steps = steps[i:]
					break loop
				case StatusPaused: // 暂停状态需要从当前步骤继续执行
					steps = steps[i:]
					break loop
				case StatusCompleted:
					steps = steps[i+1:]
					break loop
				case StatusFailed, StatusCanceled, StatusTimeout:
					finalErr = step.StepData.Err
					if finalErr == nil {
						finalErr = fmt.Errorf("step %s ended with status %s", step.StepData.Name, step.StepData.Status)
					}
					finalStatus = step.StepData.Status
					stepResult = step.StepData.Result

					for _, completedStep := range steps[:i] {
						if completedStep.StepData.Status != StatusCompleted {
							continue
						}
						tp.updateStepProgress(completedStep.StepData.Name, 100)
						executedSteps = append(executedSteps, executedStep{
							StepRegInfo: completedStep,
							ExecResult: StepExecResult{
								Status:        completedStep.StepData.Status,
								Payload:       completedStep.StepData.Payload,
								PreStepResult: completedStep.StepData.PreStepResult,
								Result:        completedStep.StepData.Result,
							},
						})
					}
					return finalErr
				case StatusRollingBack: // 进入回滚流程
					remainStep = steps[i+1:]
					steps = steps[0:i]
				case StatusRolledBack, StatusRollbackFailed: // 进入回滚流程
					remainStep = steps[i:]
					steps = steps[0 : i-1]
				}
				// 倒序找到最后执行失败一步的执行结果
				for i := len(remainStep) - 1; i >= 0; i-- {
					step := remainStep[i]
					if step.StepData.Status == StatusFailed ||
						step.StepData.Status == StatusTimeout ||
						step.StepData.Status == StatusCanceled {
						finalErr = step.StepData.Err
						finalStatus = step.StepData.Status
						stepResult = step.StepData.Result
					}
					// 找出所有的错误，并加入到finalErr中
					if step.StepData.Status == StatusRollbackFailed {
						finalErr = errors.Join(finalErr, step.StepData.Err)
					}
					if step.StepData.Status == StatusRollingBack {
						break
					}
				}
				// 初始化已完成的step的进度为100
				for _, step := range steps {
					if step.StepData.Status == StatusCompleted {
						tp.updateStepProgress(step.StepData.Name, 100)
					}
					executedSteps = append(executedSteps, executedStep{
						StepRegInfo: step,
						ExecResult: StepExecResult{
							Status:        step.StepData.Status,
							Payload:       step.StepData.Payload,
							PreStepResult: step.StepData.PreStepResult,
							Result:        step.StepData.Result,
						},
					})
				}
				return finalErr
			}
		}
	}
	tp.TaskRegData.TaskData.Status = StatusRunning
	for _, step := range steps {
		tp.TaskRegData.TaskData.RunningStepName = step.StepData.Name
		tp.asyncReportTask(nil, nil)
		stepData := step.StepData
		select {
		case <-ctx.Done():
			finalStatus, finalErr = tp.handleInterruptedStep(ctx, step, stepResult)
			stepData.Status = finalStatus
			executedSteps = append(executedSteps, executedStep{
				StepRegInfo: step,
				ExecResult: StepExecResult{
					Status:          stepData.Status,
					ProcessPosition: stepData.ProcessPosition,
					Message:         stepData.Message,
					Err:             stepData.Err,
					Payload:         stepData.Payload,
					PreStepResult:   stepData.PreStepResult,
					Result:          stepData.Result,
				},
			})
			return finalErr
		default:

			// 跳过已完成的步骤，使用其结果作为下一步的输入
			if step.StepData.Status == StatusCompleted {
				if len(step.StepData.Result) > 0 {
					stepResult = step.StepData.Result
				}
				// 已完成步骤的进度应该是100
				tp.updateStepProgress(step.StepData.Name, 100)
				continue
			}

			if step.StepData.Status == StatusWaiting || step.StepData.Status == "" {
				step.StepData.Status = StatusRunning
			}

			stepData.Status = StatusRunning
			stepData.PreStepResult = stepResult
			tp.asyncReportStep(stepData)

			execDone := make(chan StepExecResult, 1)
			safe.Go(func() {
				defer close(execDone)
				pos := 0
				// 给ctx 添加一个能力 可以使用ctx 获取一个函数 使用函数去更新position然后report
				updatePositionFunc := func(position int) {
					pos = position
					// 更新step进度
					tp.updateStepProgress(stepData.Name, pos)
					stepData.ProcessPosition = pos
					tp.asyncReportStep(stepData)
					// 更新task进度
					tp.asyncReportTask(nil, nil)
				}
				updateResultFunc := func(result []byte) {
					tp.updateStepProgress(stepData.Name, pos)
					stepData.ProcessPosition = pos
					stepData.Result = result
					// 更新step进度
					tp.asyncReportStep(stepData)
					// 更新task进度
					tp.asyncReportTask(result, nil)
				}

				ctx = genUpdatePositionCtx(ctx, updatePositionFunc)
				ctx = genUpdateResultCtx(ctx, updateResultFunc)
				var stepExecResult StepExecResult
				if err := safe.Try(func() {
					stepExecResult = step.StepWorker.Execute(ctx, StepInput{
						TaskID:        tp.TaskRegData.TaskData.TaskID,
						Payload:       tp.TaskRegData.TaskData.Payload,
						PreStepResult: stepResult,
						Context:       tp.TaskRegData.Context,
					})
				}); err != nil {
					logger.Error(ctx, "[TaskProcessor] Execute", zap.Error(err))
					stepExecResult = StepExecFailed(err)
				}
				execDone <- stepExecResult
			})

			var stepExecResult StepExecResult
			select {
			case <-ctx.Done():
				finalStatus, finalErr = tp.handleInterruptedStep(ctx, step, stepResult)
				stepData.Status = finalStatus
				stepData.PreStepResult = stepResult
				stepData.Err = finalErr
				// 超时/取消时保持当前进度，不更新为100
				tp.asyncReportStep(stepData)
				executedSteps = append(executedSteps, executedStep{
					StepRegInfo: step,
					ExecResult: StepExecResult{
						Status:          stepData.Status,
						ProcessPosition: stepData.ProcessPosition,
						Message:         stepData.Message,
						Err:             stepData.Err,
						Payload:         stepData.Payload,
						PreStepResult:   stepData.PreStepResult,
						Result:          stepData.Result,
					},
				})
				return finalErr
			case stepExecResult = <-execDone:
			}

			if ctxErr := context.Cause(ctx); ctxErr != nil {
				finalStatus, finalErr = tp.handleInterruptedStep(ctx, step, stepResult)
				stepData.Status = finalStatus
				stepData.PreStepResult = stepResult
				stepData.Err = finalErr
				tp.asyncReportStep(stepData)
				executedSteps = append(executedSteps, executedStep{
					StepRegInfo: step,
					ExecResult: StepExecResult{
						Status:          stepData.Status,
						ProcessPosition: stepData.ProcessPosition,
						Message:         stepData.Message,
						Err:             stepData.Err,
						Payload:         stepData.Payload,
						PreStepResult:   stepData.PreStepResult,
						Result:          stepData.Result,
					},
				})
				return finalErr
			}

			if stepExecResult.Status == StatusFailed {
				finalErr = stepExecResult.Err
				finalStatus = StatusFailed
				stepData.Status = StatusFailed
				// 失败时保持当前进度（如果有的话），不更新为100
				if stepExecResult.ProcessPosition > 0 {
					tp.updateStepProgress(stepData.Name, stepExecResult.ProcessPosition)
				}
				stepData.PreStepResult = stepResult
				stepData.Err = finalErr
				stepData.Result = stepExecResult.Result
				stepData.ProcessPosition = stepExecResult.ProcessPosition
				tp.asyncReportStep(stepData)
				// 失败的步骤不需要添加到 executedSteps 中进行回滚
				return finalErr
			}

			// 异步步骤需要轮询状态
			if stepExecResult.Status == StatusPending {
				stepData.Status = StatusPending
				stepData.PreStepResult = stepResult
				stepData.Result = stepExecResult.Result
				// 更新step进度
				tp.updateStepProgress(stepData.Name, stepExecResult.ProcessPosition)
				tp.asyncReportStep(stepData)
				tp.asyncReportTask(stepExecResult.Result, nil)

				// 轮询异步步骤的状态直到完成或失败
				pollResult, err := tp.pollStatus(ctx, step, stepExecResult.Result)
				if pollResult.Status.IsFailed() {
					finalErr = err
					if errors.Is(err, context.DeadlineExceeded) || pollResult.Status == StatusTimeout {
						finalStatus = StatusTimeout
					} else {
						finalStatus = pollResult.Status
					}
					logger.Error(ctx, "[TaskProcessor] pollStatus", zap.Error(err))
					return finalErr
				}
				stepExecResult.Status = pollResult.Status
				stepExecResult.Result = pollResult.Result
				stepExecResult.Err = pollResult.Err
				stepExecResult.ProcessPosition = pollResult.ProcessPosition
				stepExecResult.Message = pollResult.Message
				// 更新step进度
				tp.updateStepProgress(stepData.Name, pollResult.ProcessPosition)
				if after, ok := step.StepWorker.(StepPullStatusAfter); ok {
					if err = safe.TryE(func() error {
						return after.After(ctx, StepPollAfterInput{
							StepPollInput: StepPollInput{
								TaskID:     tp.TaskRegData.TaskData.TaskID,
								Payload:    tp.TaskRegData.TaskData.Payload,
								StepResult: stepExecResult.Result,
								Context:    tp.TaskRegData.Context,
							},
							AfterResult: stepExecResult,
						})
					}); err != nil {
						logger.Error(ctx, "[TaskProcessor] StepPullStatusAfter", zap.Error(err))
						finalErr = err
						finalStatus = StatusFailed
						return finalErr
					}
				}
			}

			stepResult = stepExecResult.Result
			// 只有成功完成的步骤才添加到 executedSteps 中，失败的步骤不需要回滚
			if stepExecResult.Status == StatusCompleted {
				executedSteps = append(executedSteps, executedStep{
					StepRegInfo: step,
					ExecResult:  stepExecResult,
				})
				// 完成但有错误时，传递错误给上层任务判断，不回滚
				if stepExecResult.Err != nil {
					finalErr = errors.Join(finalErr, stepExecResult.Err)
				}
			}

			step.StepData.Status = stepExecResult.Status
			step.StepData.Result = stepExecResult.Result
			stepData.Status = stepExecResult.Status
			// 更新step进度（完成时应该是100，失败时保持当前进度）
			if stepExecResult.Status == StatusCompleted {
				tp.updateStepProgress(stepData.Name, 100)
			} else if stepExecResult.ProcessPosition > 0 {
				tp.updateStepProgress(stepData.Name, stepExecResult.ProcessPosition)
			}

			step.StepData.Status = stepExecResult.Status
			step.StepData.ProcessPosition = stepExecResult.ProcessPosition
			step.StepData.Message = stepExecResult.Message
			step.StepData.Result = stepExecResult.Result
			step.StepData.Err = stepExecResult.Err
			tp.asyncReportStep(step.StepData)
			tp.asyncReportTask(stepExecResult.Result, stepExecResult.Err)
		}
	}
	return finalErr
}

func (tp *TaskProcessor) pauseTask(ctx context.Context, stepWorker StepWorker, stepResult StepInput) {
	if pauseWorker, ok := stepWorker.(StepWorkerPause); ok {
		if err := safe.TryE(func() error { return pauseWorker.Pause(context.WithoutCancel(ctx), stepResult) }); err != nil {
			logger.Error(ctx, "[TaskProcessor] StepWorker.Pause", zap.Error(err))
		}
	}
}
func (tp *TaskProcessor) stopTask(ctx context.Context, stepWorker StepWorker, stepResult StepInput) {
	if stopWorker, ok := stepWorker.(StepWorkerStop); ok {
		if err := safe.TryE(func() error { return stopWorker.Stop(context.WithoutCancel(ctx), stepResult) }); err != nil {
			logger.Error(ctx, "[TaskProcessor] StepWorker.Pause", zap.Error(err))
		}
	}
}
func (tp *TaskProcessor) cancelTask(ctx context.Context, stepWorker StepWorker, stepResult StepInput) {
	if cancelWorker, ok := stepWorker.(StepWorkerCancel); ok {
		if err := safe.TryE(func() error { return cancelWorker.Cancel(ctx, stepResult) }); err != nil {
			logger.Error(ctx, "[TaskProcessor] StepWorker.Pause", zap.Error(err))
		}
	}
}

// shouldRollback 根据最终状态和停止模式判断是否需要回滚
func (tp *TaskProcessor) shouldRollback(finalStatus Status) bool {
	// 不需要回滚的情况：
	// 1. 任务正常完成
	// 2. 任务被停止（Stop模式）
	// 3. 任务被暂停（Pause模式）
	// 4. 任务超时
	shouldNotRollback := finalStatus == StatusCompleted ||
		finalStatus == StatusStopped ||
		finalStatus == StatusPaused ||
		finalStatus == StatusTimeout

	return !shouldNotRollback
}

// executeRollback 执行回滚操作
func (tp *TaskProcessor) executeRollback(ctx context.Context, executedSteps []executedStep, finalErr *error) {
	rollbackErrs := make([]error, 0)

	for i := len(executedSteps) - 1; i >= 0; i-- {
		step := executedSteps[i]

		stepData := step.StepRegInfo.StepData
		stepData.Status = StatusRollingBack
		stepData.Result = step.ExecResult.Result
		tp.asyncReportStep(stepData)

		rollbackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 1*time.Minute)
		if err := safe.TryE(func() error {
			// 执行回滚操作
			return step.StepRegInfo.StepWorker.Rollback(rollbackCtx, StepRollbackInput{
				TaskID:     tp.TaskRegData.TaskData.TaskID,
				Payload:    tp.TaskRegData.TaskData.Payload,
				StepResult: step.ExecResult.Result,
				Context:    tp.TaskRegData.Context,
			})
		}); err != nil {
			cancel()
			logger.Error(ctx, "[TaskProcessor] Rollback failed", zap.Error(err))
			rollbackErrs = append(rollbackErrs, err)
			stepData.Status = StatusRollbackFailed
			stepData.Err = err
			stepData.Result = step.ExecResult.Result
			tp.asyncReportStep(stepData)
		} else {
			cancel()
			stepData.Status = StatusRolledBack
			stepData.Err = nil
			stepData.Result = step.ExecResult.Result
			tp.asyncReportStep(stepData)
		}

	}

	if len(rollbackErrs) > 0 {
		*finalErr = errors.Join(append([]error{*finalErr}, rollbackErrs...)...)
	}
}

func (tp *TaskProcessor) reportTaskInfo(result []byte, err error) error {
	defer func() {
		if r := recover(); r != nil {
			logger.Error(context.Background(), "[TaskProcessor] internalReportTask failed", zap.Any("panic", r))
		}
	}()
	return tp.Reporter.ReportTaskInfo(tp.buildTaskReport(result, err))
}

func (tp *TaskProcessor) buildTaskReport(result []byte, err error) *TaskReport {
	if result != nil {
		tp.TaskRegData.TaskData.Result = result
	}

	ctx := tp.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	if !isFinishedStatus(tp.TaskRegData.TaskData.Status) {
		if ctxErr := context.Cause(ctx); ctxErr != nil {
			switch tp.stopMode {
			case StopModeStop:
				tp.TaskRegData.TaskData.Status = StatusStopped
			case StopModePause:
				tp.TaskRegData.TaskData.Status = StatusPaused
			case StopModeCancel:
				if errors.Is(ctxErr, context.DeadlineExceeded) {
					tp.TaskRegData.TaskData.Status = StatusTimeout
				} else {
					tp.TaskRegData.TaskData.Status = StatusCanceled
				}
			default:
				if errors.Is(ctxErr, context.DeadlineExceeded) {
					tp.TaskRegData.TaskData.Status = StatusTimeout
				} else {
					tp.TaskRegData.TaskData.Status = StatusCanceled
				}
			}
		}
	}

	pos := 0
	if tp.TaskRegData.TaskData.Status == StatusCompleted {
		pos = 100
	} else {
		pos = tp.calculateTaskProgress()
	}
	return &TaskReport{
		TaskID:          tp.TaskRegData.TaskData.TaskID,
		TaskCode:        tp.TaskRegData.TaskData.TaskCode,
		Status:          tp.TaskRegData.TaskData.Status,
		RunningStepName: tp.TaskRegData.TaskData.RunningStepName,
		ProcessPosition: pos,
		Message:         tp.TaskRegData.TaskData.Message,
		Context:         ContextToMap(tp.TaskRegData.Context),
		Payload:         tp.TaskRegData.TaskData.Payload,
		Result:          tp.TaskRegData.TaskData.Result,
		Error:           err,
	}
}

func (tp *TaskProcessor) reportStepInfo(stepData *StepData) error {
	defer func() {
		if r := recover(); r != nil {
			logger.Error(context.Background(), "[TaskProcessor] reportStepInfo failed", zap.Any("panic", r))
		}
	}()
	return tp.Reporter.ReportStepInfo(&StepReport{
		TaskID:           tp.TaskRegData.TaskData.TaskID,
		StepName:         stepData.Name,
		Status:           stepData.Status,
		PollStepInterval: stepData.PollStepInterval,
		ProcessPosition:  stepData.ProcessPosition,
		Message:          stepData.Message,
		Error:            stepData.Err,
		Payload:          tp.TaskRegData.TaskData.Payload,
		PreStepResult:    stepData.PreStepResult,
		Result:           stepData.Result,
	})
}

// asyncReportTask 异步发送任务报告
func (tp *TaskProcessor) asyncReportTask(result []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			if err = tp.reportTaskInfo(result, err); err != nil {
				logger.Error(context.Background(), "[TaskProcessor] reportTaskInfo failed", zap.Error(err))
			}
		}
	}()
	tp.taskReportChan <- taskReportMsg{report: tp.buildTaskReport(result, err)}
}

// asyncReportStep 异步发送步骤报告
func (tp *TaskProcessor) asyncReportStep(stepData *StepData) {
	stepDataCopy := *stepData
	defer func() {
		if r := recover(); r != nil {
			if err := tp.reportStepInfo(&stepDataCopy); err != nil {
				logger.Error(context.Background(), "[TaskProcessor] reportStepInfo failed",
					zap.String("step", stepDataCopy.Name), zap.Error(err))
			}
		}
	}()
	tp.stepReportChan <- stepReportMsg{stepData: &stepDataCopy}
}

// taskReportWorker 任务报告异步工作协程
func (tp *TaskProcessor) reportWorker() {
	taskReportChan := tp.taskReportChan
	stepReportChan := tp.stepReportChan

	for {
		if taskReportChan == nil && stepReportChan == nil {
			return
		}

		select {
		case msg, ok := <-taskReportChan:
			if !ok {
				taskReportChan = nil
				continue
			}
			if err := tp.internalReportTask(msg.report); err != nil {
				logger.Error(context.Background(), "[TaskProcessor] reportTaskInfo failed", zap.Error(err))
			}
		case msg, ok := <-stepReportChan:
			if !ok {
				stepReportChan = nil
				continue
			}
			if err := tp.reportStepInfo(msg.stepData); err != nil {
				logger.Error(context.Background(), "[TaskProcessor] reportStepInfo failed",
					zap.String("step", msg.stepData.Name), zap.Error(err))
			}
		}
	}
}
func (tp *TaskProcessor) internalReportTask(report *TaskReport) error {
	defer func() {
		if r := recover(); r != nil {
			logger.Error(context.Background(), "[TaskProcessor] internalReportTask failed", zap.Any("panic", r))
		}
	}()
	return tp.Reporter.ReportTaskInfo(report)
}

func (tp *TaskProcessor) pollStatus(ctx context.Context, step StepRegInfo, stepResult []byte) (StepExecResult, error) {
	interval := step.StepData.PollStepInterval
	if interval == 0 {
		interval = 500 * time.Millisecond
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			status, finalErr := tp.handleInterruptedStep(ctx, step, stepResult)
			return StepExecResult{
				Status:  status,
				Message: "interrupted",
				Err:     finalErr,
			}, finalErr
		case <-ticker.C:
			var pollResult StepPollStatusResult
			if err := safe.Try(func() {
				pollResult = step.StepWorker.PollStatus(ctx, StepPollInput{
					TaskID:     tp.TaskRegData.TaskData.TaskID,
					Payload:    tp.TaskRegData.TaskData.Payload,
					StepResult: stepResult,
					Context:    tp.TaskRegData.Context,
				})
			}); err != nil {
				logger.Error(ctx, "[TaskProcessor] PollStatus panic", zap.Error(err))
				pollResult = StepPollFailed(err)
			}

			if ctxErr := context.Cause(ctx); ctxErr != nil {
				status, finalErr := tp.handleInterruptedStep(ctx, step, stepResult)
				return StepExecResult{
					Status:  status,
					Message: "interrupted",
					Err:     finalErr,
				}, finalErr
			}

			if pollResult.Status == StatusPending {
				stepData := step.StepData
				stepData.Status = StatusPending
				if pollResult.Result != nil {
					stepData.Result = pollResult.Result
				}
				// 更新step进度
				tp.updateStepProgress(stepData.Name, pollResult.ProcessPosition)
				tp.TaskRegData.TaskData.Message = pollResult.ProcessMessage
				tp.asyncReportTask(stepData.Result, nil)
				stepData.ProcessPosition = pollResult.ProcessPosition
				stepData.Message = pollResult.ProcessMessage
				tp.asyncReportStep(stepData)
				continue
			}

			// 更新step进度（完成时应该是100）
			if pollResult.Status == StatusCompleted {
				tp.updateStepProgress(step.StepData.Name, 100)
			} else {
				tp.updateStepProgress(step.StepData.Name, pollResult.ProcessPosition)
			}
			return StepExecResult{
				Status:          pollResult.Status,
				ProcessPosition: pollResult.ProcessPosition,
				Message:         pollResult.ProcessMessage,
				Result:          pollResult.Result,
				Err:             pollResult.Err,
			}, pollResult.Err
		}
	}
}
