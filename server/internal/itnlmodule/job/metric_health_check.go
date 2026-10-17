package job

import (
	"context"
	"vane-common/module/job"
)

const (
	MetricHealthCheckJobName = "metric_health_check"
)

var _ job.IJobTask = (*MetricHealthCheckJob)(nil)

type MetricHealthCheckJob struct {
	worker func()
}

func NewMetricHealthCheckJob(worker func()) *MetricHealthCheckJob {
	return &MetricHealthCheckJob{
		worker: worker,
	}
}
func (h *MetricHealthCheckJob) Run(ctx context.Context) {
	h.worker()
}
