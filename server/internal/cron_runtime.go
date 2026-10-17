package internal

import (
	"context"
	"time"

	"vane-common/module"
	"vane-common/module/job"
	"vane-common/pkg/logger"
	pkgJob "vane-common/server/pkg/job"
	"vane-common/server/pkg/role"

	"github.com/robfig/cron/v3"
)

const LockTtl = 10 * time.Second

type CronRuntime struct {
	cron *cron.Cron
}

func NewCronRuntime() *CronRuntime {
	return &CronRuntime{cron: cron.New()}
}

func (r *CronRuntime) Init(ctx context.Context, modules []module.ModuleInfo) error {
	for _, info := range modules {
		info := info
		if stateMgrModule, ok := info.Module.(module.IModuleStateMgr); ok {
			for _, stateMgr := range stateMgrModule.StateMgr() {
				stateMgr := stateMgr
				run := func(runCtx context.Context) {
					state := pkgJob.State{
						IsGlobal:        stateMgr.IsGlobal,
						ReplicationRole: role.GetRole().ReplicationRole,
						LockTtl:         LockTtl,
					}
					pkgJob.RunJob(runCtx, info.Name, stateMgr.Name, state, stateMgr.Syncer.Run)
				}
				job.RegisterFlush(stateMgr.Name, job.FuncJob(run))
				if _, err := r.cron.AddFunc(stateMgr.Spec, func() { run(ctx) }); err != nil {
					logger.Errorf(ctx, "module [%s] state mgr [%s] add error: %v", info.Name, stateMgr.Name, err)
					return err
				}
			}
		}

		for _, jobInfo := range info.Module.Job() {
			jobInfo := jobInfo
			run := func(runCtx context.Context) {
				state := pkgJob.State{
					IsGlobal:        jobInfo.IsGlobal,
					ReplicationRole: role.GetRole().ReplicationRole,
					LockTtl:         LockTtl,
				}
				pkgJob.RunJob(runCtx, info.Name, jobInfo.Name, state, jobInfo.Job.Run)
			}
			job.RegisterFlush(jobInfo.Name, job.FuncJob(run))
			if _, err := r.cron.AddFunc(jobInfo.Spec, func() { run(ctx) }); err != nil {
				logger.Errorf(ctx, "module [%s] job [%s] add error: %v", info.Name, jobInfo.Name, err)
				return err
			}
		}
	}

	return nil
}

func (r *CronRuntime) Run(ctx context.Context, modules []module.ModuleInfo) {
	time.Sleep(50 * time.Millisecond)
	for _, info := range modules {
		for _, jobInfo := range info.Module.Job() {
			if !jobInfo.LaunchRun {
				continue
			}
			state := pkgJob.State{
				IsGlobal:        jobInfo.IsGlobal,
				ReplicationRole: role.GetRole().ReplicationRole,
				LockTtl:         LockTtl,
			}
			pkgJob.RunJob(ctx, info.Name, jobInfo.Name, state, jobInfo.Job.Run)
		}

		if stateMgrModule, ok := info.Module.(module.IModuleStateMgr); ok {
			for _, stateMgr := range stateMgrModule.StateMgr() {
				if !stateMgr.LaunchRun {
					continue
				}
				state := pkgJob.State{
					IsGlobal:        stateMgr.IsGlobal,
					ReplicationRole: role.GetRole().ReplicationRole,
					LockTtl:         LockTtl,
				}
				pkgJob.RunJob(ctx, info.Name, stateMgr.Name, state, stateMgr.Syncer.Run)
			}
		}
	}

	r.cron.Run()
}

func (r *CronRuntime) Stop() context.Context {
	if r == nil || r.cron == nil {
		doneCtx, cancel := context.WithCancel(context.Background())
		cancel()
		return doneCtx
	}
	return r.cron.Stop()
}
