package job

import "context"

type IJobTask interface {
	Run(ctx context.Context)
}

type FuncJob func(ctx context.Context)

func (j FuncJob) Run(ctx context.Context) {
	j(ctx)
}
