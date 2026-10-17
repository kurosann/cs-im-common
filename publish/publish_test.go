package publish

import (
	"context"
	"testing"
	"time"
)

// 为了测试，我们需要设置一个空的传输实现
type testRemoteTransport struct {
	sendCalled bool
	sendError  error
	sentMsgs   []struct {
		endpoint string
		topic    string
		msg      []byte
	}
}

func (t *testRemoteTransport) SendToRemote(ctx context.Context, endpoint, topic string, data []byte) error {
	t.sendCalled = true
	t.sentMsgs = append(t.sentMsgs, struct {
		endpoint string
		topic    string
		msg      []byte
	}{endpoint, topic, data})
	return t.sendError
}

type testSubscriberStore struct {
	recordCalled bool
	removeCalled bool
	queryCalled  bool
	queryResult  map[string][]string // 改为 map 来支持不同 topic 的查询
	queryError   error
}

func (t *testSubscriberStore) RecordSubscribe(ctx context.Context, topic string) error {
	t.recordCalled = true
	return nil
}

func (t *testSubscriberStore) RemoveSubscribe(ctx context.Context, topic string) error {
	t.removeCalled = true
	return nil
}

func (t *testSubscriberStore) QueryAllSubcribe(ctx context.Context) (map[string][]string, error) {
	t.queryCalled = true
	if t.queryResult == nil {
		t.queryResult = make(map[string][]string)
	}
	return t.queryResult, t.queryError
}

func TestPublishManager_Send(t *testing.T) {
	// 设置测试环境
	testTransport := &testRemoteTransport{}
	testStore := &testSubscriberStore{}

	// 设置全局变量
	SetRemoteTransport(testTransport)
	SetSubscriberStore(testStore)

	dataChan := make(chan *publishData, 100)
	sm := newSubscriberManager()
	sendMsg := newSendMsg()
	localhost := newLocalNetwork(8080)

	pm := newPublishManager(dataChan, sm, sendMsg, localhost)

	ctx := context.Background()
	topic := "test-topic"
	data := []byte("test data")

	// 测试 Send 方法
	err := pm.Send(ctx, topic, data)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 验证数据是否正确发送到 channel
	select {
	case publishData := <-dataChan:
		if publishData.topic != topic {
			t.Errorf("Expected topic %s, got %s", topic, publishData.topic)
		}
		if string(publishData.msg) != string(data) {
			t.Errorf("Expected data %s, got %s", string(data), string(publishData.msg))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected to receive data from channel")
	}
}

func TestPublishManager_Start_LocalMessage(t *testing.T) {
	// 设置测试环境
	testTransport := &testRemoteTransport{}
	testStore := &testSubscriberStore{}

	SetRemoteTransport(testTransport)
	SetSubscriberStore(testStore)

	dataChan := make(chan *publishData, 100)
	sm := newSubscriberManager()
	sendMsg := newSendMsg()
	localhost := newLocalNetwork(8080)

	pm := newPublishManager(dataChan, sm, sendMsg, localhost)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 publishManager
	err := pm.Start(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 订阅本地主题
	localTopic := "local-topic"
	_, ch, err := sm.Subscribe(ctx, localTopic)
	if err != nil {
		t.Errorf("Expected no error subscribing, got %v", err)
	}

	// 发送本地消息 - 直接发送到本地订阅者
	data := []byte("local message")
	sm.SendLocalSubcribe(ctx, localTopic, data)

	// 验证消息接收
	select {
	case receivedMsg := <-ch:
		if string(receivedMsg) != string(data) {
			t.Errorf("Expected message %s, got %s", string(data), string(receivedMsg))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected to receive local message")
	}

	// 停止 publishManager
	pm.Stop()
}

func TestPublishManager_Start_RemoteMessage(t *testing.T) {
	// 设置测试环境
	testTransport := &testRemoteTransport{}
	testStore := &testSubscriberStore{
		queryResult: map[string][]string{
			"remote-topic": {"remote1:8080", "remote2:8080"},
		},
	}

	SetRemoteTransport(testTransport)
	SetSubscriberStore(testStore)

	dataChan := make(chan *publishData, 100)
	sm := newSubscriberManager()
	sendMsg := newSendMsg()
	localhost := newLocalNetwork(8080)

	// 重要：先更新本地缓存
	ctx := context.Background()
	sm.UpdateSubcribe(ctx, "remote-topic", []string{"remote1:8080", "remote2:8080"})

	pm := newPublishManager(dataChan, sm, sendMsg, localhost)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动 publishManager
	err := pm.Start(ctx)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	// 发送远程消息
	remoteTopic := "remote-topic"
	data := []byte("remote message")

	dataChan <- &publishData{topic: remoteTopic, msg: data}

	// 等待处理
	time.Sleep(50 * time.Millisecond)

	// 验证远程发送被调用
	if !testTransport.sendCalled {
		t.Error("Expected remote transport Send to be called")
	}

	// 验证发送的消息数量
	if len(testTransport.sentMsgs) != 2 {
		t.Errorf("Expected 2 sent messages, got %d", len(testTransport.sentMsgs))
	}

	// 停止 publishManager
	pm.Stop()
}

func TestPublishManager_IsAlive(t *testing.T) {
	dataChan := make(chan *publishData, 100)
	sm := newSubscriberManager()
	sendMsg := newSendMsg()
	localhost := newLocalNetwork(8080)

	pm := newPublishManager(dataChan, sm, sendMsg, localhost)

	ctx := context.Background()
	if !pm.IsAlive(ctx) {
		t.Error("Expected publishManager to be alive")
	}

	// 测试取消的 context
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	if pm.IsAlive(canceledCtx) {
		t.Error("Expected publishManager to be not alive with canceled context")
	}
}

func TestPublishManager_ReloadLocalhost(t *testing.T) {
	dataChan := make(chan *publishData, 100)
	sm := newSubscriberManager()
	sendMsg := newSendMsg()
	oldLocal := newLocalNetwork(8080)

	pm := newPublishManager(dataChan, sm, sendMsg, oldLocal)

	// 重新加载 localhost
	newLocal := newLocalNetwork(9090)
	pm.ReloadLocalhost(newLocal)

	// 验证 localhost 已更新
	if pm.localhost != newLocal {
		t.Error("Expected localhost to be updated")
	}
}
