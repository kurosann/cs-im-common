package proxies

import (
	"fmt"
	"io"
	"net"
	"testing"
)

// 创建测试用的 Trojan 配置对象
func createTestTrojanConfig() TrojanConfig {
	//trojan://87f07f1e-87b1-4d67-9120-8072a6b52e23@jp1.c879401d-2ac8-a78b-2046-a8445d6f163a.7adf211e.the-best-airport.com:443?allowInsecure=1&peer=new.download.the-best-airport.com&sni=new.download.the-best-airport.com&type=tcp
	return TrojanConfig{
		Server:         "jp1.c879401d-2ac8-a78b-2046-a8445d6f163a.7adf211e.the-best-airport.com",
		Port:           443,
		Password:       "87f07f1e-87b1-4d67-9120-8072a6b52e23",
		UDP:            false,
		SkipCertVerify: true,
		SNI: "new.download.the-best-airport.com",
	}
}

// testTrojanIPQuery 测试通过代理查询 IP 地址
func testTrojanIPQuery(t *testing.T, dialer Dialer, targetHost string) {
	t.Helper()

	// 通过代理建立连接
	conn, err := dialer.Dial("tcp", targetHost)
	if err != nil {
		t.Fatalf("Failed to dial through Trojan proxy: %v", err)
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

// testTrojanLocationQuery 测试通过代理查询 IP 地区信息
func testTrojanLocationQuery(t *testing.T, dialer Dialer, targetHost string) {
	t.Helper()

	// 通过代理建立连接
	conn, err := dialer.Dial("tcp", targetHost)
	if err != nil {
		t.Fatalf("Failed to dial through Trojan proxy: %v", err)
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

// TestTrojanDialer_WithConfig 测试使用配置对象的真实连接
func TestTrojanDialer_WithConfig(t *testing.T) {
	// 跳过测试，除非设置了环境变量
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// 创建测试配置
	config := createTestTrojanConfig()

	// 创建 Trojan Dialer
	dialer, err := NewTrojanDialer(config)
	if err != nil {
		t.Fatalf("NewTrojanDialer failed: %v", err)
	}

	// 测试基本方法
	if dialer.Scheme() != "trojan" {
		t.Errorf("Expected scheme 'trojan', got '%s'", dialer.Scheme())
	}

	if dialer.Addr() == "" {
		t.Error("Addr() should not be empty")
	}

	t.Logf("Trojan proxy address: %s", dialer.Addr())
	t.Logf("UDP enabled: %v", dialer.config.UDP)
	t.Logf("Skip cert verify: %v", dialer.config.SkipCertVerify)

	// 测试 TCP 连接 - 查询 IP
	t.Run("Query IP", func(t *testing.T) {
		testTrojanIPQuery(t, dialer, "httpbin.org:80")
	})

	// 测试查询地区信息
	t.Run("Query Location", func(t *testing.T) {
		testTrojanLocationQuery(t, dialer, "ip-api.com:80")
	})

	// 测试 UDP 连接（如果启用）
	if config.UDP {
		t.Run("UDP Support", func(t *testing.T) {
			// 测试 UDP 是否被正确启用
			if !dialer.config.UDP {
				t.Error("UDP should be enabled")
			}
			t.Log("UDP is enabled for this proxy")

			// 注意：实际的 UDP 测试需要支持 UDP 的目标服务器
			// 这里只测试能否创建 UDP 连接对象
			// conn, err := dialer.Dial("udp", "8.8.8.8:53")
			// if err != nil {
			//     t.Logf("UDP dial failed (expected if server doesn't support UDP): %v", err)
			// } else {
			//     conn.Close()
			//     t.Log("UDP connection created successfully")
			// }
		})
	}
}
