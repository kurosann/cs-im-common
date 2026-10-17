package statemgr

import (
	"context"
	"vane-common/module/job"
	"vane-common/pkg/logger"
	"vane-common/pkg/safe"
)

type StateEntity interface {
	GetID() string
}

type IPullStateSyncer[R StateEntity, L StateEntity] interface {
	IsReady(ctx context.Context) bool
	ListRemote(ctx context.Context) ([]R, error)
	ListLocal(ctx context.Context) ([]L, error)
	CompareExist(ctx context.Context, remote R, local L) bool
	Create(ctx context.Context, remote R) error
	Update(ctx context.Context, remote R, local L) error
	Delete(ctx context.Context, local L) error
}

var _ job.IJobTask = (*StateSyncerPullWorker[StateEntity, StateEntity])(nil)

type StateSyncerPullWorker[R StateEntity, L StateEntity] struct {
	syncer IPullStateSyncer[R, L]
	name   string
}

func NewPullStateSyncer[R StateEntity, L StateEntity](name string, syncer IPullStateSyncer[R, L]) *StateSyncerPullWorker[R, L] {
	return &StateSyncerPullWorker[R, L]{
		syncer: syncer,
		name:   name,
	}
}

func (s *StateSyncerPullWorker[R, L]) Run(ctx context.Context) {
	if !s.syncer.IsReady(ctx) {
		return
	}
	var remote []R
	if err := safe.TryE(func() error {
		var err error
		remote, err = s.syncer.ListRemote(ctx)
		return err
	}); err != nil {
		logger.Errorf(ctx, "state syncer [%s] list remote error: %s", s.name, err)
		return
	}

	var local []L
	if err := safe.TryE(func() error {
		var err error
		local, err = s.syncer.ListLocal(ctx)
		return err
	}); err != nil {
		logger.Errorf(ctx, "state syncer [%s] list local error: %s", s.name, err)
		return
	}
	remoteMap := make(map[string]R)
	for _, remoteItem := range remote {
		remoteMap[remoteItem.GetID()] = remoteItem
	}
	localMap := make(map[string]L)
	for _, localItem := range local {
		localMap[localItem.GetID()] = localItem
	}

	for _, remoteItem := range remote {
		localItem, ok := localMap[remoteItem.GetID()]
		if !ok || !s.syncer.CompareExist(ctx, remoteItem, localItem) {
			if err := safe.TryE(func() error { return s.syncer.Create(ctx, remoteItem) }); err != nil {
				logger.Errorf(ctx, "state syncer [%s] create remote error: %s", s.name, err)
			}
		} else {
			if err := safe.TryE(func() error { return s.syncer.Update(ctx, remoteItem, localItem) }); err != nil {
				logger.Errorf(ctx, "state syncer [%s] update remote error: %s", s.name, err)
			}
		}
	}
	for _, localItem := range local {
		_, ok := remoteMap[localItem.GetID()]
		if !ok {
			if err := safe.TryE(func() error { return s.syncer.Delete(ctx, localItem) }); err != nil {
				logger.Errorf(ctx, "state syncer [%s] delete remote error: %s", s.name, err)
			}
		}
	}
}

type StateSyncRegInfo struct {
	Name      string       // 定时任务名称
	Syncer    job.IJobTask // job 实现
	LaunchRun bool         // 是否为启动时运行
	Spec      string       // cron表达式或者运行逻辑匹配
	IsGlobal  bool         // 是否为全局任务
}

type PullStateSyncerOption interface {
	apply(syncer *StateSyncRegInfo)
}

type pullLaunchRunOption struct {
	launchRun bool
}

// WithPullLaunchRun 设置启动时立即运行
func WithPullLaunchRun() PullStateSyncerOption {
	return &pullLaunchRunOption{launchRun: true}
}

func (o *pullLaunchRunOption) apply(syncer *StateSyncRegInfo) {
	syncer.LaunchRun = o.launchRun
}

type pullSpecOption struct {
	spec string
}

// WithPullSpec 设置定时任务执行时间
func WithPullSpec(spec string) PullStateSyncerOption {
	return &pullSpecOption{spec: spec}
}

func (o *pullSpecOption) apply(syncer *StateSyncRegInfo) {
	syncer.Spec = o.spec
}

// NewPullStateSyncRegInfo 创建一个拉取同步器注册信息
// name: 名称
// syncer: 同步器
func NewPullStateSyncRegInfo[R StateEntity, L StateEntity](name string, syncer IPullStateSyncer[R, L], options ...PullStateSyncerOption) StateSyncRegInfo {
	syncInfo := StateSyncRegInfo{
		Name:     name,
		Syncer:   NewPullStateSyncer(name, syncer),
		Spec:     "@every 10s", // 默认10秒执行一次
		IsGlobal: true,
	}
	for _, option := range options {
		option.apply(&syncInfo)
	}
	return syncInfo
}
