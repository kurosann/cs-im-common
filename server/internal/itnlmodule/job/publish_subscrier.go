package job

import (
	"context"
	"vane-common/module/job"
	"vane-common/publish"

	"vane-common/pkg/logger"

	"go.uber.org/zap"
)

const (
	PublishSubcriberJobName = "publish_subscrier_sync"
)

var _ job.IJobTask = (*PublishSubcriberJob)(nil)

type PublishSubcriberJob struct {
}

func NewPublishSubcriberJob() *PublishSubcriberJob {
	return &PublishSubcriberJob{}
}

func (j *PublishSubcriberJob) Run(ctx context.Context) {
	if err := publish.SyncSubcribe(ctx); err != nil {
		logger.Error(ctx, "sync publish subscrier failed", zap.Error(err))
	}
}
