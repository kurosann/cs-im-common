package pilot

import (
	"vane-common/clusterprofile"
	"vane-common/server/pkg/role"
	pilottrpc "vane-common/trpc/profile/pilot"
)

const (
	PilotServer    = role.ServiceRole(pilottrpc.PilotServer)
	PilotWalletBot = role.ServiceRole(pilottrpc.PilotWalletBot)
)

var Profile = clusterprofile.Profile{
	Name: "pilot",
	Services: map[role.ServiceRole]clusterprofile.ServiceProfile{
		PilotServer: {
			RoleConfig: clusterprofile.RoleConfig{
				NodeRole:               role.NodeRoleService,
				DefaultReplicationRole: role.ReplicationRoleSlave,
			},
		},
		PilotWalletBot: {
			RoleConfig: clusterprofile.RoleConfig{
				NodeRole:               role.NodeRoleService,
				DefaultReplicationRole: role.ReplicationRoleSlave,
			},
		},
	},
}
