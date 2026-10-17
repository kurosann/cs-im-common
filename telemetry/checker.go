package telemetry

import (
	"context"
	"errors"
	"sync"
	"time"
	"vane-common/pkg/logger"
	"vane-common/pkg/safe"

	gosundheit "github.com/AppsFlyer/go-sundheit"
	"github.com/AppsFlyer/go-sundheit/checks"
)

type CheckerItem map[string]any

type HealthChecker interface {
	Check(ctx context.Context) (details CheckerItem, err error)
}

type HealthCheckerFunc func(ctx context.Context) (details CheckerItem, err error)

func (f HealthCheckerFunc) Check(ctx context.Context) (details CheckerItem, err error) {
	return f(ctx)
}

type HealthItemChecker struct {
	gosundheit.Check
}

func NewHealthChecker(name string, checker HealthChecker) *HealthItemChecker {
	return &HealthItemChecker{
		Check: &checks.CustomCheck{
			CheckName: name,
			CheckFunc: func(ctx context.Context) (interface{}, error) {
				item, err := checker.Check(ctx)
				if err != nil {
					return item, err
				}
				return item, nil
			},
		},
	}
}

func NewNoScheduleHealthChecker() gosundheit.Health {
	h := &noScheduleCheckHealth{
		ctx:     context.TODO(),
		results: make(map[string]gosundheit.Result, 16),
	}

	return h
}

type noScheduleCheckHealth struct {
	ctx     context.Context
	results map[string]gosundheit.Result
	lock    sync.RWMutex
	checks  []gosundheit.Check
}

func (h *noScheduleCheckHealth) Deregister(name string) {
}

func (h *noScheduleCheckHealth) DeregisterAll() {
}

func (h *noScheduleCheckHealth) RegisterCheck(check gosundheit.Check, opts ...gosundheit.CheckOption) error {
	if check == nil {
		return errors.New("check must not be nil")
	}
	if check.Name() == "" {
		return errors.New("check name must not be empty")
	}
	h.lock.Lock()
	defer h.lock.Unlock()

	h.checks = append(h.checks, check)
	return nil
}

func (h *noScheduleCheckHealth) Results() (results map[string]gosundheit.Result, healthy bool) {
	wg := safe.NewWaitGroup()

	for _, check := range h.checks {
		wg.Go(func() { h.checkAndUpdateResult(check, time.Now()) })
	}

	if err := wg.WaitAndRecover(); err != nil {
		logger.Errorf(h.ctx, "check results error: %v", err)
	}

	healthy = allHealthy(h.results)
	results = h.results
	return
}

func (h *noScheduleCheckHealth) checkAndUpdateResult(check gosundheit.Check, checkTime time.Time) {
	now := time.Now()
	details, err := check.Execute(h.ctx)
	h.updateResult(check.Name(), details, now.Sub(checkTime), err, now)
}
func (h *noScheduleCheckHealth) IsHealthy() (healthy bool) {
	return allHealthy(h.results)
}

type marshalableError struct {
	Message string `json:"message,omitempty"`
	Cause   error  `json:"cause,omitempty"`
}

func newMarshalableError(err error) error {
	if err == nil {
		return nil
	}

	mr := marshalableError{
		Message: err.Error(),
	}
	cause := errors.Unwrap(err)
	if !errors.Is(cause, err) {
		mr.Cause = newMarshalableError(cause)
	}

	return mr
}

func (e marshalableError) Error() string {
	return e.Message
}

func (h *noScheduleCheckHealth) updateResult(
	name string, details interface{}, checkDuration time.Duration, err error, t time.Time) {

	h.lock.RLock()

	prevResult, ok := h.results[name]
	h.lock.RUnlock()
	result := gosundheit.Result{
		Details:            details,
		Error:              newMarshalableError(err),
		Timestamp:          t,
		Duration:           checkDuration,
		TimeOfFirstFailure: nil,
	}

	if !result.IsHealthy() {
		if ok {
			result.ContiguousFailures = prevResult.ContiguousFailures + 1
			if prevResult.IsHealthy() {
				result.TimeOfFirstFailure = &t
			} else {
				result.TimeOfFirstFailure = prevResult.TimeOfFirstFailure
			}
		} else {
			result.ContiguousFailures = 1
			result.TimeOfFirstFailure = &t
		}
	}
	h.lock.Lock()
	defer h.lock.Unlock()
	h.results[name] = result
}
func allHealthy(results map[string]gosundheit.Result) (healthy bool) {
	for _, v := range results {
		if !v.IsHealthy() {
			return false
		}
	}

	return true
}
