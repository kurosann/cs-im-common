package statemgr

import (
	"context"
	"vane-common/module/job"
	"vane-common/pkg/logger"
)

type IPushStateSyncer[L any] interface {
	IsReady(ctx context.Context) bool
	ListLocal(ctx context.Context) ([]L, error)
	Push(ctx context.Context, local L) error
}

var _ job.IJobTask = (*StateSyncerPushWorker[any])(nil)

type StateSyncerPushWorker[L any] struct {
	syncer IPushStateSyncer[L]
	name   string
}

func NewPushStateSyncer[L any](name string, syncer IPushStateSyncer[L]) *StateSyncerPushWorker[L] {
	return &StateSyncerPushWorker[L]{
		syncer: syncer,
		name:   name,
	}
}

func (s *StateSyncerPushWorker[L]) Run(ctx context.Context) {
	if !s.syncer.IsReady(ctx) {
		return
	}
	local, err := s.syncer.ListLocal(ctx)
	if err != nil {
		logger.Errorf(ctx, "state syncer [%s] list local error: %s", s.name, err)
		return
	}
	for _, localItem := range local {
		if err := s.syncer.Push(ctx, localItem); err != nil {
			logger.Errorf(ctx, "state syncer [%s] push error: %s", s.name, err)
		}
	}
}

type PushStateSyncerOption interface {
	apply(syncer *StateSyncRegInfo)
}

type pushLaunchRunOption struct {
	launchRun bool
}

func (o *pushLaunchRunOption) apply(syncer *StateSyncRegInfo) {
	syncer.LaunchRun = o.launchRun
}

// WithPushLaunchRun 设置启动时立即运行
func WithPushLaunchRun() PushStateSyncerOption {
	return &pushLaunchRunOption{launchRun: true}
}

type pushSpecOption struct {
	spec string
}

func (o *pushSpecOption) apply(syncer *StateSyncRegInfo) {
	syncer.Spec = o.spec
}

// WithPushSpec 设置定时任务执行时间
func WithPushSpec(spec string) PushStateSyncerOption {
	return &pushSpecOption{spec: spec}
}

// NewPushStateSyncRegInfo 创建一个推送同步器注册信息
// name: 名称
// syncer: 同步器
func NewPushStateSyncRegInfo[L any](name string, syncer IPushStateSyncer[L], options ...PushStateSyncerOption) StateSyncRegInfo {
	syncInfo := StateSyncRegInfo{
		Name:   name,
		Syncer: NewPushStateSyncer(name, syncer),
		Spec:   "@every 10s", // 默认10秒执行一次
	}
	for _, option := range options {
		option.apply(&syncInfo)
	}
	return syncInfo
}
