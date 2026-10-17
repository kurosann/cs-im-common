package depend

import (
	"context"
	"time"

	"vane-common/pkg/logger"
	"vane-common/pkg/safe"
	"vane-common/pkg/taskmgr"
	"vane-common/trpc"
	"vane-common/v1/pilot/system"
)

// TaskDataProvider 任务数据提供者
type TaskDataProvider struct {
}

func NewTaskDataProvider() *TaskDataProvider {
	return &TaskDataProvider{}
}

func (p *TaskDataProvider) WatchTask() (taskmgr.TaskReceiveChan, error) {
	ctx := context.Background()
	ch := make(taskmgr.TaskReceiveChan)
	safe.GoKeep(ctx, 10*time.Second, func() {
		conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), trpc.CenterServer)
		if err != nil {
			logger.Errorf(ctx, "WatchTask get conn error: %v", err)
			return
		}

		client := system.NewTaskManagerServiceClient(conn)

		stream, err := client.WatchTask(ctx, &system.WatchTaskReq{})
		if err != nil {
			logger.Errorf(ctx, "WatchTask create stream error: %v", err)
			return
		}
		for {
			resp, err := stream.Recv()
			if err != nil {
				logger.Errorf(ctx, "WatchTask recv error: %v", err)
				return
			}
			ch <- &taskmgr.CallTaskReq{
				TaskID: resp.TaskId,
			}
		}
	})

	return ch, nil
}

func (p *TaskDataProvider) ListenTask() ([]*taskmgr.CallTaskReq, error) {
	ctx := context.Background()
	var resp *system.ListenTaskResp
	err := Retry(3, 1*time.Second, func() error {
		conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), trpc.CenterServer)
		if err != nil {
			return err
		}

		client := system.NewTaskManagerServiceClient(conn)
		resp, err = client.ListenTask(ctx, &system.ListenTaskReq{})
		return err
	})
	if err != nil {
		return nil, err
	}

	result := make([]*taskmgr.CallTaskReq, len(resp.Tasks))
	for i, task := range resp.Tasks {
		result[i] = &taskmgr.CallTaskReq{
			TaskID: task.TaskId,
		}
	}

	return result, nil
}

var _ taskmgr.ITaskDataProvider = (*TaskDataProvider)(nil)
