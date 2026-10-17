package taskmgr

import "context"

// TaskWorker 任务处理器接口
type TaskWorker interface {
	// Arrange 步骤编排
	Arrange(ctx context.Context, payload []byte) (steps []StepRegInfo)
}
