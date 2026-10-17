package statemgr

import (
	"context"

	"vane-common/module/job"
)

func RegisterFlush(name string, worker job.IJobTask) {
	job.RegisterFlush(name, worker)
}

func GetJob(name string) job.IJobTask {
	return job.GetJob(name)
}

func Flush(ctx context.Context, name string) {
	job.Flush(ctx, name)
}
