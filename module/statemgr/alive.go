package statemgr

import (
	"context"
	"time"
	"vane-common/module/job"
	"vane-common/pkg/logger"
)

type AliveEntity interface {
	GetTimestamp() int64
}

type IAliveChecker[L AliveEntity] interface {
	IsReady(ctx context.Context) bool
	List(ctx context.Context) ([]L, error)
	Delete(ctx context.Context, item L) error
}

var _ job.IJobTask = (*AliveCheckerWorker[AliveEntity])(nil)

type AliveCheckerWorker[L AliveEntity] struct {
	checker IAliveChecker[L]
	name    string
	ttl     time.Duration
}

func NewAliveChecker[L AliveEntity](name string, ttl time.Duration, checker IAliveChecker[L]) *AliveCheckerWorker[L] {
	return &AliveCheckerWorker[L]{
		checker: checker,
		name:    name,
		ttl:     ttl,
	}
}

func (c *AliveCheckerWorker[L]) Run(ctx context.Context) {
	if !c.checker.IsReady(ctx) {
		return
	}
	alive, err := c.checker.List(ctx)
	if err != nil {
		logger.Errorf(ctx, "alive checker [%s] get alive error: %s", c.name, err)
		return
	}
	for _, item := range alive {
		if item.GetTimestamp() < time.Now().Unix()-int64(c.ttl.Seconds()) {
			if err := c.checker.Delete(ctx, item); err != nil {
				logger.Errorf(ctx, "alive checker [%s] delete item [%v] error: %s", c.name, item, err)
			}
		}
	}
}

type aliveRegInfo struct {
	Spec      string
	Ttl       time.Duration
	LaunchRun bool
}

type AliveCheckerOption interface {
	apply(info *aliveRegInfo)
}
type aliveTtlOption struct {
	ttl time.Duration
}

func (o *aliveTtlOption) apply(info *aliveRegInfo) {
	info.Ttl = o.ttl
}

// WithAliveTtl 设置存活检查器超时时间
func WithAliveTtl(ttl time.Duration) AliveCheckerOption {
	return &aliveTtlOption{ttl: ttl}
}

type aliveLaunchRunOption struct {
	launchRun bool
}

func (o *aliveLaunchRunOption) apply(info *aliveRegInfo) {
	info.LaunchRun = o.launchRun
}

// WithAliveLaunchRun 设置启动时立即运行
func WithAliveLaunchRun() AliveCheckerOption {
	return &aliveLaunchRunOption{launchRun: true}
}

type aliveSpecOption struct {
	spec string
}

func (o *aliveSpecOption) apply(info *aliveRegInfo) {
	info.Spec = o.spec
}

// WithAliveSpec 设置定时任务执行时间
func WithAliveSpec(spec string) AliveCheckerOption {
	return &aliveSpecOption{spec: spec}
}

// NewAliveCheckerRegInfo 创建一个存活检查器注册信息
// name: 名称
// checker: 存活检查器
// options: 选项
func NewAliveCheckerRegInfo[L AliveEntity](name string, checker IAliveChecker[L], options ...AliveCheckerOption) StateSyncRegInfo {
	aliveRegInfo := aliveRegInfo{
		Ttl:  10 * time.Second,
		Spec: "@every 10s", // 默认10秒执行一次
	}
	for _, option := range options {
		option.apply(&aliveRegInfo)
	}
	return StateSyncRegInfo{
		Name:      name,
		Syncer:    NewAliveChecker(name, aliveRegInfo.Ttl, checker),
		Spec:      aliveRegInfo.Spec,
		LaunchRun: aliveRegInfo.LaunchRun,
		IsGlobal:  true,
	}
}
