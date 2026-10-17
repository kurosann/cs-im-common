package clock

import (
	"testing"
	"time"
)

// ==================== Unit Tests ====================

func TestNewNtpClock_NoAddrs(t *testing.T) {
	c := NewNtpClock(nil, 10*time.Minute)
	nc, ok := c.(*ntpClock)
	if !ok {
		t.Fatal("expected *ntpClock")
	}
	if !nc.HasSynced() {
		t.Error("expected synced with nil addrs")
	}
	a := nc.anchor.Load()
	if a == nil {
		t.Fatal("anchor is nil")
	}
	if a.syncWall == 0 || a.syncMono == 0 || a.ntpBase == 0 {
		t.Error("anchor fields should be non-zero")
	}
}

func TestNewNtpClock_EmptyAddrs(t *testing.T) {
	c := NewNtpClock([]string{}, 10*time.Minute)
	if !c.HasSynced() {
		t.Error("expected synced with empty addrs")
	}
}

func TestNtpClock_HasSynced_Initial(t *testing.T) {
	c := NewNtpClock(nil, 10*time.Minute)
	if !c.HasSynced() {
		t.Error("expected synced")
	}
}

func TestNtpClock_Now_Basic(t *testing.T) {
	c := NewNtpClock(nil, 10*time.Minute)
	now := c.Now()
	realNow := time.Now()
	diff := realNow.Sub(now)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Errorf("clock.Now() diverges from time.Now() by %v", diff)
	}
}

func TestNtpClock_Now_Monotonic(t *testing.T) {
	c := NewNtpClock(nil, 10*time.Minute)

	t1 := c.Now()
	t2 := c.Now()

	if t2.Before(t1) {
		t.Error("Now() must be monotonic: t2 < t1")
	}
}

func TestNtpClock_Now_SleepDetection(t *testing.T) {
	c := NewNtpClock(nil, 10*time.Minute)
	nc := c.(*ntpClock)

	// 重置 lastSleepCheck 确保休眠检测不被节流
	nc.lastSleepCheck.Store(0)

	// 模拟熄屏 2 小时：将锚点拨到 2 小时前，monotonic 仅经过 1s
	fakeWall := time.Now().Add(-2 * time.Hour).UnixNano()
	fakeMono := nanotime() - int64(1*time.Second)
	nc.anchor.Store(&ntpAnchor{
		ntpBase:  fakeWall,
		syncMono: fakeMono,
		syncWall: fakeWall,
	})

	nc.syncing.Store(0)

	now := c.Now()
	realNow := time.Now()
	diff := realNow.Sub(now)
	if diff < 0 {
		diff = -diff
	}
	if diff > time.Second {
		t.Errorf("sleep detection should fall back to wall clock, diff=%v", diff)
	}
}

func TestNtpClock_Now_SleepCheckThrottle(t *testing.T) {
	c := NewNtpClock(nil, 10*time.Minute)
	nc := c.(*ntpClock)

	// 第一次 Now()：必定做休眠检测（lastSleepCheck=0）
	firstCheck := nc.lastSleepCheck.Load()
	_ = c.Now()
	if nc.lastSleepCheck.Load() == firstCheck {
		t.Error("first Now() should update lastSleepCheck")
	}

	// 100ms 内的调用：跳过休眠检测，lastSleepCheck 不变
	lastAfterFirst := nc.lastSleepCheck.Load()
	for i := 0; i < 100; i++ {
		_ = c.Now()
	}
	if nc.lastSleepCheck.Load() != lastAfterFirst {
		t.Error("Now() within throttle window should NOT update lastSleepCheck")
	}
}

func TestNtpClock_Now_NoPanicOnNilAnchor(t *testing.T) {
	nc := &ntpClock{
		addrs: nil,
		ttl:   10 * time.Minute,
	}
	if nc.Now().IsZero() {
		t.Error("Now() should not return zero even with nil anchor")
	}
}

func TestDefClock_Now(t *testing.T) {
	c := defClock{}
	now := c.Now()
	if now.IsZero() {
		t.Error("defClock.Now() returned zero time")
	}
}

func TestDefClock_HasSynced(t *testing.T) {
	c := defClock{}
	if !c.HasSynced() {
		t.Error("defClock.HasSynced() should always return true")
	}
}

func TestAbs(t *testing.T) {
	tests := []struct {
		input    int64
		expected int64
	}{
		{5, 5},
		{-5, 5},
		{0, 0},
		{int64(time.Hour), int64(time.Hour)},
		{-int64(time.Hour), int64(time.Hour)},
	}
	for _, tt := range tests {
		if got := abs(tt.input); got != tt.expected {
			t.Errorf("abs(%d) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestSetClock_And_GetClock(t *testing.T) {
	original := GetClock()
	defer SetClock(original)

	custom := defClock{}
	SetClock(custom)
	if GetClock() != custom {
		t.Error("GetClock should return the clock set by SetClock")
	}
}

func TestSince(t *testing.T) {
	original := GetClock()
	defer SetClock(original)
	SetClock(defClock{})

	start := Now()
	time.Sleep(10 * time.Millisecond)
	elapsed := Since(start)
	if elapsed < 10*time.Millisecond {
		t.Errorf("Since() = %v, expected at least 10ms", elapsed)
	}
}

func TestNtpAnchor_ConcurrentRead(t *testing.T) {
	c := NewNtpClock(nil, 10*time.Minute)
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			_ = c.Now()
		}
		close(done)
	}()

	// Concurrent reads while writer is reading — atomic.Pointer is safe
	for i := 0; i < 1000; i++ {
		_ = c.Now()
	}
	<-done
}

// ==================== Benchmarks ====================

func BenchmarkNow_NtpClock(b *testing.B) {
	c := NewNtpClock(nil, 10*time.Minute)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Now()
	}
}

func BenchmarkNow_DefClock(b *testing.B) {
	c := defClock{}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Now()
	}
}

func BenchmarkNow_StdTime(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = time.Now()
	}
}

func BenchmarkNow_NtpClock_Parallel(b *testing.B) {
	c := NewNtpClock(nil, 10*time.Minute)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = c.Now()
		}
	})
}

func BenchmarkNow_DefClock_Parallel(b *testing.B) {
	c := defClock{}
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = c.Now()
		}
	})
}

func BenchmarkNow_StdTime_Parallel(b *testing.B) {
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = time.Now()
		}
	})
}

func BenchmarkNanotime(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = nanotime()
	}
}

func BenchmarkNtpClock_HasSynced(b *testing.B) {
	c := NewNtpClock(nil, 10*time.Minute)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.HasSynced()
	}
}

func BenchmarkNowBare_NtpClock(b *testing.B) {
	c := NewNtpClock(nil, 10*time.Minute)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.(*ntpClock).Now()
	}
}

func BenchmarkNowBare_NtpClock_Parallel(b *testing.B) {
	c := NewNtpClock(nil, 10*time.Minute)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = c.(*ntpClock).Now()
		}
	})
}

// BenchmarkNow_NtpClock_FastPath 节流窗口内（GORM NowFunc 热路径）：无 time.Now() 调用
func BenchmarkNow_NtpClock_FastPath(b *testing.B) {
	c := NewNtpClock(nil, 10*time.Minute)
	nc := c.(*ntpClock)
	// 预置 lastSleepCheck，使所有调用落进节流窗口
	nc.lastSleepCheck.Store(nanotime())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Now()
	}
}

// BenchmarkNow_NtpClock_FastPath_Parallel 节流窗口内并发
func BenchmarkNow_NtpClock_FastPath_Parallel(b *testing.B) {
	c := NewNtpClock(nil, 10*time.Minute)
	nc := c.(*ntpClock)
	nc.lastSleepCheck.Store(nanotime())
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = c.Now()
		}
	})
}

// BenchmarkNow_NtpClock_TtlExpired 覆盖 TTL 到期分支（cooldown 未到，不会真正触发 sync）
func BenchmarkNow_NtpClock_TtlExpired(b *testing.B) {
	c := NewNtpClock(nil, 1*time.Nanosecond) // TTL 立即到期
	nc := c.(*ntpClock)
	// 预置 lastAttempt，防止 sync 实际触发
	nc.lastAttempt.Store(nanotime())
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = c.Now()
	}
}
