package job

import (
	"context"
	"vane-common/module/job"
	"vane-common/trpc"
)

const (
	SyncSvcmapJobName = "sync_svcmap"
)

var _ job.IJobTask = (*SyncSvcmapJob)(nil)

type SyncSvcmapJob struct {
}

func NewSyncSvcmapJob() *SyncSvcmapJob {
	return &SyncSvcmapJob{}
}

func (s *SyncSvcmapJob) Run(ctx context.Context) {
	trpc.SyncDynamicSvcmap(ctx)
}
