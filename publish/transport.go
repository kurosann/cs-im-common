package publish

import "context"

type IRemoteTransport interface {
	SendToRemote(ctx context.Context, endpoint, topic string, data []byte) error
}

var (
	remoteTransport IRemoteTransport = &emptyRemoteTransport{}
)

func SetRemoteTransport(transport IRemoteTransport) {
	remoteTransport = transport
}

type emptyRemoteTransport struct{}

func (e *emptyRemoteTransport) SendToRemote(ctx context.Context, endpoint, topic string, data []byte) error {
	return nil
}
