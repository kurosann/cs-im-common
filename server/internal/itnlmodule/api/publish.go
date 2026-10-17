package api

import (
	"context"

	"vane-common/publish"
	"vane-common/v1/pilot/system"
)

var (
	_ system.PublisherInternalServiceServer = (*PublishApi)(nil)
	_ system.PublisherServiceServer         = (*PublishApi)(nil)
	_ system.SubscriberServiceServer        = (*PublishApi)(nil)
)

type PublishApi struct {
	system.UnimplementedPublisherInternalServiceServer
	system.UnimplementedSubscriberServiceServer
	system.UnimplementedPublisherServiceServer
}

func NewPublishApi() *PublishApi {
	return &PublishApi{}
}

func (p *PublishApi) Subscribe(req *system.SubscribeReq, stream system.SubscriberService_SubscribeServer) error {
	ch, cancel, err := publish.Subscribe(stream.Context(), req.Topic)
	if err != nil {
		return err
	}
	defer cancel()
	for {
		select {
		case <-stream.Context().Done():
			return nil
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			if err := stream.Send(&system.SubscribeResp{Topic: req.Topic, Data: msg}); err != nil {
				return err
			}
		}
	}
}

func (p *PublishApi) Send(ctx context.Context, req *system.SendReq) (*system.SendResp, error) {
	publish.PublishLocal(ctx, req.Topic, req.Data)
	return &system.SendResp{}, nil
}

func (p *PublishApi) Update(ctx context.Context, req *system.UpdateReq) (*system.UpdateResp, error) {
	if err := publish.Update(ctx, req.Topic, req.Endpoints); err != nil {
		return nil, err
	}
	return &system.UpdateResp{}, nil
}

func (p *PublishApi) Publish(ctx context.Context, req *system.PublishReq) (*system.PublishResp, error) {
	if err := publish.Publish(ctx, req.Topic, req.Data); err != nil {
		return nil, err
	}
	return &system.PublishResp{}, nil
}
