package role

import (
	"sync"
	"time"
)

type Role struct {
	ReplicationRole           ReplicationRole
	ReplicationMasterExpireAt time.Time
	NodeRole                  NodeRole
	ServiceRole               ServiceRole
}

type ReplicationRole string

const (
	ReplicationRoleNone   ReplicationRole = "none"
	ReplicationRoleMaster ReplicationRole = "master"
	ReplicationRoleSlave  ReplicationRole = "slave"
)

type NodeRole string

const (
	NodeRoleControl NodeRole = "control"
	NodeRoleWorker  NodeRole = "worker"
	NodeRoleService NodeRole = "service"
)

type ServiceRole string

var (
	role *Role
	once sync.Once
)

func initRole() {
	once.Do(func() {
		role = &Role{
			ReplicationRole: ReplicationRoleSlave,
			NodeRole:        NodeRoleService,
		}
	})
}
func SetReplicationRole(replicationRole ReplicationRole) {
	initRole()
	role.ReplicationRole = replicationRole
}
func SetReplicationMasterRole(replicationRole ReplicationRole, ttl time.Duration) {
	initRole()
	role.ReplicationRole = replicationRole
	if replicationRole == ReplicationRoleMaster {
		role.ReplicationMasterExpireAt = time.Now().Add(ttl)
	}
}
func SetNodeRole(nodeRole NodeRole) {
	initRole()
	role.NodeRole = nodeRole
}
func SetServiceRole(serviceRole ServiceRole) {
	initRole()
	role.ServiceRole = serviceRole
}
func GetRole() Role {
	initRole()
	return *role
}
func IsMaster() bool {
	initRole()
	return role.ReplicationRole == ReplicationRoleMaster && role.ReplicationMasterExpireAt.After(time.Now())
}
