package alertlog

type AlertLog struct {
	Id            int64            // 告警日志ID
	AlertCode     AlertLogCode     // 告警代码
	Severity      AlertLogSeverity // 告警级别
	Message       string           // 告警消息
	ResourceId    string           // 资源ID
	ResourceType  AlertLogResType  // 资源类型
	Status        AlertLogStatus   // 告警状态
	Assignee      string           // 处理人
	Remark        string           // 备注
	ConfirmStatus string           // 告警确认状态
	ConfirmAt     int64            // 确认时间
}

type AlertLogCode string

const (
	AlertLogCodeScriptRestart AlertLogCode = "script_restart" // 脚本重启
)

type AlertLogResType string

const (
	AlertLogResTypeScript AlertLogResType = "script" // 脚本
)

type AlertLogSeverity string

func (a AlertLogSeverity) String() string {
	return string(a)
}

const (
	AlertLogCritical AlertLogSeverity = "critical" // 致命 (Critical): 服务完全不可用，需要立即处理。
	AlertLogError    AlertLogSeverity = "error"    // 错误 (Error): 部分功能异常或性能严重下降，需要尽快处理。
	AlertLogWarning  AlertLogSeverity = "warning"  // 警告 (Warning): 潜在问题或性能偏差，需要关注。
	AlertLogInfo     AlertLogSeverity = "info"     // 信息 (Info): 一般性事件通知，通常无需立即操作。
)

type AlertLogStatus string

func (a AlertLogStatus) String() string {
	return string(a)
}

const (
	AlertLogStatusFiring       AlertLogStatus = "firing"       // 触发中 (Firing): 告警条件正在满足，问题持续存在。
	AlertLogStatusResolved     AlertLogStatus = "resolved"     // 已解决 (Resolved): 告警条件已不再满足，系统自动恢复。
	AlertLogStatusAcknowledged AlertLogStatus = "acknowledged" // 已确认 (Acknowledged): 已被人员确认，正在处理中。
	AlertLogStatusClosed       AlertLogStatus = "closed"       // 已关闭 (Closed): 问题已被人工处理完毕。
)

const (
	// 告警确认状态
	AlertLogConfirmStatusUnconfirmed = "unconfirmed" // 未确认
	AlertLogConfirmStatusConfirmed   = "confirmed"   // 已确认
)
