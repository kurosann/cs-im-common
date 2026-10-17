package ws

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/net/websocket"
)

// 创建测试用的 WebSocket 服务器
func setupTestServer() *httptest.Server {
	handler := websocket.Handler(func(ws *websocket.Conn) {
		var msg string
		for {
			// 接收消息
			err := websocket.Message.Receive(ws, &msg)
			if err != nil {
				return
			}
			// 回显消息
			err = websocket.Message.Send(ws, "echo: "+msg)
			if err != nil {
				return
			}
		}
	})
	return httptest.NewServer(handler)
}

func TestWebSocketConnection(t *testing.T) {
	// 启动测试服务器
	server := setupTestServer()
	defer server.Close()

	// 将 http URL 转换为 ws URL
	wsURL := "ws://" + server.Listener.Addr().String()

	// 测试用例
	tests := []struct {
		name    string
		header  http.Header
		message string
	}{
		{
			name:    "基本连接测试",
			header:  http.Header{"User-Agent": []string{"test"}},
			message: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建连接
			dialer := &Dialer{Header: tt.header}
			conn, err := dialer.Dial(context.Background(), wsURL)
			if err != nil {
				t.Fatalf("连接失败: %v", err)
			}
			defer conn.Close()

			// 测试发送消息
			err = conn.Send(tt.message)
			if err != nil {
				t.Fatalf("发送消息失败: %v", err)
			}

			// 测试接收消息
			response, err := conn.Read()
			if err != nil {
				t.Fatalf("接收消息失败: %v", err)
			}

			expectedResponse := "echo: " + tt.message
			if response != expectedResponse {
				t.Errorf("期望收到 %q, 实际收到 %q", expectedResponse, response)
			}

			// 测试连接状态
			if !conn.IsAlive() {
				t.Error("连接应该处于活跃状态")
			}

			// 测试超时设置
			err = conn.SetReadDeadline(time.Now().Add(time.Second))
			if err != nil {
				t.Errorf("设置读取超时失败: %v", err)
			}

			err = conn.SetWriteDeadline(time.Now().Add(time.Second))
			if err != nil {
				t.Errorf("设置写入超时失败: %v", err)
			}
		})
	}
}

func TestConnectionError(t *testing.T) {
	// 测试无效的 URL
	dialer := &Dialer{}
	_, err := dialer.Dial(context.Background(), "ws://invalid-url")
	if err == nil {
		t.Error("期望连接失败，但是成功了")
	}
}
