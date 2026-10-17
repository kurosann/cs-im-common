package depend

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"vane-common/pkg/taskmgr"
	"vane-common/trpc"
	"vane-common/v1/pilot/system"
)

// TaskStepInfoProvider 任务步骤信息提供者
type TaskStepInfoProvider struct {
}

func NewTaskStepInfoProvider() *TaskStepInfoProvider {
	return &TaskStepInfoProvider{}
}

func (p *TaskStepInfoProvider) GetTaskStepInfo(taskId string) (*taskmgr.TaskData, error) {
	ctx := context.Background()

	var resp *system.TaskDataResp
	err := Retry(3, 1*time.Second, func() error {
		conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), trpc.CenterServer)
		if err != nil {
			return err
		}

		client := system.NewTaskManagerServiceClient(conn)
		resp, err = client.GetTaskStepInfo(ctx, &system.TaskDataReq{
			TaskId: taskId,
		})
		return err
	})
	if err != nil {
		return nil, err
	}

	// 将 system.TaskDataResp 转换为 taskmgr.TaskData
	return convertToTaskData(resp), nil
}

// convertToTaskData 将 system.TaskDataResp 转换为 taskmgr.TaskData
func convertToTaskData(resp *system.TaskDataResp) *taskmgr.TaskData {
	if resp == nil {
		return nil
	}

	steps := make([]*taskmgr.StepData, len(resp.Steps))
	for i, step := range resp.Steps {
		var stepErr error
		if step.Err != "" {
			stepErr = errors.New(step.Err)
		}
		steps[i] = &taskmgr.StepData{
			TaskID:           resp.TaskId,
			Name:             step.Name,
			Status:           taskmgr.Status(step.Status),
			Err:              stepErr,
			Message:          step.Message,
			Payload:          step.Payload,
			PreStepResult:    step.PreStepResult,
			Result:           step.Result,
			PollStepInterval: time.Duration(step.PollStepInterval) * time.Millisecond,
			ProcessPosition:  int(step.ProcessPosition),
		}
	}

	var expiredAt *time.Time
	if resp.ExpiredAt > 0 {
		t := time.Unix(resp.ExpiredAt, 0)
		expiredAt = &t
	}

	return &taskmgr.TaskData{
		TaskID:          resp.TaskId,
		TaskCode:        resp.TaskCode,
		RunningStepName: resp.RunningStepName,
		Message:         resp.Message,
		Payload:         resp.Payload,
		Status:          taskmgr.Status(resp.Status),
		Steps:           steps,
		Context:         convertContextFromProto(resp.Context),
		ExpiredAt:       expiredAt,
	}
}

// convertContextFromProto 将 []byte (JSON) 转换为 map[string]any
func convertContextFromProto(data []byte) map[string]any {
	if data == nil || len(data) == 0 {
		return nil
	}
	var ctx map[string]any
	if err := json.Unmarshal(data, &ctx); err != nil {
		return nil
	}
	return ctx
}

var _ taskmgr.ITaskStepInfoProvider = (*TaskStepInfoProvider)(nil)
