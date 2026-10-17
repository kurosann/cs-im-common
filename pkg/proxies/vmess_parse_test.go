package proxies

import (
	"encoding/base64"
	"encoding/json"
	"testing"
)

func TestNewVMessDialerWithUrl_AllowsPayloadWithSlash(t *testing.T) {
	proxyAddr := "vmess://ew0KICAidiI6ICIyIiwNCiAgInBzIjogIlx1RDgzQ1x1RERFQ1x1RDgzQ1x1RERFN1/oi7Hlm70iLA0KICAiYWRkIjogIjgyLjE5OC4yNDYuOTciLA0KICAicG9ydCI6ICIxODAiLA0KICAiaWQiOiAiZDEzZmMyZjUtM2UwNS00Nzk1LTgxZWItNDQxNDNhMDllNTUyIiwNCiAgImFpZCI6ICIwIiwNCiAgInNjeSI6ICJhdXRvIiwNCiAgIm5ldCI6ICJ0Y3AiLA0KICAidHlwZSI6ICJub25lIiwNCiAgImhvc3QiOiAiIiwNCiAgInBhdGgiOiAiIiwNCiAgInRscyI6ICIiLA0KICAic25pIjogIiIsDQogICJhbHBuIjogIiIsDQogICJmcCI6ICIiLA0KICAiaW5zZWN1cmUiOiAiMCINCn0="

	dialer, err := NewVMessDialerWithUrl(proxyAddr)
	if err != nil {
		t.Fatalf("NewVMessDialerWithUrl failed: %v", err)
	}

	if dialer.config.Server != "82.198.246.97" {
		t.Fatalf("unexpected server: %s", dialer.config.Server)
	}
	if dialer.config.Port != 180 {
		t.Fatalf("unexpected port: %d", dialer.config.Port)
	}
	if dialer.config.UUID != "d13fc2f5-3e05-4795-81eb-44143a09e552" {
		t.Fatalf("unexpected uuid: %s", dialer.config.UUID)
	}
	if dialer.config.Network != "tcp" {
		t.Fatalf("unexpected network: %s", dialer.config.Network)
	}
	if dialer.config.TLS {
		t.Fatalf("expected tls disabled")
	}
	if dialer.config.SkipCertVerify {
		t.Fatalf("expected skip-cert-verify disabled")
	}
	if len(dialer.config.WSHeaders) != 0 {
		t.Fatalf("expected no ws headers, got: %#v", dialer.config.WSHeaders)
	}
}

func TestNewVMessDialerWithUrl_SupportsRawURLBase64AndWSHost(t *testing.T) {
	payload := map[string]any{
		"v":        "2",
		"ps":       "ws-test",
		"add":      "ws.example.com",
		"port":     "443",
		"id":       "d13fc2f5-3e05-4795-81eb-44143a09e552",
		"aid":      "0",
		"scy":      "auto",
		"net":      "ws",
		"type":     "none",
		"host":     "cdn.example.com",
		"path":     "/socket",
		"tls":      "tls",
		"sni":      "edge.example.com",
		"insecure": "1",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	proxyAddr := "vmess://" + base64.RawURLEncoding.EncodeToString(jsonData)
	dialer, err := NewVMessDialerWithUrl(proxyAddr)
	if err != nil {
		t.Fatalf("NewVMessDialerWithUrl failed: %v", err)
	}

	if dialer.config.Network != "ws" {
		t.Fatalf("unexpected network: %s", dialer.config.Network)
	}
	if !dialer.config.TLS {
		t.Fatalf("expected tls enabled")
	}
	if !dialer.config.SkipCertVerify {
		t.Fatalf("expected skip-cert-verify enabled")
	}
	if dialer.config.SNI != "edge.example.com" {
		t.Fatalf("unexpected sni: %s", dialer.config.SNI)
	}
	if dialer.config.WSPath != "/socket" {
		t.Fatalf("unexpected ws path: %s", dialer.config.WSPath)
	}
	if dialer.config.WSHeaders["Host"] != "cdn.example.com" {
		t.Fatalf("unexpected ws host header: %#v", dialer.config.WSHeaders)
	}
}
