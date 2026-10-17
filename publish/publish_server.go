package publish

import (
	"context"
	"vane-common/server/serverconfig"
)

type PublishServer struct {
	subscriberManager *subscriberManager
	publishManager    *publishManager
	sendMsg           *sendMsg
	localhost         *localNetwork
	port              int
	publishDataChan   chan *publishData
}

func EnablePublishServer() *PublishServer {
	return &PublishServer{}
}

func (p *PublishServer) Init(ctx context.Context) error {
	p.publishDataChan = make(chan *publishData, 100)
	p.sendMsg = newSendMsg()

	serverConfig, ok := serverconfig.GetServerConfig(ctx)
	if ok {
		p.port = serverConfig.Port
	}
	p.subscriberManager = newSubscriberManager()
	p.localhost = newLocalNetwork(p.port)
	p.publishManager = newPublishManager(p.publishDataChan, p.subscriberManager, p.sendMsg, p.localhost)

	setSubscriber(p.subscriberManager)
	setPublisher(p.publishManager)
	return nil
}

func (p *PublishServer) Reload(ctx context.Context) error {
	serverConfig, ok := serverconfig.GetServerConfig(ctx)
	if ok {
		p.port = serverConfig.Port
	}
	p.localhost = newLocalNetwork(p.port)

	p.publishManager.ReloadLocalhost(p.localhost)
	return nil
}

func (p *PublishServer) Start(ctx context.Context) error {
	return p.publishManager.Start(ctx)
}

func (p *PublishServer) Stop() {
	p.publishManager.Stop()
}
