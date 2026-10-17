package proxies

import (
	"context"
	"net"
	"time"
)

// bindConnToDialContext 只在建立连接阶段将 conn 绑定到 ctx。
// 连接建立成功后调用返回的 release，避免后续业务读写继续受该 ctx 影响。
func bindConnToDialContext(ctx context.Context, conn net.Conn) (release func()) {
	if conn == nil {
		return func() {}
	}

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	stop := context.AfterFunc(ctx, func() {
		_ = conn.Close()
	})

	return func() {
		stop()
		_ = conn.SetDeadline(time.Time{})
	}
}

func dialWithContextFallback(ctx context.Context, dial func() (net.Conn, error)) (net.Conn, error) {
	type dialResult struct {
		conn net.Conn
		err  error
	}

	resultCh := make(chan dialResult, 1)
	go func() {
		conn, err := dial()
		select {
		case resultCh <- dialResult{conn: conn, err: err}:
		default:
			if conn != nil {
				_ = conn.Close()
			}
		}
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-resultCh:
		return result.conn, result.err
	}
}
