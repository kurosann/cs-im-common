package trpc

type ServiceProfile struct {
	CenterServer      SvcName
	OplogRecordServer SvcName
	AlertLogServer    SvcName
}

func ApplyServiceProfile(profile ServiceProfile) {
	SetCenterServer(profile.CenterServer)
	SetOplogRecordServer(profile.OplogRecordServer)
	SetAlertLogServer(profile.AlertLogServer)
}
