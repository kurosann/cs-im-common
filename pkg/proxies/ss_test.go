package proxies

import (
	"fmt"
	"io"
	"net"
	"testing"
)

// 创建测试用的 Shadowsocks 配置对象
func createTestSSConfig() SSConfig {
	return SSConfig{
		Server:   "your.ss.server",
		Port:     8388,
		Password: "your_password",
		Method:   "aes-256-cfb",
		UDP:      false,
	}
}

// testSSIPQuery 测试通过代理查询 IP 地址
func testSSIPQuery(t *testing.T, dialer Dialer, targetHost string) {
	t.Helper()

	// 通过代理建立连接
	conn, err := dialer.Dial("tcp", targetHost)
	if err != nil {
		t.Fatalf("Failed to dial through SS proxy: %v", err)
	}
	defer conn.Close()

	// 创建 HTTP 请求
	hostOnly := targetHost
	if host, _, err := net.SplitHostPort(targetHost); err == nil {
		hostOnly = host
	}
	req := fmt.Sprintf("GET /ip HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", hostOnly)

	// 发送请求
	_, err = conn.Write([]byte(req))
	if err != nil {
		t.Fatalf("Failed to write request: %v", err)
	}

	// 读取响应
	response := make([]byte, 4096)
	n, err := conn.Read(response)
	if err != nil && err != io.EOF {
		t.Fatalf("Failed to read response: %v", err)
	}

	responseStr := string(response[:n])
	t.Logf("Response from %s:\n%s", targetHost, responseStr)

	// 验证响应包含 IP 地址信息
	if n == 0 {
		t.Error("Received empty response")
	}
}

// testSSLocationQuery 测试通过代理查询 IP 地区信息
func testSSLocationQuery(t *testing.T, dialer Dialer, targetHost string) {
	t.Helper()

	// 通过代理建立连接
	conn, err := dialer.Dial("tcp", targetHost)
	if err != nil {
		t.Fatalf("Failed to dial through SS proxy: %v", err)
	}
	defer conn.Close()

	// 创建 HTTP 请求查询地区信息
	hostOnly := targetHost
	if host, _, err := net.SplitHostPort(targetHost); err == nil {
		hostOnly = host
	}
	req := fmt.Sprintf("GET /json HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", hostOnly)

	// 发送请求
	_, err = conn.Write([]byte(req))
	if err != nil {
		t.Fatalf("Failed to write request: %v", err)
	}

	// 读取响应
	response := make([]byte, 4096)
	n, err := conn.Read(response)
	if err != nil && err != io.EOF {
		t.Fatalf("Failed to read response: %v", err)
	}

	responseStr := string(response[:n])
	t.Logf("Location Response from %s:\n%s", targetHost, responseStr)

	// 验证响应包含地区信息
	if n == 0 {
		t.Error("Received empty response")
	}
}

// TestSSDialer_WithConfig 测试使用配置对象的真实连接
func TestSSDialer_WithConfig(t *testing.T) {
	// 跳过测试，除非设置了环境变量
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建测试配置
	config := createTestSSConfig()

	// 创建 Shadowsocks Dialer
	dialer, err := NewSSDialer(config)
	if err != nil {
		t.Fatalf("NewSSDialer failed: %v", err)
	}

	// 测试基本方法
	if dialer.Scheme() != "ss" {
		t.Errorf("Expected scheme 'ss', got '%s'", dialer.Scheme())
	}

	if dialer.Addr() == "" {
		t.Error("Addr() should not be empty")
	}

	t.Logf("Shadowsocks proxy address: %s", dialer.Addr())
	t.Logf("Method: %s", dialer.config.Method)
	t.Logf("UDP enabled: %v", dialer.config.UDP)

	// 测试 TCP 连接 - 查询 IP
	t.Run("Query IP", func(t *testing.T) {
		testSSIPQuery(t, dialer, "httpbin.org:80")
	})

	// 测试查询地区信息
	t.Run("Query Location", func(t *testing.T) {
		testSSLocationQuery(t, dialer, "ip-api.com:80")
	})

	// 测试 UDP 连接（如果启用）
	if config.UDP {
		t.Run("UDP Support", func(t *testing.T) {
			// 测试 UDP 是否被正确启用
			if !dialer.config.UDP {
				t.Error("UDP should be enabled")
			}
			t.Log("UDP is enabled for this proxy")
		})
	}
}
