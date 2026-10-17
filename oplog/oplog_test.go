package oplog_test

import (
	"context"
	"testing"
	"time"
	"vane-common/oplog"
	oplogpb "vane-common/oplog/proto"
	"vane-common/pkg/timeutil"
	"vane-common/trpc"
	pilottrpc "vane-common/trpc/profile/pilot"
)

func TestOplog(t *testing.T) {
	trpc.InitTrpcDiscover("test", "localhost:8080", trpc.NewSimpleSvcmap(map[string][]string{
		"bg-log": {"localhost:8566"},
	}))
	oplog.Init(context.Background(), &OplogRecord{})
	ol := oplog.NewDbResOplog(oplog.NewAction("test", "test"), nil)
	oplog.SyncOplog(context.Background(), ol)
	time.Sleep(1 * time.Second)
}

var _ oplog.OplogRecorder = (*OplogRecord)(nil)

type OplogRecord struct {
}

func (o *OplogRecord) QueryOplog(ctx context.Context, id int64) (log *oplog.OplogModel, err error) {
	conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), pilottrpc.PilotServer)
	if err != nil {
		return nil, err
	}
	client := oplogpb.NewOplogServiceClient(conn)
	resp, err := client.QueryOplog(ctx, &oplogpb.QueryOplogReq{Id: id})
	if err != nil {
		return nil, err
	}
	return &oplog.OplogModel{
		Id:              resp.Oplog.Id,
		TraceId:         resp.Oplog.TraceId,
		Action:          resp.Oplog.Action,
		ActionMsg:       resp.Oplog.ActionMsg,
		ResourceId:      resp.Oplog.ResourceId,
		ResourceType:    oplog.OplogResType(resp.Oplog.ResourceType),
		LogType:         o.PbOplogTypeModel2(resp.Oplog.LogType),
		Role:            resp.Oplog.Role,
		Status:          o.PbOplogStatus2Model(resp.Oplog.Status),
		ProcessPosition: resp.Oplog.ProcessPosition,
		OpScriptId:      resp.Oplog.OpScriptId,
		Level:           o.PbOplogLevel2Model(resp.Oplog.Level),
		CancelBy:        resp.Oplog.CancelBy,
		FinishAt:        timeutil.PbTimestamp2PTime(resp.Oplog.FinishAt),
		Err:             resp.Oplog.Err,
	}, nil
}

func (o *OplogRecord) PbOplogTypeModel2(t oplogpb.OplogType) oplog.OplogType {
	switch t {
	case oplogpb.OplogType_OPLG_TYPE_EVENT:
		return oplog.OplogTypeEvent
	case oplogpb.OplogType_OPLG_TYPE_ASYNC:
		return oplog.OplogTypeAsync
	}
	return oplog.OplogTypeEvent
}

func (o *OplogRecord) PbOplogStatus2Model(s oplogpb.OplogStatus) oplog.OplogStatus {
	switch s {
	case oplogpb.OplogStatus_OPLG_STATUS_PENDING:
		return oplog.OplogStatusPending
	case oplogpb.OplogStatus_OPLG_STATUS_RUNNING:
		return oplog.OplogStatusRunning
	case oplogpb.OplogStatus_OPLG_STATUS_COMPLETED:
		return oplog.OplogStatusCompleted
	}
	return oplog.OplogStatusPending
}

func (o *OplogRecord) PbOplogLevel2Model(l oplogpb.OplogLevel) oplog.OplogLevel {
	switch l {
	case oplogpb.OplogLevel_OPLG_LEVEL_INFO:
		return oplog.OplogLevelInfo
	case oplogpb.OplogLevel_OPLG_LEVEL_WARN:
		return oplog.OplogLevelWarn
	case oplogpb.OplogLevel_OPLG_LEVEL_ERROR:
		return oplog.OplogLevelError
	}
	return oplog.OplogLevelInfo
}
func (o *OplogRecord) model2PbOplogType(t oplog.OplogType) oplogpb.OplogType {
	switch t {
	case oplog.OplogTypeEvent:
		return oplogpb.OplogType_OPLG_TYPE_EVENT
	case oplog.OplogTypeAsync:
		return oplogpb.OplogType_OPLG_TYPE_ASYNC
	}
	return oplogpb.OplogType_OPLG_TYPE_EVENT
}

func (o *OplogRecord) model2PbOplogStatus(s oplog.OplogStatus) oplogpb.OplogStatus {
	switch s {
	case oplog.OplogStatusPending:
		return oplogpb.OplogStatus_OPLG_STATUS_PENDING
	case oplog.OplogStatusRunning:
		return oplogpb.OplogStatus_OPLG_STATUS_RUNNING
	case oplog.OplogStatusCompleted:
		return oplogpb.OplogStatus_OPLG_STATUS_COMPLETED
	}
	return oplogpb.OplogStatus_OPLG_STATUS_PENDING
}

func (o *OplogRecord) model2PbOplogLevel(l oplog.OplogLevel) oplogpb.OplogLevel {
	switch l {
	case oplog.OplogLevelInfo:
		return oplogpb.OplogLevel_OPLG_LEVEL_INFO
	case oplog.OplogLevelWarn:
		return oplogpb.OplogLevel_OPLG_LEVEL_WARN
	case oplog.OplogLevelError:
		return oplogpb.OplogLevel_OPLG_LEVEL_ERROR
	}
	return oplogpb.OplogLevel_OPLG_LEVEL_INFO
}

func (o *OplogRecord) RecordOplog(ctx context.Context, log *oplog.OplogModel) (id int64, err error) {
	conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), pilottrpc.PilotServer)
	if err != nil {
		return 0, err
	}
	client := oplogpb.NewOplogServiceClient(conn)
	resp, err := client.RecordOplog(ctx, &oplogpb.RecordOplogReq{Oplog: &oplogpb.Oplog{
		Id:              log.Id,
		TraceId:         log.TraceId,
		Action:          log.Action,
		ActionMsg:       log.ActionMsg,
		ResourceId:      log.ResourceId,
		ResourceType:    string(log.ResourceType),
		LogType:         o.model2PbOplogType(log.LogType),
		Role:            log.Role,
		Status:          o.model2PbOplogStatus(log.Status),
		ProcessPosition: log.ProcessPosition,
		OpScriptId:      log.OpScriptId,
		Level:           o.model2PbOplogLevel(log.Level),
		CancelBy:        log.CancelBy,
		FinishAt:        timeutil.PTime2PbTimestamp(log.FinishAt),
		Err:             log.Err,
	}})
	if err != nil {
		return 0, err
	}
	return resp.Id, nil
}

func NewOplogRecord() *OplogRecord {
	return &OplogRecord{}
}
