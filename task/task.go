package task

import (
	"context"
	"errors"
	"vane-common/v1/pilot/system"

	"vane-common/module/task"
	internalTask "vane-common/pkg/task"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type TaskApi struct {
	system.UnimplementedTaskServiceServer
}

func NewTaskApi() *TaskApi {
	return &TaskApi{}
}

func (t *TaskApi) RunTask(ctx context.Context, req *system.RunTaskReq) (*system.RunTaskResp, error) {
	result, taskID, err := task.RunTask2(ctx, req.TaskCode, req.Payload)
	if err != nil {
		if errors.Is(err, internalTask.ErrTaskNotFound) {
			return nil, status.Error(codes.NotFound, "wallet not found")
		}
		return nil, status.Error(codes.Internal, "wallet failed")
	}
	return &system.RunTaskResp{Result: result, TaskId: taskID}, nil
}

func (t *TaskApi) QueryTask(ctx context.Context, req *system.QueryTaskReq) (*system.QueryTaskResp, error) {
	result, err := task.QueryTask(req.TaskId)
	if err != nil {
		return nil, err
	}
	return &system.QueryTaskResp{
		Status:          t.state2Pb(result.Status),
		Order:           int64(result.Order),
		StepName:        result.StepName,
		TaskId:          result.TaskID,
		StepId:          result.StepID,
		ProcessPosition: int64(result.ProcessPosition),
		Err:             result.Err.Error(),
		Payload:         result.Payload,
		Result:          result.Result,
	}, nil
}
func (t *TaskApi) state2Pb(state internalTask.Status) system.TaskStatus {
	switch state {
	case internalTask.StatusPending:
		return system.TaskStatus_TaskStatusPending
	case internalTask.StatusRunning:
		return system.TaskStatus_TaskStatusRunning
	case internalTask.StatusCompleted:
		return system.TaskStatus_TaskStatusCompleted
	case internalTask.StatusRollingBack:
		return system.TaskStatus_TaskStatusRollingBack
	case internalTask.StatusRolledBack:
		return system.TaskStatus_TaskStatusRolledBack
	case internalTask.StatusFailed:
		return system.TaskStatus_TaskStatusFailed
	case internalTask.StatusTimeout:
		return system.TaskStatus_TaskStatusTimeout
	case internalTask.StatusCanceled:
		return system.TaskStatus_TaskStatusCanceled
	default:
		return system.TaskStatus_TaskStatusUnknown
	}
}
