package safe

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTry(t *testing.T) {
	tests := []struct {
		name    string
		args    func()
		wantErr bool
	}{
		{name: "panic", args: func() {
			panic("panic")
		}, wantErr: true},
		{name: "not problem", args: func() {
		}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Try(tt.args); (err != nil) != tt.wantErr {
				t.Errorf("TryE() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTryE(t *testing.T) {
	tests := []struct {
		name    string
		args    func() error
		wantErr bool
	}{
		{name: "panic", args: func() error {
			panic("panic")
		}, wantErr: true},
		{name: "error", args: func() error {
			return errors.New("error")
		}, wantErr: true},
		{name: "not problem", args: func() error {
			return nil
		}, wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := TryE(tt.args); (err != nil) != tt.wantErr {
				t.Errorf("TryE() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWaitGroup_WaitAndRecover(t *testing.T) {
	t.Run("没有panic和错误", func(t *testing.T) {
		wg := NewWaitGroup()
		wg.Go(func() {})
		wg.GoE(func() error { return nil })
		if err := wg.WaitAndRecover(); err != nil {
			t.Errorf("预期无错误，实际得到: %v", err)
		}
	})

	t.Run("只有错误", func(t *testing.T) {
		wg := NewWaitGroup()
		expectedErr := errors.New("test error")
		wg.GoE(func() error { return expectedErr })
		err := wg.WaitAndRecover()
		if !errors.Is(err, expectedErr) {
			t.Errorf("预期错误 %v，实际得到: %v", expectedErr, err)
		}
	})

	t.Run("只有panic", func(t *testing.T) {
		wg := NewWaitGroup()
		wg.Go(func() { panic("test panic") })
		err := wg.WaitAndRecover()
		if err == nil || err.Error() != "panic: test panic" {
			t.Errorf("预期panic错误，实际得到: %v", err)
		}
	})

	t.Run("混合错误和panic", func(t *testing.T) {
		wg := NewWaitGroup()
		err1 := errors.New("error 1")
		err2 := errors.New("error 2")

		wg.GoE(func() error { return err1 })
		wg.Go(func() { panic("test panic") })
		wg.GoE(func() error { return err2 })
		errs := []error{err1, err2, errors.New("panic: test panic")}

		err := wg.WaitAndRecover()
		// 乱序输出
		for _, e := range errs {
			if !strings.Contains(err.Error(), e.Error()) {
				t.Errorf("预期合并错误 %v，实际得到: %v", errs, err)
			}
		}
	})
}

func TestGoAndWait(t *testing.T) {
	t.Run("正常执行", func(t *testing.T) {
		if err := GoAndWait(func() {}); err != nil {
			t.Errorf("预期无错误，实际得到: %v", err)
		}
	})

	t.Run("单个panic", func(t *testing.T) {
		err := GoAndWait(func() { panic("test panic") })
		if err == nil {
			t.Fatalf("预期错误，实际得到: %v", err)
		}
	})

	t.Run("多个panic混合", func(t *testing.T) {
		err := GoAndWait(
			func() { panic("panic1") },
			func() {},
			func() { panic("panic2") },
		)

		if err == nil {
			t.Fatalf("预期错误，实际得到: %v", err)
		}
	})
}

func TestGoCtxEx(t *testing.T) {
	wg := NewWaitGroup()
	dict := make(map[string]int)
	wg.GoCtxEx(func(ctx *WaitGroupContext) error {
		ctx.Lock()
		defer ctx.Unlock()
		dict["test1"] = 1
		return nil
	})
	wg.GoCtxEx(func(ctx *WaitGroupContext) error {
		ctx.Lock()
		defer ctx.Unlock()
		dict["test2"] = 2
		return nil
	})
	wg.GoCtxEx(func(ctx *WaitGroupContext) error {
		ctx.Lock()
		defer ctx.Unlock()
		dict["test3"] = 3
		return nil
	})
	wg.GoCtxEx(func(ctx *WaitGroupContext) error {
		ctx.Lock()
		defer ctx.Unlock()
		time.Sleep(1 * time.Second)
		dict["test4"] = 4
		return nil
	})
	err := wg.WaitAndRecover()
	assert.NoError(t, err)
	assert.Equal(t, dict, map[string]int{"test1": 1, "test2": 2, "test3": 3, "test4": 4})
}
