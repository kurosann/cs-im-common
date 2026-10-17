package ws

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MockHandler 模拟消息处理器
type MockHandler struct {
	receivedMsgs []string
	errors       []error
}

func (m *MockHandler) OnReceive(msg []byte) error {
	m.receivedMsgs = append(m.receivedMsgs, string(msg))
	return nil
}

func (m *MockHandler) OnError(err error) {
	m.errors = append(m.errors, err)
}

func (m *MockHandler) OnReconnect() [][]byte {
	return [][]byte{[]byte("reconnect")}
}

func TestKeepAliver_Basic(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	mockDialer := Dialer{}
	mockHandler := &MockHandler{}

	ka := &KeepAliver{
		Address:    "ws://" + server.Listener.Addr().String(),
		Dialer:     mockDialer,
		MsgHandler: mockHandler,
		Delay:      time.Millisecond * 100,
	}

	connCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	// 测试连接建立
	ctx, err := ka.KeepAlive(connCtx)
	assert.NoError(t, err, "连接应该建立成功")
	assert.True(t, ka.IsAlive(), "连接应该处于活跃状态")

	// 测试发送消息
	err = ka.Send("test message")
	assert.NoError(t, err, "发送消息应该成功")

	// 测试重连
	ka.Reconnect()
	time.Sleep(time.Millisecond * 200) // 等待重连
	assert.True(t, ka.IsAlive(), "重连后连接应该处于活跃状态")

	// 测试关闭
	ka.Close()
	assert.False(t, ka.IsAlive(), "关闭后连接不应该处于活跃状态")

	// 确保上下文被取消
	select {
	case <-ctx.Done():
		// 正确
	default:
		t.Error("上下文应该被取消")
	}
}

func TestKeepAliver_SendError(t *testing.T) {
	server := setupTestServer()
	defer server.Close()

	mockDialer := Dialer{}
	mockHandler := &MockHandler{}

	ka := &KeepAliver{
		Address:    "ws://" + server.Listener.Addr().String(),
		Dialer:     mockDialer,
		MsgHandler: mockHandler,
		Delay:      time.Millisecond * 100,
	}

	connCtx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	ctx, err := ka.KeepAlive(connCtx)
	assert.NoError(t, err, "连接应该建立成功")

	ka.conn.Close() // 模拟网络连接断开,保持连接活跃
	assert.True(t, ka.IsAlive(), "连接应该处于活跃状态")
	ka.Close() // 模拟连接断开

	mockHandler.errors = []error{}
	err = ka.Send("test message")
	assert.Error(t, err, "向断开的连接发送消息应该返回错误")
	assert.Empty(t, mockHandler.errors, "错误应该为空")

	<-ctx.Done() // 等待上下文结束
}
