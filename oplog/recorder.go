package oplog

import (
	"context"
	"time"
)

type OplogModel struct {
	Id              int64        `json:"id"`               // 日志ID
	TraceId         string       `json:"req_id"`           // 请求ID
	Action          string       `json:"action"`           // 操作
	ActionMsg       string       `json:"action_msg"`       // 操作消息
	ResourceId      string       `json:"resource_id"`      // 资源ID
	ResourceName    string       `json:"resource_name"`    // 资源名称
	ResourceType    OplogResType `json:"resource_type"`    // 资源类型
	LogType         OplogType    `json:"log_type"`         // 日志类型
	Role            string       `json:"role"`             // 角色
	Status          OplogStatus  `json:"status"`           // 状态
	ProcessPosition int32        `json:"process_position"` // 进度
	OpScriptId      int64        `json:"op_script_id"`     // 操作脚本ID
	Level           OplogLevel   `json:"level"`            // 日志级别
	CancelBy        string       `json:"cancel_by"`        // 取消人
	FinishAt        *time.Time   `json:"finish_at"`        // 完成时间
	Err             string       `json:"err"`              // 错误
}
type OplogRecorder interface {
	RecordOplog(ctx context.Context, log *OplogModel) (id int64, err error)
	QueryOplog(ctx context.Context, id int64) (log *OplogModel, err error)
}

var oplogRecorder OplogRecorder = (*emptyRecorder)(nil)

type emptyRecorder struct {
}

func (e *emptyRecorder) RecordOplog(ctx context.Context, log *OplogModel) (id int64, err error) {
	return 0, nil
}
func (e *emptyRecorder) QueryOplog(ctx context.Context, id int64) (log *OplogModel, err error) {
	return nil, nil
}
