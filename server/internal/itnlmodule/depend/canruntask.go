package depend

import (
	"context"
	"time"

	"vane-common/pkg/taskmgr"
	"vane-common/trpc"
	"vane-common/v1/pilot/system"
)

// CanRunTaskChecker 任务运行检查器
type CanRunTaskChecker struct {
}

func NewCanRunTaskChecker() *CanRunTaskChecker {
	return &CanRunTaskChecker{}
}

func (c *CanRunTaskChecker) CanRunTask(taskId string) error {
	ctx := context.Background()
	return Retry(3, 1*time.Second, func() error {
		conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), trpc.CenterServer)
		if err != nil {
			return err
		}

		client := system.NewTaskManagerServiceClient(conn)
		_, err = client.CanRunTask(ctx, &system.CanRunTaskReq{
			TaskId: taskId,
		})
		return err
	})
}

var _ taskmgr.ICanRunTask = (*CanRunTaskChecker)(nil)
