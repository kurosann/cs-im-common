package alertlog

import (
	"context"
	"vane-common/pkg/logger"
	"vane-common/pkg/safe"
)

var _globalAlertLog alertlogService

func Init(ctx context.Context, recorder IAlertLogRecorder) {
	_globalAlertLog = alertlogService{
		ctx:        ctx,
		recorder:   recorder,
		confirmLog: make(chan int64, 100),
		createLog:  make(chan AlertLog, 100),
	}
	_globalAlertLog.start()
}

type alertlogService struct {
	ctx      context.Context
	recorder IAlertLogRecorder

	createLog  chan AlertLog
	confirmLog chan int64
}

func (a *alertlogService) start() {
	safe.Go(func() {
		for {
			select {
			case <-a.ctx.Done():
				logger.Infof(a.ctx, "[alertLog] alertlogService stopped")
				return
			case log := <-a.createLog:
				if err := a.recorder.CreateAlertLog(a.ctx, &log); err != nil {
					logger.Errorf(a.ctx, "[alertLog] create alert log failed: %v", err)
					continue
				}
			case id := <-a.confirmLog:
				if err := a.recorder.UpdateAlertLog(a.ctx, &AlertLog{
					Id:            id,
					Status:        AlertLogStatusClosed,
					ConfirmStatus: AlertLogConfirmStatusConfirmed,
				}); err != nil {
					logger.Errorf(a.ctx, "[alertLog] confirm alert log failed: %v", err)
					continue
				}
			}
		}
	})
}

func Alert(ctx context.Context, alertCode AlertLogCode, severity AlertLogSeverity, message string, resourceId string, resourceType AlertLogResType) {
	log := AlertLog{
		AlertCode:    alertCode,
		Severity:     severity,
		Message:      message,
		ResourceId:   resourceId,
		ResourceType: resourceType,
	}
	_globalAlertLog.createLog <- log
	logger.Infof(ctx, "[alertLog] alertlogService create alert log %+v", log)
}

func Confirm(ctx context.Context, alertLogId int64) {
	_globalAlertLog.confirmLog <- alertLogId
	logger.Infof(ctx, "[alertLog] alertlogService confirm alert log %d", alertLogId)
}
