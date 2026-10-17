package depend

import (
	"context"
	"errors"
	"sync"
	"time"
)

var (
	ErrDistributeLockExists        = errors.New("distribute lock exists")
	ErrDistributeLockNotExists     = errors.New("distribute lock not exists")
	ErrDistributeLockValueMismatch = errors.New("distribute lock value mismatch")
)

var _ IDistributeLock = (*MemoryDistributeLock)(nil)

type MemoryDistributeLock struct {
	mu    sync.Mutex
	locks map[string]memoryLockItem
}

type memoryLockItem struct {
	value     string
	expiresAt time.Time
}

func NewMemoryDistributeLock() *MemoryDistributeLock {
	return &MemoryDistributeLock{
		locks: make(map[string]memoryLockItem),
	}
}

func (l *MemoryDistributeLock) Lock(ctx context.Context, key string, ttl time.Duration) error {
	return l.LockWithValue(ctx, key, "", ttl)
}

func (l *MemoryDistributeLock) Unlock(ctx context.Context, key string) error {
	return l.UnlockWithValue(ctx, key, "")
}

func (l *MemoryDistributeLock) KeepAlive(ctx context.Context, key string, ttl time.Duration) error {
	return l.KeepAliveWithValue(ctx, key, "", ttl)
}

func (l *MemoryDistributeLock) LockWithValue(ctx context.Context, key string, value string, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.cleanExpiredLocked(time.Now())
	if _, ok := l.locks[key]; ok {
		return ErrDistributeLockExists
	}
	l.locks[key] = memoryLockItem{
		value:     value,
		expiresAt: l.expiresAt(ttl),
	}
	return nil
}

func (l *MemoryDistributeLock) UnlockWithValue(ctx context.Context, key string, value string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.cleanExpiredLocked(time.Now())
	item, ok := l.locks[key]
	if !ok {
		return ErrDistributeLockNotExists
	}
	if item.value != value {
		return ErrDistributeLockValueMismatch
	}
	delete(l.locks, key)
	return nil
}

func (l *MemoryDistributeLock) KeepAliveWithValue(ctx context.Context, key string, value string, ttl time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	l.cleanExpiredLocked(time.Now())
	item, ok := l.locks[key]
	if !ok {
		return ErrDistributeLockNotExists
	}
	if item.value != value {
		return ErrDistributeLockValueMismatch
	}
	item.expiresAt = l.expiresAt(ttl)
	l.locks[key] = item
	return nil
}

func (l *MemoryDistributeLock) cleanExpiredLocked(now time.Time) {
	for key, item := range l.locks {
		if !item.expiresAt.IsZero() && now.After(item.expiresAt) {
			delete(l.locks, key)
		}
	}
}

func (l *MemoryDistributeLock) expiresAt(ttl time.Duration) time.Time {
	if ttl <= 0 {
		return time.Time{}
	}
	return time.Now().Add(ttl)
}
