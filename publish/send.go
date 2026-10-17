package publish

import (
	"context"
)

type sendMsg struct {
}

func newSendMsg() *sendMsg {
	return &sendMsg{}
}

func (s *sendMsg) Send(ctx context.Context, endpoint, topic string, msg []byte) error {
	return remoteTransport.SendToRemote(ctx, endpoint, topic, msg)
}
