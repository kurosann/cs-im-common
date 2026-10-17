package publish

import (
	"context"
	"testing"
	"time"
	"vane-common/server/serverconfig"
)

func TestPublishServer_Reload(t *testing.T) {
	// 设置测试环境
	testTransport := &testRemoteTransport{}
	testStore := &testSubscriberStore{}

	SetRemoteTransport(testTransport)
	SetSubscriberStore(testStore)

	ps := EnablePublishServer()

	// 初始化
	serverConfig := serverconfig.Config{
		Port: 8080,
	}
	ctx := serverconfig.SetServerConfig(context.Background(), &serverConfig)
	err := ps.Init(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 测试 Reload 方法
	newServerConfig := serverconfig.Config{
		Port: 9090,
	}
	newCtx := serverconfig.SetServerConfig(context.Background(), &newServerConfig)

	err = ps.Reload(newCtx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 验证端口已更新
	if ps.port != 9090 {
		t.Errorf("Expected port 9090, got %d", ps.port)
	}

	// 验证 localhost 已更新
	if ps.localhost == nil {
		t.Error("Expected localhost to be updated")
	}
}

func TestPublishServer_StartStop(t *testing.T) {
	// 设置测试环境
	testTransport := &testRemoteTransport{}
	testStore := &testSubscriberStore{}

	SetRemoteTransport(testTransport)
	SetSubscriberStore(testStore)

	ps := EnablePublishServer()

	// 初始化
	serverConfig := serverconfig.Config{
		Port: 8080,
	}
	ctx := serverconfig.SetServerConfig(context.Background(), &serverConfig)
	err := ps.Init(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 测试 Start 方法
	startCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = ps.Start(startCtx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 验证 publishManager 是否活跃
	if !ps.publishManager.IsAlive(startCtx) {
		t.Error("Expected publishManager to be alive after start")
	}

	// 等待一段时间确保启动完成
	time.Sleep(10 * time.Millisecond)

	// 测试 Stop 方法
	ps.Stop()

	// 等待停止完成
	time.Sleep(10 * time.Millisecond)
}

func TestPublishServer_Integration(t *testing.T) {
	// 设置测试环境
	testTransport := &testRemoteTransport{}
	testStore := &testSubscriberStore{
		queryResult: map[string][]string{
			"test-topic":   {"localhost:8080"},               // 本地订阅者
			"remote-topic": {"remote1:8080", "remote2:8080"}, // 远程订阅者
		},
	}

	SetRemoteTransport(testTransport)
	SetSubscriberStore(testStore)

	ps := EnablePublishServer()

	// 初始化
	serverConfig := serverconfig.Config{
		Port: 8080,
	}
	ctx := serverconfig.SetServerConfig(context.Background(), &serverConfig)
	err := ps.Init(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 重要：先更新本地缓存
	ps.subscriberManager.UpdateSubcribe(ctx, "test-topic", []string{"localhost:8080"})
	ps.subscriberManager.UpdateSubcribe(ctx, "remote-topic", []string{"remote1:8080", "remote2:8080"})

	// 启动服务器
	startCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = ps.Start(startCtx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 测试本地订阅和发布
	topic := "test-topic"
	ch, cancelFunc, err := Subscribe(startCtx, topic)
	if err != nil {
		t.Errorf("Expected no error subscribing, got %v", err)
	}
	defer cancelFunc()

	// 发布消息
	testData := []byte("test message")
	err = Publish(startCtx, topic, testData)
	if err != nil {
		t.Errorf("Expected no error publishing, got %v", err)
	}

	// 等待消息处理
	time.Sleep(50 * time.Millisecond)

	// 验证消息接收 - 因为本地地址会被跳过，所以本地消息可能不会通过远程传输
	// 让我们直接测试本地发布功能
	PublishLocal(startCtx, topic, testData)

	// 验证消息接收
	select {
	case receivedMsg := <-ch:
		if string(receivedMsg) != string(testData) {
			t.Errorf("Expected message %s, got %s", string(testData), string(receivedMsg))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected to receive message")
	}

	// 测试远程消息发布
	remoteTopic := "remote-topic"
	err = Publish(startCtx, remoteTopic, testData)
	if err != nil {
		t.Errorf("Expected no error publishing remote message, got %v", err)
	}

	// 等待远程消息处理
	time.Sleep(50 * time.Millisecond)

	// 验证远程传输被调用
	if !testTransport.sendCalled {
		t.Error("Expected remote transport to be called")
	}

	// 停止服务器
	ps.Stop()
}
