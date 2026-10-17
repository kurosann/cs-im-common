package depend

import (
	"context"
	"vane-common/publish"
	"vane-common/trpc"
	system "vane-common/v1/pilot/system"
)

var _ publish.IRemoteTransport = (*PublishRemoteTransportService)(nil)

type PublishRemoteTransportService struct {
}

func NewPublishRemoteTransportService() *PublishRemoteTransportService {
	return &PublishRemoteTransportService{}
}

func (p *PublishRemoteTransportService) SendToRemote(ctx context.Context, endpoint string, topic string, data []byte) error {
	conn, err := trpc.GetSvcConnWithHost(ctx, endpoint)
	if err != nil {
		return err
	}
	api := system.NewPublisherInternalServiceClient(conn)
	_, err = api.Send(ctx, &system.SendReq{Topic: topic, Data: data})
	return err
}
