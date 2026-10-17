package depend

import (
	"context"
	"encoding/json"

	"vane-common/alertlog"
	"vane-common/pkg/logger"

	"vane-common/publish"
	"vane-common/trpc"
	sdk "vane-common/v1/log/alertlog"
	system "vane-common/v1/pilot/system"
)

var _ alertlog.IAlertLogRecorder = (*DefaultAlertLogRecorder)(nil)

type DefaultAlertLogRecorder struct {
}

func NewDefaultAlertLogRecorder() alertlog.IAlertLogRecorder {
	return &DefaultAlertLogRecorder{}
}

func (d *DefaultAlertLogRecorder) CreateAlertLog(ctx context.Context, log *alertlog.AlertLog) (err error) {
	conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), trpc.AlertLogServer)
	if err != nil {
		logger.Errorf(ctx, "[DefaultAlertLogRecorder] GetSvcConn AlertLog error:%v", err)
		return err
	}
	client := sdk.NewAlertlogServiceClient(conn)
	resp, err := client.CreateAlertlog(ctx, &sdk.CreateAlertlogReq{
		Alertlog: &sdk.AlertLog{
			AlertCode:    string(log.AlertCode),
			Severity:     string(log.Severity),
			Message:      log.Message,
			ResourceId:   log.ResourceId,
			ResourceType: string(log.ResourceType),
			Status:       string(log.Status),
			Assignee:     log.Assignee,
			Remark:       log.Remark,
		},
	})
	if err != nil {
		logger.Errorf(ctx, "[DefaultAlertLogRecorder] CreateAlertlog error:%v", err)
		return err
	}

	// 获取告警日志ID
	resp_, err_ := client.GetAlertlog(ctx, &sdk.GetAlertlogReq{
		Id: resp.Id,
	})
	if err_ != nil {
		logger.Errorf(ctx, "[DefaultAlertLogRecorder] GetAlertlog error:%v", err_)
		return err_
	}
	log.Id = resp_.Alertlog.Id

	// 发布到 controller
	conn_, err_ := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), trpc.CenterServer)
	if err_ != nil {
		logger.Errorf(ctx, "[DefaultAlertLogRecorder] GetSvcConn Controller error:%v", err)
		return err
	}
	client_ := system.NewPublisherServiceClient(conn_)
	jsonData, err_ := json.Marshal(resp_.Alertlog)
	if err_ != nil {
		logger.Errorf(ctx, "[DefaultAlertLogRecorder] json.Marshal error:%v", err)
		return err_
	}
	_, err_ = client_.Publish(ctx, &system.PublishReq{Topic: publish.TopicAlertLog, Data: jsonData})
	if err_ != nil {
		logger.Errorf(ctx, "[DefaultAlertLogRecorder] Publish error:%v", err)
		return err_
	}

	return nil

}

func (d *DefaultAlertLogRecorder) UpdateAlertLog(ctx context.Context, log *alertlog.AlertLog) (err error) {
	conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), trpc.AlertLogServer)
	if err != nil {
		logger.Errorf(ctx, "[DefaultAlertLogRecorder] GetSvcConn AlertLog error:%v", err)
		return err
	}
	client := sdk.NewAlertlogServiceClient(conn)

	oldLog, err := d.GetAlertLog(ctx, log.Id)
	if err != nil {
		logger.Errorf(ctx, "[DefaultAlertLogRecorder] GetAlertLog error:%v", err)
		return err
	}

	_, err = client.UpdateAlertlog(ctx, &sdk.UpdateAlertlogReq{
		Alertlog: &sdk.AlertLog{
			Id:            log.Id,
			Status:        string(log.Status),
			Assignee:      oldLog.Assignee,
			Remark:        oldLog.Remark,
			ConfirmStatus: log.ConfirmStatus,
		},
	})
	if err != nil {
		logger.Errorf(ctx, "[DefaultAlertLogRecorder] UpdateAlertlog error:%v", err)
		return err
	}

	return nil
}

func (d *DefaultAlertLogRecorder) GetAlertLog(ctx context.Context, id int64) (log *alertlog.AlertLog, err error) {
	conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), trpc.AlertLogServer)
	if err != nil {
		logger.Errorf(ctx, "[DefaultAlertLogRecorder] GetSvcConn AlertLog error:%v", err)
		return nil, err
	}
	client := sdk.NewAlertlogServiceClient(conn)
	resp, err := client.GetAlertlog(ctx, &sdk.GetAlertlogReq{
		Id: id,
	})
	if err != nil {
		logger.Errorf(ctx, "[DefaultAlertLogRecorder] GetAlertlog error:%v", err)
		return nil, err
	}
	return &alertlog.AlertLog{
		Id:            resp.Alertlog.Id,
		AlertCode:     alertlog.AlertLogCode(resp.Alertlog.AlertCode),
		Severity:      alertlog.AlertLogSeverity(resp.Alertlog.Severity),
		Message:       resp.Alertlog.Message,
		ResourceId:    resp.Alertlog.ResourceId,
		ResourceType:  alertlog.AlertLogResType(resp.Alertlog.ResourceType),
		Status:        alertlog.AlertLogStatus(resp.Alertlog.Status),
		Assignee:      resp.Alertlog.Assignee,
		Remark:        resp.Alertlog.Remark,
		ConfirmStatus: resp.Alertlog.ConfirmStatus,
		ConfirmAt:     resp.Alertlog.ConfirmAt.AsTime().Unix(),
	}, nil
}
