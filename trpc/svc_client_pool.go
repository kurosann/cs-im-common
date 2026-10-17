package trpc

import (
	"context"
	"sync"

	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials"

	"vane-common/pkg/clock"
)

// SvcClientPool 按服务名分池：一个 svc 共用一条 ClientConn，身份由每次 RPC 的
// ctx（CtxAuthToken）动态挂载。不影响原有按 svc+token 分键的 ClientPool。
type SvcClientPool struct {
	resolverMode DiscoverResolverMode
	svcConnMap   sync.Map
	tlsSelector  *clientSelectorCreds
	mu           sync.RWMutex
}

// NewSvcClientPool 创建按服务分键的连接池。
func NewSvcClientPool(mode DiscoverResolverMode) *SvcClientPool {
	return &SvcClientPool{
		resolverMode: mode,
		tlsSelector:  newClientSelectorCreds(),
	}
}

// SetClientCreds 为指定 host 设置 mTLS 传输凭据。
func (p *SvcClientPool) SetClientCreds(host string, c credentials.TransportCredentials) {
	p.tlsSelector.SetClientCreds(host, c)
}

// GetSvcConn 按 svcName 复用连接；token 不进入池 key，由调用方写入 ctx。
// 已 Shutdown 的连接会丢弃并重建，避免 "use of closed network connection"。
func (p *SvcClientPool) GetSvcConn(_ context.Context, svcName SvcName) (*ClientConn, error) {
	key := string(svcName)

	if clientConn, ok := p.getLiveClientConn(key); ok {
		return clientConn, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if clientConn, ok := p.getLiveClientConn(key); ok {
		return clientConn, nil
	}

	conn, err := NewSvcClient(
		svcName,
		WithDiscoverResolverMode(p.resolverMode),
		WithCred(p.tlsSelector),
	)
	if err != nil {
		return nil, err
	}
	return p.setClientConn(key, conn), nil
}

// GetSvcConnWithHost 按 host 复用连接；token 不进入池 key。
func (p *SvcClientPool) GetSvcConnWithHost(_ context.Context, host string) (*ClientConn, error) {
	key := host

	if clientConn, ok := p.getLiveClientConn(key); ok {
		return clientConn, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if clientConn, ok := p.getLiveClientConn(key); ok {
		return clientConn, nil
	}

	conn, err := NewSvcClientWithHost(host, WithCred(p.tlsSelector))
	if err != nil {
		return nil, err
	}
	return p.setClientConn(key, conn), nil
}

// Invalidate 强制丢掉指定 svc 的池内连接（下次 Get 重建）。
func (p *SvcClientPool) Invalidate(svcName SvcName) {
	p.drop(string(svcName))
}

func (p *SvcClientPool) getLiveClientConn(key string) (*ClientConn, bool) {
	if conn, ok := p.svcConnMap.Load(key); ok {
		if clientConn, ok2 := conn.(*ClientConn); ok2 && clientConn != nil {
			if clientConn.GetState() == connectivity.Shutdown {
				p.drop(key)
				return nil, false
			}
			clientConn.lastUsed.Store(clock.Now().Unix())
			return clientConn, true
		}
	}
	return nil, false
}

func (p *SvcClientPool) drop(key string) {
	if conn, ok := p.svcConnMap.LoadAndDelete(key); ok {
		if clientConn, ok2 := conn.(*ClientConn); ok2 && clientConn != nil {
			clientConn.close()
		}
	}
}

func (p *SvcClientPool) setClientConn(key string, conn *CanCloseClientConn) *ClientConn {
	c := &ClientConn{ccc: conn}
	c.lastUsed.Store(clock.Now().Unix())
	p.svcConnMap.Store(key, c)
	return c
}

// CleanSvcConn 清理闲置超过 300s 的连接。
func (p *SvcClientPool) CleanSvcConn() {
	p.svcConnMap.Range(func(key, value interface{}) bool {
		conn, ok := value.(*ClientConn)
		if ok && conn != nil {
			if clock.Now().Unix()-conn.lastUsed.Load() > 300 {
				conn.close()
				p.svcConnMap.Delete(key)
			}
		}
		return true
	})
}
