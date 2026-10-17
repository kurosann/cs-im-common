package publish

import (
	"sync"
	"vane-common/pkg/network"

	"context"
	"fmt"
)

type localNetwork struct {
	port         int
	localNetwork string
	lock         sync.RWMutex
}

func newLocalNetwork(port int) *localNetwork {
	return &localNetwork{
		port: port,
	}
}

func (l *localNetwork) LocalNetwork() string {
	if l.localNetwork != "" {
		return l.localNetwork
	}
	l.lock.Lock()
	defer l.lock.Unlock()

	ip, err := network.GetSelfIP(context.Background())
	if err != nil {
		return ""
	}
	l.localNetwork = fmt.Sprintf("%s:%d", ip, l.port)
	return l.localNetwork
}
