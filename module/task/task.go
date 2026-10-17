package task

import (
	"context"
	"vane-common/pkg/task"
)

func RunTask(ctx context.Context, taskCode string, payload []byte) (result []byte, err error) {
	return task.GetGlobalTaskManager().RunTask(ctx, taskCode, payload)
}

func RunTask2(ctx context.Context, taskCode string, payload []byte) (result []byte, taskID string, err error) {
	return task.GetGlobalTaskManager().RunTask2(ctx, taskCode, payload)
}
func AsyncRunTask(ctx context.Context, taskCode string, payload []byte) (taskID string, err error) {
	return task.GetGlobalTaskManager().AsyncRunTask(ctx, taskCode, payload)
}

func QueryTask(taskID string) (result task.TaskExecResult, err error) {
	return task.GetGlobalTaskManager().QueryTask(taskID)
}
