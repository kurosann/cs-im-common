package job

import (
	"context"
	"sync"
	"time"

	"vane-common/pkg/logger"
	"vane-common/pkg/safe"
	"vane-common/server/internal/itnlmodule/depend"
	"vane-common/server/pkg/role"
)

type State struct {
	IsGlobal        bool
	ReplicationRole role.ReplicationRole
	LockTtl         time.Duration
}

var distributeLock depend.IDistributeLock

func SetDistributeLock(locker depend.IDistributeLock) {
	distributeLock = locker
}

var lockerMap = sync.Map{}

func GetLocker(jobName string) *sync.RWMutex {
	locker, _ := lockerMap.LoadOrStore(jobName, &sync.RWMutex{})
	return locker.(*sync.RWMutex)
}

func RunJob(ctx context.Context, moduleName string, jobName string, state State, worker func(ctx context.Context)) {
	if err := safe.Try(func() {
		if state.IsGlobal {
			switch state.ReplicationRole {
			case role.ReplicationRoleSlave:
				return
			case role.ReplicationRoleNone:
				ctx, cancel := context.WithCancel(ctx)
				defer cancel()
				rwMutex := sync.RWMutex{}
				if err := distributeLock.Lock(ctx, jobName, state.LockTtl); err != nil {
					logger.Errorf(ctx, "module [%s] job [%s] lock warn: %v", moduleName, jobName, err)
					return
				}
				defer func() {
					rwMutex.Lock()
					defer rwMutex.Unlock()
					if err := distributeLock.Unlock(ctx, jobName); err != nil {
						logger.Errorf(ctx, "module [%s] job [%s] unlock error: %v", moduleName, jobName, err)
					}
				}()
				safe.Go(func() {
					ticker := time.NewTicker(state.LockTtl - time.Second)
					defer ticker.Stop()
					for {
						select {
						case <-ctx.Done():
							return
						case <-ticker.C:
							if !rwMutex.TryLock() {
								continue
							}
							if err := distributeLock.KeepAlive(ctx, jobName, state.LockTtl); err != nil {
								logger.Errorf(ctx, "module [%s] job [%s] keep alive error:%s", moduleName, jobName, err)
							}
							rwMutex.Unlock()
						}
					}
				})
			case role.ReplicationRoleMaster:
			}
		}
		locker := GetLocker(jobName)
		if !locker.TryLock() {
			return
		}
		defer locker.Unlock()
		worker(ctx)
	}); err != nil {
		logger.Errorf(ctx, "module [%s] job [%s] run error: %v", moduleName, jobName, err)
	}
}
