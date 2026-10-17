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

// 创建测试用的 VLESS 配置对象
func createTestVLESSConfig() VlessConfig {
	return VlessConfig{
		Server:         "hk1.c879401d-2ac8-a78b-2046-a8445d6f163a.000d4ee1.the-best-airport.com",
		Port:           443,
		UUID:           "87f07f1e-87b1-4d67-9120-8072a6b52e23",
		Flow:           "xtls-rprx-vision",
		Cipher:         "auto",
		UDP:            true,
		Network:        "tcp",
		TLS:            true,
		SkipCertVerify: true,
		ServerName:     "new.download.the-best-airport.com",
	}
}

// testVLESSHTTPClient 使用 HTTP 客户端通过 VLESS 代理
func testVLESSHTTPClient(t *testing.T, dialer Dialer) {
	t.Helper()

	// 创建使用 VLESS 代理的 HTTP 客户端
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return dialer.Dial(network, addr)
			},
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 10 * time.Second, // 10秒超时
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

// TestVLESSDialer_WithConfig 测试使用配置对象的真实连接
func TestVLESSDialer_WithConfig(t *testing.T) {
	// 跳过测试，除非设置了环境变量
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建测试配置
	config := createTestVLESSConfig()

	// 创建 VLESS Dialer
	dialer, err := NewVlessDialer(config)
	if err != nil {
		t.Fatalf("NewVlessDialer failed: %v", err)
	}

	// 测试基本方法
	if dialer.Scheme() != "vless" {
		t.Errorf("Expected scheme 'vless', got '%s'", dialer.Scheme())
	}

	if dialer.Addr() == "" {
		t.Error("Addr() should not be empty")
	}

	t.Logf("VLESS proxy address: %s", dialer.Addr())
	t.Logf("UUID: %s", dialer.config.UUID)
	t.Logf("Encryption: %s", dialer.config.Encryption)
	t.Logf("Flow: %s", dialer.config.Flow)
	t.Logf("Network: %s", dialer.config.Network)
	t.Logf("UDP: %v", dialer.config.UDP)
	t.Logf("TLS: %v", dialer.config.TLS)
	t.Logf("SNI: %s", dialer.config.SNI)

	// 使用 HTTP 客户端测试
	t.Run("HTTP Client Test", func(t *testing.T) {
		testVLESSHTTPClient(t, dialer)
	})
}
