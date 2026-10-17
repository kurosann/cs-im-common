package proxies

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestNewProxyChain(t *testing.T) {
	localhost, err := NewProxyDialerWithUrl("socks5://127.0.0.1:7890")
	if err != nil {
		t.Fatalf("NewProxyDialerWithUrl failed: %v", err)
	}
	remote, err := NewProxyDialerWithUrl("trojan://87f07f1e-87b1-4d67-9120-8072a6b52e23@jp1.c879401d-2ac8-a78b-2046-a8445d6f163a.7adf211e.the-best-airport.com:443?allowInsecure=1&peer=new.download.the-best-airport.com&sni=new.download.the-best-airport.com&type=tcp")
	if err != nil {
		t.Fatalf("NewProxyDialerWithUrl failed: %v", err)
	}
	proxyChain := NewProxyChain([]Dialer{localhost, remote})
	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return proxyChain.Dial(network, addr)
			},
		},
	}
	start := time.Now()
	resp, err := client.Get("https://www.google.com")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}
	fmt.Println(string(body))
	fmt.Println(time.Since(start))
}
