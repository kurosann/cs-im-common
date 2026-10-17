package depend

import (
	"context"
	"time"
	"vane-common/trpc"
	"vane-common/v1/pilot/system"
)

type IDistributeLock interface {
	Lock(ctx context.Context, key string, ttl time.Duration) error
	Unlock(ctx context.Context, key string) error
	KeepAlive(ctx context.Context, key string, ttl time.Duration) error
	LockWithValue(ctx context.Context, key string, value string, ttl time.Duration) error      // 带值的锁
	UnlockWithValue(ctx context.Context, key string, value string) error                       // 带值的解锁
	KeepAliveWithValue(ctx context.Context, key string, value string, ttl time.Duration) error // 带值的续约锁
}
type DistributeLock struct {
	serviceName string
}

func NewDistributeLock(serviceName string) *DistributeLock {
	return &DistributeLock{serviceName: serviceName}
}

func (d *DistributeLock) Lock(ctx context.Context, key string, ttl time.Duration) error {
	conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), trpc.CenterServer)
	if err != nil {
		return err
	}

	_, err = system.NewDistributeLockServiceClient(conn).Lock(ctx, &system.DistributeLock{
		Key:         key,
		Ttl:         int64(ttl.Seconds()),
		ServiceName: d.serviceName,
	})
	return err
}

func (d *DistributeLock) Unlock(ctx context.Context, key string) error {
	conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), trpc.CenterServer)
	if err != nil {
		return err
	}

	_, err = system.NewDistributeLockServiceClient(conn).Unlock(ctx, &system.DistributeLock{
		Key:         key,
		ServiceName: d.serviceName,
	})
	return err
}

func (d *DistributeLock) KeepAlive(ctx context.Context, key string, ttl time.Duration) error {
	conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), trpc.CenterServer)
	if err != nil {
		return err
	}

	_, err = system.NewDistributeLockServiceClient(conn).KeepAlive(ctx, &system.DistributeLock{
		Key:         key,
		Ttl:         int64(ttl.Seconds()),
		ServiceName: d.serviceName,
	})
	return err
}
func (d *DistributeLock) LockWithValue(ctx context.Context, key string, value string, ttl time.Duration) error {
	conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), trpc.CenterServer)
	if err != nil {
		return err
	}

	_, err = system.NewDistributeLockServiceClient(conn).Lock(ctx, &system.DistributeLock{
		Key:         key,
		Value:       value,
		Ttl:         int64(ttl.Seconds()),
		ServiceName: d.serviceName,
	})
	return err
}

func (d *DistributeLock) KeepAliveWithValue(ctx context.Context, key string, value string, ttl time.Duration) error {
	conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), trpc.CenterServer)
	if err != nil {
		return err
	}
	_, err = system.NewDistributeLockServiceClient(conn).KeepAlive(ctx, &system.DistributeLock{
		Key:         key,
		Value:       value,
		Ttl:         int64(ttl.Seconds()),
		ServiceName: d.serviceName,
	})
	return err
}
func (d *DistributeLock) UnlockWithValue(ctx context.Context, key string, value string) error {
	conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), trpc.CenterServer)
	if err != nil {
		return err
	}
	_, err = system.NewDistributeLockServiceClient(conn).Unlock(ctx, &system.DistributeLock{
		Key:         key,
		Value:       value,
		ServiceName: d.serviceName,
	})
	return err
}

var _ IDistributeLock = (*emptyDistributeLock)(nil)

type emptyDistributeLock struct{}

func (e *emptyDistributeLock) KeepAliveWithValue(ctx context.Context, key string, value string, ttl time.Duration) error {
	return nil
}

func (e *emptyDistributeLock) LockWithValue(ctx context.Context, key string, value string, ttl time.Duration) error {
	return nil
}

func (e *emptyDistributeLock) UnlockWithValue(ctx context.Context, key string, value string) error {
	return nil
}

func (e *emptyDistributeLock) Lock(ctx context.Context, key string, ttl time.Duration) error {
	return nil
}

func (e *emptyDistributeLock) Unlock(ctx context.Context, key string) error {
	return nil
}

func (e *emptyDistributeLock) KeepAlive(ctx context.Context, key string, ttl time.Duration) error {
	return nil
}
