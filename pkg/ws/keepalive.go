package ws

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
	"vane-common/pkg/logger"
	"vane-common/pkg/safe"
)

var (
	ErrDialContext = errors.New("dial error")
)

type MsgHandler interface {
	OnReceive(msg []byte) error
	OnError(err error)
	OnReconnect() (msg [][]byte)
}

type KeepAliver struct {
	Address      string        // 连接地址
	Dialer       Dialer        // dialer
	MsgHandler   MsgHandler    // 消息处理器
	Delay        time.Duration // 重连间隔 default: 10s
	ReadTimeout  time.Duration // 读取超时
	WriteTimeout time.Duration // 写入超时

	conn   *Conn
	wg     sync.WaitGroup
	rw     sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once
}

func (k *KeepAliver) KeepAlive(connCtx context.Context) (context.Context, error) {
	var dialErr error
	k.once.Do(func() {
		if k.MsgHandler == nil {
			k.MsgHandler = &defaultMsgHandler{}
		}
		if k.Delay == 0 {
			k.Delay = 10 * time.Second
		}
		if k.ReadTimeout == 0 {
			k.ReadTimeout = 30 * time.Second
		}
		if k.WriteTimeout == 0 {
			k.WriteTimeout = 30 * time.Second
		}
		k.ctx, k.cancel = context.WithCancel(context.Background())
		k.wg.Add(1)
		safe.Go(func() {
			defer k.cancel()
			for {
				if err := func() error {
					c, err := k.Dialer.Dial(connCtx, k.Address)
					if err != nil {
						return k.IifCtxErr(k.ctx, fmt.Errorf("%w: %w", ErrDialContext, err))
					}
					defer c.Close()
					k.wg.Done()

					k.rw.Lock()
					k.conn = c
					k.rw.Unlock()

					for _, msg := range k.MsgHandler.OnReconnect() {
						if err = k.Send(msg); err != nil {
							return k.IifCtxErr(k.ctx, err)
						}
					}
					for {
						if k.ReadTimeout != 0 {
							c.SetReadDeadline(time.Now().Add(k.ReadTimeout))
						}
						msg, err := c.Read()
						if err != nil {
							return k.IifCtxErr(k.ctx, err)
						}
						if err := k.MsgHandler.OnReceive([]byte(msg)); err != nil {
							continue
						}
					}
				}(); err != nil {
					// 异常情况重连
					k.MsgHandler.OnError(err)
					logger.Debug(k.ctx, "keepalive error", zap.Error(err))
					if errors.Is(err, ErrDialContext) {
						k.wg.Done()
						dialErr = err
						return
					}
					time.Sleep(k.Delay)
					continue
				}
				return
			}
		})
	})
	k.wg.Wait()
	logger.Debug(k.ctx, "keepalive done", zap.Error(dialErr))
	return k.ctx, dialErr
}
func (k *KeepAliver) IifCtxErr(ctx context.Context, err error) error {
	select {
	case <-ctx.Done():
		return nil
	default:
		if errors.Is(err, context.Canceled) {
			return nil
		}
		return err
	}
}
func (k *KeepAliver) Close() {
	k.wg.Wait()
	k.rw.Lock()
	defer k.rw.Unlock()
	// 先关闭上下文
	k.cancel()
	// 再关闭连接(不产生错误)
	k.conn.Close()
	k.wg = sync.WaitGroup{}
	k.once = sync.Once{}
}

func (k *KeepAliver) IsAlive() bool {
	k.wg.Wait()
	k.rw.RLock()
	defer k.rw.RUnlock()
	return k.conn != nil && k.conn.IsAlive()
}
func (k *KeepAliver) Reconnect() {
	k.wg.Wait()
	k.rw.Lock()
	defer k.rw.Unlock()
	k.rec()
}
func (k *KeepAliver) rec() {
	// 只关闭连接 (产生错误)
	k.conn.Close()
}

func (k *KeepAliver) Send(msg any) error {
	k.wg.Wait()
	k.rw.RLock()
	defer k.rw.RUnlock()
	if k.conn == nil || !k.conn.IsAlive() {
		return errors.New("conn is dead")
	}

	if k.WriteTimeout != 0 {
		k.conn.SetWriteDeadline(time.Now().Add(k.WriteTimeout))
	}
	err := k.conn.Send(msg)
	if err != nil {
		k.rec()
		return k.conn.Send(msg)
	}
	return nil
}

type defaultMsgHandler struct{}

func (d *defaultMsgHandler) OnReceive(msg []byte) error {
	return nil
}
func (d *defaultMsgHandler) OnError(err error) {
}
func (d *defaultMsgHandler) OnReconnect() (msg [][]byte) {
	return
}
