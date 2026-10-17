package taskmgr

import (
	"context"
	"fmt"
	"time"

	commonTaskmgr "vane-common/pkg/taskmgr"
	"vane-common/server/internal/itnlmodule/depend"
)

type TaskLockProvider struct {
	ttl    time.Duration
	locker depend.IDistributeLock
}

func NewTaskLockProvider(locker depend.IDistributeLock, ttl ...time.Duration) *TaskLockProvider {
	t := 30 * time.Second
	if len(ttl) > 0 {
		t = ttl[0]
	}

	return &TaskLockProvider{
		ttl:    t,
		locker: locker,
	}
}

func (p *TaskLockProvider) Lock(taskId string, owner string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.locker.Lock(ctx, p.key(taskId), p.ttl); err != nil {
		return commonTaskmgr.ErrTaskLocked
	}
	return nil
}

func (p *TaskLockProvider) Unlock(taskId string, owner string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.locker.Unlock(ctx, p.key(taskId)); err != nil {
		return commonTaskmgr.ErrTaskLockNotExists
	}
	return nil
}

func (p *TaskLockProvider) KeepAlive(taskId string, owner string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := p.locker.KeepAlive(ctx, p.key(taskId), p.ttl); err != nil {
		return commonTaskmgr.ErrTaskLockNotExists
	}
	return nil
}
func (p *TaskLockProvider) key(taskId string) string {
	return fmt.Sprintf("system:taskmgr:running:%s", taskId)
}
