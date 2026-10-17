package depend

import (
	"context"
	"encoding/json"
	"time"

	"vane-common/pkg/taskmgr"
	"vane-common/trpc"
	"vane-common/v1/pilot/system"
)

// TaskReporter 任务报告者
type TaskReporter struct {
}

func NewTaskReporter() *TaskReporter {
	return &TaskReporter{}
}

func (r *TaskReporter) ReportTaskInfo(report *taskmgr.TaskReport) error {
	ctx := context.Background()
	return Retry(3, 1*time.Second, func() error {
		conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), trpc.CenterServer)
		if err != nil {
			return err
		}

		client := system.NewTaskManagerServiceClient(conn)
		var errMsg string
		if report.Error != nil {
			errMsg = report.Error.Error()
		}
		_, err = client.ReportTaskInfo(ctx, &system.TaskReport{
			TaskId:          report.TaskID,
			TaskCode:        report.TaskCode,
			Status:          string(report.Status),
			Message:         report.Message,
			ProcessPosition: int32(report.ProcessPosition),
			RunningStepName: report.RunningStepName,
			Error:           errMsg,
			Context:         convertContextToProto(report.Context),
			Payload:         report.Payload,
			Result:          report.Result,
		})
		return err
	})
}

func (r *TaskReporter) ReportStepInfo(report *taskmgr.StepReport) error {
	ctx := context.Background()
	return Retry(3, 1*time.Second, func() error {
		conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), trpc.CenterServer)
		if err != nil {
			return err
		}

		client := system.NewTaskManagerServiceClient(conn)
		var errMsg string
		if report.Error != nil {
			errMsg = report.Error.Error()
		}
		_, err = client.ReportStepInfo(ctx, &system.StepReport{
			TaskId:           report.TaskID,
			StepName:         report.StepName,
			Status:           string(report.Status),
			Message:          report.Message,
			Payload:          report.Payload,
			Result:           report.Result,
			Err:              errMsg,
			PollStepInterval: int64(report.PollStepInterval / time.Millisecond),
			ProcessPosition:  int32(report.ProcessPosition),
			PreStepResult:    report.PreStepResult,
		})
		return err
	})
}

// convertContextToProto 将 map[string]any 转换为 []byte (JSON)
func convertContextToProto(ctx map[string]any) []byte {
	if ctx == nil || len(ctx) == 0 {
		return nil
	}
	data, err := json.Marshal(ctx)
	if err != nil {
		return nil
	}
	return data
}
