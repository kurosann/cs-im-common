package safe

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"

	"vane-common/pkg/logger"

	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/conc/panics"
	"github.com/sourcegraph/conc/pool"
)

var recoverFunc = func(err any) {}

func SetRecoverFunc(f func(err any)) {
	recoverFunc = f
}

var _ context.Context = (*WaitGroupContext)(nil)

type WaitGroupContext struct {
	ctx    context.Context
	cancel context.CancelFunc

	shareLock sync.RWMutex
}

func (w *WaitGroupContext) Deadline() (deadline time.Time, ok bool) {
	return w.ctx.Deadline()
}

func (w *WaitGroupContext) Done() <-chan struct{} {
	return w.ctx.Done()
}

func (w *WaitGroupContext) Err() error {
	return w.ctx.Err()
}

func (w *WaitGroupContext) Value(key any) any {
	return w.ctx.Value(key)
}

func (w *WaitGroupContext) Lock() {
	w.shareLock.Lock()
}

func (w *WaitGroupContext) Unlock() {
	w.shareLock.Unlock()
}

func (w *WaitGroupContext) RLock() {
	w.shareLock.RLock()
}

func (w *WaitGroupContext) RUnlock() {
	w.shareLock.RUnlock()
}

type WaitGroup struct {
	ctx *WaitGroupContext
	conc.WaitGroup
	errs []error

	Recovered *panics.Recovered
}

func NewWaitGroup() *WaitGroup {
	return NewWaitGroupContext(context.Background())
}

func NewWaitGroupContext(ctx context.Context) *WaitGroup {
	ctx, cancel := context.WithCancel(ctx)
	return &WaitGroup{
		ctx: &WaitGroupContext{
			ctx:    ctx,
			cancel: cancel,
		},
	}
}

// Go 安全启动goroutines
func (g *WaitGroup) Go(f func()) {
	g.WaitGroup.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				g.ctx.cancel()
				logger.Errorf(g.ctx, "panic: %v %s", r, string(debug.Stack()))
				recoverFunc(r)
				g.errs = append(g.errs, fmt.Errorf("panic: %v", r))
			}
		}()
		f()
	})
}

func (g *WaitGroup) GoCtx(f func(ctx context.Context)) {
	g.WaitGroup.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				g.ctx.cancel()
				logger.Errorf(g.ctx, "panic: %v %s", r, string(debug.Stack()))
				recoverFunc(r)
				g.errs = append(g.errs, fmt.Errorf("panic: %v", r))
			}
		}()
		f(g.ctx)
	})
}

func (g *WaitGroup) GoE(f func() error) {
	g.WaitGroup.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				g.ctx.cancel()
				logger.Errorf(g.ctx, "panic: %v %s", r, string(debug.Stack()))
				recoverFunc(r)
				g.errs = append(g.errs, fmt.Errorf("panic: %v", r))
			}
		}()
		if err := f(); err != nil {
			g.errs = append(g.errs, err)
			g.ctx.cancel()
		}
	})
}

func (g *WaitGroup) GoCtxE(f func(ctx context.Context) error) {
	g.WaitGroup.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				g.ctx.cancel()
				logger.Errorf(g.ctx, "panic: %v %s", r, string(debug.Stack()))
				recoverFunc(r)
				g.errs = append(g.errs, fmt.Errorf("panic: %v", r))
			}
		}()
		if err := f(g.ctx); err != nil {
			g.errs = append(g.errs, err)
			g.ctx.cancel()
		}
	})
}

func (g *WaitGroup) GoCtxEx(f func(ctx *WaitGroupContext) error) {
	g.WaitGroup.Go(func() {
		defer func() {
			if r := recover(); r != nil {
				g.ctx.cancel()
				logger.Errorf(g.ctx, "panic: %v %s", r, string(debug.Stack()))
				recoverFunc(r)
				g.errs = append(g.errs, fmt.Errorf("panic: %v", r))
			}
		}()
		if err := f(g.ctx); err != nil {
			g.errs = append(g.errs, err)
			g.ctx.cancel()
		}
	})
}

// WaitAndRecover 等待并捕获panic
func (g *WaitGroup) WaitAndRecover() error {
	g.Recovered = g.WaitGroup.WaitAndRecover()
	if g.Recovered != nil {
		g.errs = append(g.errs, g.Recovered.AsError())
	}
	if len(g.errs) > 0 {
		return errors.Join(g.errs...)
	}
	return g.ctx.Err()
}

func Go(f func(), fs ...func()) {
	group := NewWaitGroup()
	group.Go(f)
	if len(fs) > 0 {
		for _, fn := range fs {
			group.Go(fn)
		}
	}
}

// GoKeep 安全启动goroutines，并保持运行
func GoKeep(ctx context.Context, gap time.Duration, fs ...func()) {
	defaultGap := 10 * time.Second
	if gap < 0 {
		gap = defaultGap
	}
	Go(func() {
		if err := GoAndWait(fs...); err != nil {
			logger.Errorf(ctx, "GoKeep error: %v", err)
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(gap):
				if err := GoAndWait(fs...); err != nil {
					logger.Errorf(ctx, "GoKeep error: %v", err)
				}
			}
		}
	})
}

// Go 安全启动goroutines
func GoAndWait(fs ...func()) error {
	group := NewWaitGroup()
	for _, fn := range fs {
		group.Go(fn)
	}
	return group.WaitAndRecover()
}

// Catcher 捕获器
type Catcher struct {
	panics.Catcher
	err error

	Recovered *panics.Recovered
}

// AsError 结果转error
func (c *Catcher) AsError() error {
	if c.err != nil {
		return c.err
	}
	c.Recovered = c.Catcher.Recovered()
	if c.Recovered == nil {
		return nil
	}
	return c.Recovered.AsError()
}

// TryE 带有error的panic错误捕获
func (c *Catcher) TryE(f func() error) {
	c.Try(func() { c.err = f() })
}

// Try panic错误捕获
func Try(f func()) error {
	c := Catcher{}
	c.Try(f)
	return c.AsError()
}

// TryE 带有error的panic错误捕获
func TryE(f func() error) error {
	c := Catcher{}
	c.TryE(f)
	return c.AsError()
}

type Pool struct {
	ctx    context.Context
	cancel context.CancelFunc
	pool   *pool.ContextPool
	errs   []error
}

func NewPool(maxSize int) *Pool {
	return NewPoolContext(context.Background(), maxSize)
}

func NewPoolContext(ctx context.Context, maxSize int) *Pool {
	ctx, cancel := context.WithCancel(ctx)
	return &Pool{
		ctx:    ctx,
		cancel: cancel,
		pool:   pool.New().WithContext(ctx).WithMaxGoroutines(maxSize),
	}
}

// Go 安全启动goroutines
func (g *Pool) Go(f func()) {
	g.pool.Go(func(ctx context.Context) error {
		defer func() {
			if r := recover(); r != nil {
				g.cancel()
				logger.Errorf(g.ctx, "panic: %v", r)
				recoverFunc(r)
				g.errs = append(g.errs, fmt.Errorf("panic: %v", r))
			}
		}()
		f()
		return nil
	})
}

func (g *Pool) GoCtx(f func(ctx context.Context)) {
	g.pool.Go(func(ctx context.Context) error {
		defer func() {
			if r := recover(); r != nil {
				g.cancel()
				logger.Errorf(g.ctx, "panic: %v", r)
				recoverFunc(r)
				g.errs = append(g.errs, fmt.Errorf("panic: %v", r))
			}
		}()
		f(g.ctx)
		return nil
	})
}

func (g *Pool) GoE(f func() error) {
	g.pool.Go(func(ctx context.Context) error {
		defer func() {
			if r := recover(); r != nil {
				g.cancel()
				logger.Errorf(g.ctx, "panic: %v", r)
				recoverFunc(r)
				g.errs = append(g.errs, fmt.Errorf("panic: %v", r))
			}
		}()
		if err := f(); err != nil {
			g.errs = append(g.errs, err)
			g.cancel()
		}
		return nil
	})
}

func (g *Pool) GoCtxE(f func(ctx context.Context) error) {
	g.pool.Go(func(ctx context.Context) error {
		defer func() {
			if r := recover(); r != nil {
				g.cancel()
				logger.Errorf(g.ctx, "panic: %v", r)
				recoverFunc(r)
				g.errs = append(g.errs, fmt.Errorf("panic: %v", r))
			}
		}()
		if err := f(g.ctx); err != nil {
			g.errs = append(g.errs, err)
			g.cancel()
		}
		return nil
	})
}

// WaitAndRecover 等待并捕获panic
func (g *Pool) WaitAndRecover() error {
	g.pool.Wait()
	if g.errs != nil {
		return errors.Join(g.errs...)
	}
	return g.ctx.Err()
}
