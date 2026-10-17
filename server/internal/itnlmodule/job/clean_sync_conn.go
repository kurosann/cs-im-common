package job

import (
	"context"
	"vane-common/module/job"
	"vane-common/trpc"
)

const CleanSyncConnJobName = "clean_sync_conn"

var _ job.IJobTask = (*CleanSyncConnJob)(nil)

type CleanSyncConnJob struct {
}

func NewCleanSyncConnJob() *CleanSyncConnJob {
	return &CleanSyncConnJob{}
}

func (j *CleanSyncConnJob) Run(ctx context.Context) {
	trpc.CleanSvcConn()
}
