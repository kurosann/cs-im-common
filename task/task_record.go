package task

import (
	"context"
	"errors"

	"github.com/avast/retry-go"

	"vane-common/pkg/logger"

	"vane-common/pkg/task"
	"vane-common/trpc"
	"vane-common/v1/pilot/system"
)

var _ task.Recorder = (*TaskRecorder)(nil)

type TaskRecorder struct {
	serviceName string
}

func (t *TaskRecorder) QueryStep(stepID string) (result task.StepExecResult, err error) {
	conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(context.Background()), trpc.CenterServer)
	if err != nil {
		return task.StepExecResult{}, err
	}
	resp, err := system.NewTaskRecordServiceClient(conn).QueryStepRecord(context.Background(), &system.QueryStepRecordReq{
		StepID: stepID,
	})
	if err != nil {
		return task.StepExecResult{}, err
	}
	return task.StepExecResult{
		Status:          task.Status(resp.Status),
		Order:           int(resp.Order),
		StepName:        resp.StepName,
		TaskID:          resp.TaskID,
		StepID:          resp.StepID,
		ProcessPosition: int(resp.ProcessPosition),
		Err:             errors.New(resp.Err),
		Payload:         resp.Payload,
		Result:          resp.Result,
	}, nil
}

func (t *TaskRecorder) QueryTask(taskID string) (result task.TaskExecResult, err error) {
	conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(context.Background()), trpc.CenterServer)
	if err != nil {
		return task.TaskExecResult{}, err
	}
	resp, err := system.NewTaskRecordServiceClient(conn).QueryTaskRecord(context.Background(), &system.QueryTaskRecordReq{
		TaskID: taskID,
	})
	if err != nil {
		return task.TaskExecResult{}, err
	}
	return task.TaskExecResult{
		Status:          task.Status(resp.Status),
		Order:           int(resp.Order),
		StepName:        resp.StepName,
		StepID:          resp.StepID,
		ProcessPosition: int(resp.ProcessPosition),
		Err:             errors.New(resp.Err),
		Payload:         resp.Payload,
		Result:          resp.Result,
	}, nil
}

func NewTaskRecorder(serviceName string) *TaskRecorder {
	return &TaskRecorder{
		serviceName: serviceName,
	}
}

func Ife(err error, def error) error {
	if err == nil {
		return def
	}
	return err
}
func IfeStr(err error, def string) string {
	if err == nil {
		return def
	}
	return err.Error()
}
func (t *TaskRecorder) RecordStep(stepID string, result task.StepExecResult) (id string, err error) {
	conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(context.Background()), trpc.CenterServer)
	if err != nil {
		return "", err
	}
	if stepID == "" {
		var id string
		if err := retry.Do(func() error {
			res, err := system.NewTaskRecordServiceClient(conn).SaveStepRecord(context.Background(), &system.StepRecord{
				Status:          string(result.Status),
				Order:           int64(result.Order),
				StepName:        result.StepName,
				TaskID:          result.TaskID,
				StepID:          result.StepID,
				ProcessPosition: int64(result.ProcessPosition),
				Err:             IfeStr(result.Err, ""),
				Payload:         result.Payload,
				Result:          result.Result,
				ServiceName:     t.serviceName,
			})
			if err != nil {
				logger.Errorf(context.Background(), "RecordStep failed %s", err.Error())
				return err
			}
			id, err = res.Base.Id, err
			if err != nil {
				return err
			}

			return nil
		}, retry.LastErrorOnly(true), retry.Attempts(3)); err != nil {
			logger.Errorf(context.Background(), "RecordStep failed %s", err.Error())
			return "", err
		}
		return id, nil
	} else {
		var id string
		if err := retry.Do(func() error {
			res, err := system.NewTaskRecordServiceClient(conn).UpdateStepRecord(context.Background(), &system.StepRecord{
				Base: &system.Base{
					Id: stepID,
				},
				Status:          string(result.Status),
				Order:           int64(result.Order),
				StepName:        result.StepName,
				TaskID:          result.TaskID,
				StepID:          result.StepID,
				ProcessPosition: int64(result.ProcessPosition),
				Err:             IfeStr(result.Err, ""),
				Payload:         result.Payload,
				Result:          result.Result,
				ServiceName:     t.serviceName,
			})
			if err != nil {
				return err
			}
			id, err = res.Base.Id, err
			if err != nil {
				return err
			}
			return nil
		}, retry.LastErrorOnly(true), retry.Attempts(3)); err != nil {
			logger.Errorf(context.Background(), "UpdateStepRecord failed %s", err.Error())
			return "", err
		}
		return id, nil
	}
}

func (t *TaskRecorder) RecordTask(taskID string, result task.TaskExecResult) (id string, err error) {
	conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(context.Background()), trpc.CenterServer)
	if err != nil {
		return "", err
	}
	if taskID == "" {
		var id string
		if err := retry.Do(func() error {
			res, err := system.NewTaskRecordServiceClient(conn).SaveTaskRecord(context.Background(), &system.TaskRecord{
				Status:          string(result.Status),
				Order:           int64(result.Order),
				StepName:        result.StepName,
				StepID:          result.StepID,
				ProcessPosition: int64(result.ProcessPosition),
				Err:             IfeStr(result.Err, ""),
				Payload:         result.Payload,
				Result:          result.Result,
				ServiceName:     t.serviceName,
				TaskCode:        result.TaskCode,
			})
			if err != nil {
				return err
			}
			id, err = res.Base.Id, err
			if err != nil {
				return err
			}
			return nil
		}, retry.LastErrorOnly(true), retry.Attempts(3)); err != nil {
			logger.Errorf(context.Background(), "RecordTask failed %s", err.Error())
			return "", err
		}
		return id, nil
	} else {
		var id string
		if err := retry.Do(func() error {
			res, err := system.NewTaskRecordServiceClient(conn).UpdateTaskRecord(context.Background(), &system.TaskRecord{
				Base: &system.Base{
					Id: taskID,
				},
				Status:          string(result.Status),
				Order:           int64(result.Order),
				StepName:        result.StepName,
				StepID:          result.StepID,
				ProcessPosition: int64(result.ProcessPosition),
				Err:             IfeStr(result.Err, ""),
				Payload:         result.Payload,
				Result:          result.Result,
				ServiceName:     t.serviceName,
				TaskCode:        result.TaskCode,
			})
			if err != nil {
				return err
			}
			id, err = res.Base.Id, err
			if err != nil {
				return err
			}
			return nil
		}, retry.LastErrorOnly(true), retry.Attempts(3)); err != nil {
			logger.Errorf(context.Background(), "UpdateTaskRecord failed %s", err.Error())
			return "", err
		}
		return id, nil
	}

}
