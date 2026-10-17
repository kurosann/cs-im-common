package proxies

import (
	"fmt"
	"io"
	"testing"
)

// 创建测试用的 VMess 配置对象
func createTestVMessConfig() VMessConfig {
	return VMessConfig{
		Server:         "101.36.108.164",
		Port:           7777,
		UUID:           "6a30d332-3a9d-4ecc-8bb4-2ed3fa64f8a6",
		AlterId:        0,
		Cipher:         "auto",
		Network:        "tcp",
		TLS:            false,
		SkipCertVerify: true,
	}
}

// TestVMessDialer_WithConfig 测试使用配置对象的真实连接
func TestVMessDialer_WithConfig(t *testing.T) {
	// 跳过测试，除非设置了环境变量
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建测试配置
	config := createTestVMessConfig()

	// 创建 VMess Dialer
	dialer, err := NewVMessDialer(config)
	if err != nil {
		t.Fatalf("NewVMessDialer failed: %v", err)
	}

	// 测试基本方法
	if dialer.Scheme() != "vmess" {
		t.Errorf("Expected scheme 'vmess', got '%s'", dialer.Scheme())
	}

	if dialer.Addr() == "" {
		t.Error("Addr() should not be empty")
	}

	t.Logf("VMess proxy address: %s", dialer.Addr())
	t.Logf("UUID: %s", dialer.config.UUID)
	t.Logf("Cipher: %s", dialer.config.Cipher)
	t.Logf("Network: %s", dialer.config.Network)
	t.Logf("TLS: %v", dialer.config.TLS)

	// 通过代理连接到 IP 查询服务
	// 使用 httpbin.org/ip 来查询 IP 地址
	testIPQuery(t, dialer, "httpbin.org:80")

	// 使用 ip-api.com 来查询 IP 和地区信息
	testIPAndLocation(t, dialer, "ip-api.com:80")
}

// testIPQuery 测试通过代理查询 IP 地址
func testIPQuery(t *testing.T, dialer Dialer, targetHost string) {
	t.Helper()

	// 通过代理建立连接
	conn, err := dialer.Dial("tcp", targetHost)
	if err != nil {
		t.Fatalf("Failed to dial through VMess proxy: %v", err)
	}
	defer conn.Close()

	// 创建 HTTP 请求
	req := fmt.Sprintf("GET /ip HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", targetHost)

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

// testIPAndLocation 测试通过代理查询 IP 和地区信息
func testIPAndLocation(t *testing.T, dialer Dialer, targetHost string) {
	t.Helper()

	// 通过代理建立连接
	conn, err := dialer.Dial("tcp", targetHost)
	if err != nil {
		t.Fatalf("Failed to dial through VMess proxy: %v", err)
	}
	defer conn.Close()

	// 创建 HTTP 请求查询 IP 和地区
	req := fmt.Sprintf("GET /json HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", targetHost)

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

	// 验证响应包含地区信息
	if n == 0 {
		t.Error("Received empty response")
	}
}
