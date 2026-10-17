package oplog

import (
	"context"
	"sync"
	"time"
	"vane-common/pkg/dataaccess/model"
	"vane-common/pkg/logger"
	"vane-common/pkg/safe"
	"vane-common/trpc/common"
)

var once sync.Once

type OplogResType string

type OplogType string

const (
	OplogTypeEvent OplogType = "event"
	OplogTypeAsync OplogType = "async"
)

type OplogStatus string

const (
	OplogStatusPending   OplogStatus = "pending"
	OplogStatusRunning   OplogStatus = "running"
	OplogStatusCompleted OplogStatus = "completed"
	OplogStatusFailed    OplogStatus = "failed"
	OplogStatusCanceled  OplogStatus = "canceling"
)

type OplogLevel string

const (
	OplogLevelInfo  OplogLevel = "info"
	OplogLevelWarn  OplogLevel = "warn"
	OplogLevelError OplogLevel = "error"
)

type Action struct {
	Code string
	Msg  string
}

func NewAction(code, msg string) Action {
	return Action{
		Code: code,
		Msg:  msg,
	}
}

func (a Action) WithMsg(msg string) Action {
	a.Msg = msg
	return a
}

type Oplog struct {
	Action       Action       // 操作
	TableRes     model.Table  // 资源
	ResourceId   string       // 资源ID
	ResourceType OplogResType // 资源类型
	ScriptId     int64        // 操作脚本ID
	Err          error        // 错误
}

func (a *Oplog) WithErr(err error) *Oplog {
	a.Err = err
	return a
}
func (a *Oplog) WithScriptId(scriptId int64) *Oplog {
	a.ScriptId = scriptId
	return a
}

func NewDbResOplog(action Action, res model.Table) *Oplog {
	return &Oplog{
		Action:   action,
		TableRes: res,
	}
}

func SyncOplog(ctx context.Context, log *Oplog) {
	if log.TableRes != nil {
		log.ResourceId = log.TableRes.GetID().String()
		log.ResourceType = OplogResType(log.TableRes.TableName())
	}

	syncChan <- &OplogModel{
		Action:       log.Action.Code,
		ActionMsg:    log.Action.Msg,
		ResourceId:   log.ResourceId,
		ResourceType: log.ResourceType,
		OpScriptId:   log.ScriptId,
		TraceId:      common.GetRequestId(ctx),
	}
}

func AsyncOplog(ctx context.Context, log *Oplog) (id int64, err error) {
	return oplogRecorder.RecordOplog(ctx, &OplogModel{
		TraceId:      common.GetRequestId(ctx),
		Action:       log.Action.Code,
		ActionMsg:    log.Action.Msg,
		ResourceId:   log.ResourceId,
		ResourceType: log.ResourceType,
		OpScriptId:   log.ScriptId,
	})
}

type UpdateOplogReq struct {
	Id              int64
	Status          OplogStatus
	ProcessPosition int32
	Err             error
}

func UpdateOplog(ctx context.Context, id int64, status OplogStatus, processPosition int32, errs ...error) {
	var err error
	if len(errs) > 0 {
		err = errs[0]
	}
	updateChan <- &UpdateOplogReq{
		Id:              id,
		Status:          status,
		ProcessPosition: processPosition,
		Err:             err,
	}
}

func FinishOplog(ctx context.Context, id int64) {
	finishChan <- id
}

var (
	syncChan   = make(chan *OplogModel, 100)
	updateChan = make(chan *UpdateOplogReq, 100)
	finishChan = make(chan int64, 100)
)

func Init(ctx context.Context, recorder OplogRecorder) {
	oplogRecorder = recorder
	once.Do(func() {
		safe.GoKeep(ctx, 10*time.Second, func() {
			for {
				select {
				case <-ctx.Done():
					return
				case log := <-syncChan:
					_, err := oplogRecorder.RecordOplog(ctx, log)
					if err != nil {
						logger.Errorf(ctx, "sync oplog failed: %v", err)
						continue
					}
				case req := <-updateChan:
					log, err := oplogRecorder.QueryOplog(ctx, req.Id)
					if err != nil {
						logger.Errorf(ctx, "update oplog failed: %v", err)
						continue
					}
					log.Status = req.Status
					log.ProcessPosition = req.ProcessPosition
					log.Err = req.Err.Error()
					_, err = oplogRecorder.RecordOplog(ctx, log)
					if err != nil {
						logger.Errorf(ctx, "update oplog failed: %v", err)
						continue
					}
				case id := <-finishChan:
					log, err := oplogRecorder.QueryOplog(ctx, id)
					if err != nil {
						logger.Errorf(ctx, "finish oplog failed: %v", err)
						continue
					}
					log.Status = OplogStatusCompleted
					log.ProcessPosition = 100
					now := time.Now()
					log.FinishAt = &now
					_, err = oplogRecorder.RecordOplog(ctx, log)
					if err != nil {
						logger.Errorf(ctx, "finish oplog failed: %v", err)
						continue
					}
				}
			}
		})
	})
}
