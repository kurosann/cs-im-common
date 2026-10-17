package job

import (
	"context"
	"fmt"
	"time"

	"vane-common/module/job"
	"vane-common/pkg/logger"
	"vane-common/pkg/node"
	"vane-common/server/internal/itnlmodule/depend"
	"vane-common/server/pkg/role"
)

const ElectionMasterJobName = "election_master"

var _ job.IJobTask = (*ElectionMasterJob)(nil)

type ElectionMasterJob struct {
	distributeLock depend.IDistributeLock
	serviceName    string
	lockTtl        time.Duration
	container      *node.ContainerNode
}

func NewElectionMasterJob(serviceName string, lockTtl time.Duration) *ElectionMasterJob {
	return &ElectionMasterJob{
		distributeLock: depend.NewDistributeLock(serviceName),
		serviceName:    serviceName,
		lockTtl:        lockTtl,
		container:      node.NewContainerNode(serviceName),
	}
}

func (e *ElectionMasterJob) Run(ctx context.Context) {
	if role.GetRole().ReplicationRole == role.ReplicationRoleNone {
		return
	}
	containerId, err := e.container.GetContainerID(ctx)
	if err != nil {
		logger.Errorf(ctx, "get container id error: %s", err.Error())
		return
	}
	key := fmt.Sprintf("%s:%s", ElectionMasterJobName, e.serviceName)
	if role.IsMaster() {
		if err := e.distributeLock.KeepAliveWithValue(ctx, key, containerId, e.lockTtl); err != nil {
			logger.Errorf(ctx, "election master keep alive error: %s", err.Error())
			role.SetReplicationRole(role.ReplicationRoleSlave)
			logger.Infof(ctx, "service role reset to slave")
			return
		}
		role.SetReplicationMasterRole(role.ReplicationRoleMaster, e.lockTtl)
	} else {
		if err := e.distributeLock.LockWithValue(ctx, key, containerId, e.lockTtl); err != nil {
			role.SetReplicationRole(role.ReplicationRoleSlave)
			return
		}
		role.SetReplicationMasterRole(role.ReplicationRoleMaster, e.lockTtl)
		logger.Infof(ctx, "service role set to master!")
	}
}
