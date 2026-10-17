package proxies

import (
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
)

func createTestAnyTLSConfig() AnyTLSConfig {
	return AnyTLSConfig{
		Server:         "",
		Port:           443,
		Password:       "",
		Network:        "tcp",
		TLS:            true,
		SkipCertVerify: true,
		SNI:            "",
	}
}

func testAnyTLSHTTPGet(t *testing.T, dialer Dialer, host, path string) string {
	t.Helper()

	conn, err := dialer.Dial("tcp", net.JoinHostPort(host, "80"))
	if err != nil {
		t.Fatalf("Failed to dial %s: %v", host, err)
	}
	defer conn.Close()

	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", path, host)
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatalf("Failed to write request: %v", err)
	}

	buf := make([]byte, 4096)
	n, err := conn.Read(buf)
	if err != nil && err != io.EOF {
		t.Fatalf("Failed to read response: %v", err)
	}

	resp := string(buf[:n])
	if idx := strings.Index(resp, "\r\n\r\n"); idx != -1 {
		resp = strings.TrimSpace(resp[idx+4:])
	}
	t.Logf("%s%s: %s", host, path, resp)
	return resp
}

func TestAnyTLSDialer_WithConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	config := createTestAnyTLSConfig()

	dialer, err := NewAnyTLSDialer(config)
	if err != nil {
		t.Fatalf("NewAnyTLSDialer failed: %v", err)
	}
	defer dialer.Close()

	if dialer.Scheme() != "anytls" {
		t.Errorf("Expected scheme 'anytls', got '%s'", dialer.Scheme())
	}
	if dialer.Addr() == "" {
		t.Error("Addr() should not be empty")
	}
	t.Logf("AnyTLS proxy address: %s", dialer.Addr())

	t.Run("Connectivity", func(t *testing.T) {
		body := testAnyTLSHTTPGet(t, dialer, "cp.cloudflare.com", "/generate_204")
		if body == "" {
			t.Error("Expected non-empty response from connectivity check")
		}
	})

	t.Run("Query IP", func(t *testing.T) {
		body := testAnyTLSHTTPGet(t, dialer, "httpbin.org", "/ip")
		if body == "" {
			t.Error("Expected non-empty response from IP query")
		}
	})

	t.Run("Query Location", func(t *testing.T) {
		body := testAnyTLSHTTPGet(t, dialer, "ip-api.com", "/line/?fields=country,regionName,city,isp,query")
		if body == "" {
			t.Error("Expected non-empty response from location query")
		}
	})
}

func TestAnyTLSDialer_WithUrl(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	proxyAddr := "anytls://@:443?sni=&insecure=true"

	dialer, err := NewAnyTLSDialerWithUrl(proxyAddr)
	if err != nil {
		t.Fatalf("NewAnyTLSDialerWithUrl failed: %v", err)
	}
	defer dialer.Close()

	if dialer.Scheme() != "anytls" {
		t.Errorf("Expected scheme 'anytls', got '%s'", dialer.Scheme())
	}
	t.Logf("AnyTLS proxy address (from URL): %s", dialer.Addr())

	t.Run("Connectivity", func(t *testing.T) {
		body := testAnyTLSHTTPGet(t, dialer, "cp.cloudflare.com", "/generate_204")
		if body == "" {
			t.Error("Expected non-empty response from connectivity check")
		}
	})

	t.Run("Query IP", func(t *testing.T) {
		body := testAnyTLSHTTPGet(t, dialer, "httpbin.org", "/ip")
		if body == "" {
			t.Error("Expected non-empty response from IP query")
		}
	})
}

func TestAnyTLSDialer_UrlParsing(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{
			name:    "full url",
			url:     "anytls://password@example.com:8443?sni=cdn.example.com&insecure=true&alpn=h2,http/1.1",
			wantErr: false,
		},
		{
			name:    "minimal url",
			url:     "anytls://password@example.com:443",
			wantErr: false,
		},
		{
			name:    "with ws path",
			url:     "anytls://password@example.com:443?path=/ws",
			wantErr: false,
		},
		{
			name:    "missing password",
			url:     "anytls://example.com:443",
			wantErr: true,
		},
		{
			name:    "wrong scheme",
			url:     "https://password@example.com:443",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dialer, err := NewAnyTLSDialerWithUrl(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}
			dialer.Close()
		})
	}
}
