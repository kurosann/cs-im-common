package trpc

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/grpc/credentials"

	"vane-common/pkg/clock"
)

type ClientPool struct {
	resolverMode DiscoverResolverMode
	svcConnMap   sync.Map
	tlsSelector  *clientSelectorCreds
	mu           sync.RWMutex
}

func NewClientPool(mode DiscoverResolverMode) *ClientPool {
	return &ClientPool{
		resolverMode: mode,
		tlsSelector:  newClientSelectorCreds(),
	}
}

func (p *ClientPool) SetClientCreds(host string, c credentials.TransportCredentials) {
	p.tlsSelector.SetClientCreds(host, c)
}

func (p *ClientPool) GetSvcConn(ctx context.Context, svcName SvcName) (*ClientConn, error) {
	token := getTokenKey(ctx)
	key := fmt.Sprintf("%s-%s", svcName, token)

	if clientConn, ok := p.getClientConn(key); ok {
		return clientConn, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if clientConn, ok := p.getClientConn(key); ok {
		return clientConn, nil
	}

	conn, err := NewTokenClient(
		svcName,
		token,
		WithDiscoverResolverMode(p.resolverMode),
		WithCred(p.tlsSelector),
	)
	if err != nil {
		return nil, err
	}
	return p.setClientConn(key, conn), nil
}
func (p *ClientPool) GetSvcConnWithHost(ctx context.Context, host string) (*ClientConn, error) {
	token := getTokenKey(ctx)
	key := fmt.Sprintf("%s-%s", host, token)

	if clientConn, ok := p.getClientConn(key); ok {
		return clientConn, nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if clientConn, ok := p.getClientConn(key); ok {
		return clientConn, nil
	}

	conn, err := NewTokenClientWithHost(host, token)
	if err != nil {
		return nil, err
	}
	return p.setClientConn(key, conn), nil
}
func (p *ClientPool) getClientConn(key string) (*ClientConn, bool) {
	if conn, ok := p.svcConnMap.Load(key); ok {
		if clientConn, ok2 := conn.(*ClientConn); ok2 && clientConn != nil {
			clientConn.lastUsed.Store(clock.Now().Unix())
			return clientConn, true
		}
	}
	return nil, false
}
func (p *ClientPool) setClientConn(key string, conn *CanCloseClientConn) *ClientConn {
	c := &ClientConn{ccc: conn}
	c.lastUsed.Store(clock.Now().Unix())
	p.svcConnMap.Store(key, c)
	return c
}
func (p *ClientPool) CleanSvcConn() {
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
