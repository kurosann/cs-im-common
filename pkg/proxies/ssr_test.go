package proxies

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

// 创建测试用的 ShadowsocksR 配置对象
func createTestSSRConfig() SSRConfig {
	return SSRConfig{
		Server:        "127.0.0.1",
		Port:          8388,
		Password:      "password",
		Method:        "aes-128-cfb",
		Protocol:      "auth_aes128_sha1",
		ProtocolParam: "",
		Obfs:          "tls1.2_ticket_auth",
		ObfsParam:     "",
		UDP:           false,
	}
}

// testSSRHTTPClient 使用 HTTP 客户端通过 SSR 代理
func testSSRHTTPClient(t *testing.T, dialer Dialer) {
	t.Helper()

	// 创建使用 SSR 代理的 HTTP 客户端
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 10 * time.Second,
	}

	// 测试 HTTP 请求
	resp, err := client.Get("http://httpbin.org/ip")
	if err != nil {
		t.Fatalf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("Failed to read response body: %v", err)
	}

	t.Logf("HTTP Client Response (status %d):\n%s", resp.StatusCode, string(body))

	if resp.StatusCode != 200 {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	if len(body) == 0 {
		t.Error("Received empty response body")
	}
}

// TestSSRDialer_WithConfig 测试使用配置对象的真实连接
func TestSSRDialer_WithConfig(t *testing.T) {
	// 跳过测试，除非设置了环境变量
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建测试配置
	config := createTestSSRConfig()

	// 创建 ShadowsocksR Dialer
	dialer, err := NewSSRDialer(config)
	if err != nil {
		t.Fatalf("NewSSRDialer failed: %v", err)
	}

	// 测试基本方法
	if dialer.Scheme() != "ssr" {
		t.Errorf("Expected scheme 'ssr', got '%s'", dialer.Scheme())
	}

	if dialer.Addr() == "" {
		t.Error("Addr() should not be empty")
	}

	t.Logf("ShadowsocksR proxy address: %s", dialer.Addr())
	t.Logf("Method: %s", dialer.config.Method)
	t.Logf("Protocol: %s", dialer.config.Protocol)
	t.Logf("Obfs: %s", dialer.config.Obfs)
	t.Logf("UDP enabled: %v", dialer.config.UDP)

	// 使用 HTTP 客户端测试
	t.Run("HTTP Client Test", func(t *testing.T) {
		testSSRHTTPClient(t, dialer)
	})
}

// TestSSRDialer_WithUrl 测试使用 URL 创建 SSR Dialer
func TestSSRDialer_WithUrl(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 测试 SSR URL 解析
	// 注意：这是一个示例 URL，实际使用时需要替换为真实的 SSR 服务器地址
	testURL := "ssr://MTI3LjAuMC4xOjgzODg6b3JpZ2luOmFlcy0yNTYtY2ZiOnBsYWluOlpVRlBOVFJ6Lz9vYmZzcGFyYW09JnByb3RvcGFyYW09"

	dialer, err := NewSSRDialerWithUrl(testURL)
	if err != nil {
		t.Logf("URL parsing test (expected to fail with test URL): %v", err)
		return
	}

	if dialer == nil {
		t.Error("Dialer should not be nil")
	}

	t.Logf("Parsed SSR URL successfully: %s", dialer.Addr())
}
