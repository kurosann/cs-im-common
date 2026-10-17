package publish

import (
	"context"
	"vane-common/pkg/logger"
	"vane-common/pkg/safe"

	"go.uber.org/zap"
)

type IPublisher interface {
	Start(ctx context.Context) error
	Stop()
	Send(ctx context.Context, topic string, msg []byte) error
	IsAlive(ctx context.Context) bool
}

type publishManager struct {
	ctx               context.Context
	cancel            context.CancelFunc
	remoteDataChan    chan *publishData
	subscriberManager *subscriberManager
	sendMsg           *sendMsg
	localhost         *localNetwork
}

type publishData struct {
	topic string
	msg   []byte
}

func newPublishManager(dataChan chan *publishData, subscriberManager *subscriberManager, sendMsg *sendMsg, localhost *localNetwork) *publishManager {
	return &publishManager{
		remoteDataChan:    dataChan,
		subscriberManager: subscriberManager,
		sendMsg:           sendMsg,
		localhost:         localhost,
	}
}

func (p *publishManager) ReloadLocalhost(localhost *localNetwork) {
	p.localhost = localhost
}

func (p *publishManager) Start(ctx context.Context) error {
	p.ctx, p.cancel = context.WithCancel(ctx)
	safe.GoKeep(p.ctx, 0, func() {
		for {
			select {
			case <-p.ctx.Done():
				return
			case data := <-p.remoteDataChan:
				logger.Debugf(p.ctx, "send msg to %s, topic: %s, msg: %s", p.localhost.LocalNetwork(), data.topic, string(data.msg))
				p.subscriberManager.SendLocalSubcribe(p.ctx, data.topic, data.msg)
				endpointMap, err := p.subscriberManager.QuerySubcribe(p.ctx, data.topic)
				if err != nil {
					logger.Error(p.ctx, "query endpoint error", zap.Error(err))
					continue
				}
				for endpoint := range endpointMap {
					if endpoint == p.localhost.LocalNetwork() {
						continue
					}
					p.sendMsg.Send(p.ctx, endpoint, data.topic, data.msg)
				}
			}
		}
	})
	return nil
}

func (p *publishManager) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *publishManager) Send(ctx context.Context, topic string, msg []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case p.remoteDataChan <- &publishData{
		topic: topic,
		msg:   msg,
	}:
		return nil
	}
}

func (p *publishManager) IsAlive(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return false
	default:
		return true
	}
}
