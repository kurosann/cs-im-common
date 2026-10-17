package alertlog

import (
	"context"
)

type IAlertLogRecorder interface {
	CreateAlertLog(ctx context.Context, log *AlertLog) (err error)
	UpdateAlertLog(ctx context.Context, log *AlertLog) (err error)
	GetAlertLog(ctx context.Context, id int64) (log *AlertLog, err error)
}
