package pilot

import "vane-common/trpc"

const (
	PilotServer    trpc.SvcName = "pilot-server"
	PilotWalletBot trpc.SvcName = "pilot-wallet-bot"
)

var ServiceProfile = trpc.ServiceProfile{
	CenterServer:      PilotServer,
	OplogRecordServer: PilotServer,
	AlertLogServer:    PilotServer,
}
