package server

import "vane-common/clusterprofile"

type RoleConfig = clusterprofile.RoleConfig

func DefaultRoleConfig() RoleConfig {
	return clusterprofile.DefaultRoleConfig()
}
