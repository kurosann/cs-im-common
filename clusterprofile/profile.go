package clusterprofile

import (
	"vane-common/server/pkg/role"
	"vane-common/server/serverconfig"
)

// RoleConfig 描述服务在某个集群中的默认角色配置。
type RoleConfig struct {
	NodeRole               role.NodeRole
	DefaultReplicationRole role.ReplicationRole
}

// DefaultRoleConfig 返回未命中任何集群资料时使用的兜底角色配置。
func DefaultRoleConfig() RoleConfig {
	return RoleConfig{
		NodeRole:               role.NodeRoleService,
		DefaultReplicationRole: role.ReplicationRoleSlave,
	}
}

// ServiceProfile 描述服务在某个集群中的常量配置。
type ServiceProfile struct {
	RoleConfig   RoleConfig
	ServerConfig serverconfig.Config
}

// DefaultServiceProfile 返回未命中任何集群资料时使用的兜底服务资料。
func DefaultServiceProfile() ServiceProfile {
	return ServiceProfile{
		RoleConfig: DefaultRoleConfig(),
	}
}

// Profile 描述一套集群常量配置。
type Profile struct {
	Name     string
	Services map[role.ServiceRole]ServiceProfile
}

// GetServiceProfile 获取指定服务在该集群中的默认资料。
func (p Profile) GetServiceProfile(serviceName role.ServiceRole) ServiceProfile {
	if profile, ok := p.Services[serviceName]; ok {
		return profile
	}
	return DefaultServiceProfile()
}
