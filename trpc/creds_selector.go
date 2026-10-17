package trpc

import (
	"context"
	"errors"
	"net"
	"sync"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

var _ credentials.TransportCredentials = (*clientSelectorCreds)(nil)

type clientSelectorCreds struct {
	creds map[string]credentials.TransportCredentials
	mu    sync.RWMutex
}

func newClientSelectorCreds() *clientSelectorCreds {
	return &clientSelectorCreds{
		creds: make(map[string]credentials.TransportCredentials),
	}
}

func (s *clientSelectorCreds) SetClientCreds(host string, c credentials.TransportCredentials) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.creds[host] = c
}

func (s *clientSelectorCreds) ClientHandshake(ctx context.Context, authority string, conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	creds := s.getCredsForHandshake(authority)
	return creds.ClientHandshake(ctx, authority, conn)
}

func (s *clientSelectorCreds) ServerHandshake(conn net.Conn) (net.Conn, credentials.AuthInfo, error) {
	return nil, nil, errors.New("selectorCreds is for client only")
}

func (s *clientSelectorCreds) Info() credentials.ProtocolInfo {
	return credentials.ProtocolInfo{}
}

func (s *clientSelectorCreds) Clone() credentials.TransportCredentials {
	return &clientSelectorCreds{}
}

func (s *clientSelectorCreds) OverrideServerName(name string) error {
	return nil
}

func (s *clientSelectorCreds) getCredsForHandshake(authority string) credentials.TransportCredentials {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if c, ok := s.creds[authority]; ok {
		return c
	}
	if host, _, err := net.SplitHostPort(authority); err == nil {
		if c, ok := s.creds[host]; ok {
			return c
		}
	}
	return insecure.NewCredentials()
}
