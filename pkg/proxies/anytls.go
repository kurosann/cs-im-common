package proxies

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/kurosann/anytls-go"
	"golang.org/x/net/websocket"
)

var _ Dialer = (*AnyTLSDialer)(nil)

type AnyTLSConfig struct {
	Server         string `json:"server" yaml:"server"`
	Port           int    `json:"port" yaml:"port"`
	Password       string `json:"password" yaml:"password"`
	Network        string `json:"network" yaml:"network"`
	TLS            bool   `json:"tls" yaml:"tls"`
	SkipCertVerify bool   `json:"skip-cert-verify" yaml:"skip-cert-verify"`
	SNI            string `json:"sni" yaml:"sni"`
	ALPN           []string
	WSPath         string            `json:"ws-path" yaml:"ws-path"`
	WSHeaders      map[string]string `json:"ws-headers" yaml:"ws-headers"`
}

type AnyTLSDialer struct {
	addr      string
	config    AnyTLSConfig
	dialer    *anytls_go.AnytlsDialer
	upstream  Dialer
	transport *transportDialer
}

func NewAnyTLSDialer(config AnyTLSConfig) (*AnyTLSDialer, error) {
	if config.Server == "" {
		return nil, fmt.Errorf("anytls server is required")
	}
	if config.Password == "" {
		return nil, fmt.Errorf("anytls password is required")
	}
	if config.Port <= 0 || config.Port > 65535 {
		return nil, fmt.Errorf("invalid port: %d", config.Port)
	}
	if config.Network == "" {
		config.Network = "tcp"
	}
	if config.SNI == "" {
		config.SNI = config.Server
	}

	serverAddr := net.JoinHostPort(config.Server, fmt.Sprintf("%d", config.Port))
	addr := fmt.Sprintf("anytls://%s@%s:%d", config.Password, config.Server, config.Port)

	td := &transportDialer{config: config}

	a := &AnyTLSDialer{
		addr:      addr,
		config:    config,
		transport: td,
	}

	a.dialer = anytls_go.NewAnytlsDialer(context.Background(), &anytls_go.AnytlsConfig{
		ServerAddr:         serverAddr,
		Password:           config.Password,
		SNI:                config.SNI,
		InsecureSkipVerify: config.SkipCertVerify,
		ALPN:               config.ALPN,
		Dialer:             td,
		DialTimeout:        5 * time.Second,
		IdleTimeout:        30 * time.Second,
	})
	return a, nil
}

func NewAnyTLSDialerWithUrl(proxyAddr string) (*AnyTLSDialer, error) {
	u, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("parse anytls url error: %w", err)
	}
	if u.Scheme != "anytls" {
		return nil, fmt.Errorf("invalid anytls url: scheme must be anytls")
	}

	host := u.Host
	if host == "" {
		return nil, fmt.Errorf("invalid anytls url: missing host")
	}

	password := ""
	if u.User != nil {
		password = u.User.String()
	}
	if password == "" {
		return nil, fmt.Errorf("anytls password is required")
	}

	h, portStr, err := net.SplitHostPort(host)
	if err != nil {
		h = host
		portStr = "443"
	}
	port, _ := strconv.Atoi(portStr)

	query := u.Query()

	sni := query.Get("sni")
	if sni == "" {
		sni = h
	}

	insecure, _ := strconv.ParseBool(query.Get("insecure"))

	var alpn []string
	if a := query.Get("alpn"); a != "" {
		alpn = strings.Split(a, ",")
	}

	config := AnyTLSConfig{
		Server:         h,
		Port:           port,
		Password:       password,
		Network:        "tcp",
		TLS:            true,
		SkipCertVerify: insecure,
		SNI:            sni,
		ALPN:           alpn,
	}

	if wsPath := query.Get("path"); wsPath != "" {
		config.Network = "ws"
		config.WSPath = wsPath
	}

	return NewAnyTLSDialer(config)
}

func (a *AnyTLSDialer) Addr() string {
	return a.addr
}

func (a *AnyTLSDialer) SetDialer(dialer Dialer) {
	a.upstream = dialer
	a.transport.upstream = dialer
}

func (a *AnyTLSDialer) Dial(network, targetAddr string) (net.Conn, error) {
	return a.DialContext(context.Background(), network, targetAddr)
}

func (a *AnyTLSDialer) DialContext(ctx context.Context, network, targetAddr string) (net.Conn, error) {
	conn, err := a.dialer.DialContext(ctx, network, targetAddr)
	if err != nil {
		return nil, fmt.Errorf("create anytls stream error: %w", err)
	}
	return NewProxyConn(conn, "anytls"), nil
}

func (a *AnyTLSDialer) Close() error {
	return a.dialer.Close()
}

func (a *AnyTLSDialer) ProxyHost() string {
	return a.config.Server
}

func (a *AnyTLSDialer) Scheme() string {
	return "anytls"
}

type transportDialer struct {
	config   AnyTLSConfig
	upstream Dialer
}

func (d *transportDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	resolvedAddr := GetGlobalDNSResolver().ResolveAddr(ctx, address)

	var conn net.Conn
	var err error

	if d.upstream != nil {
		conn, err = d.upstream.DialContext(ctx, "tcp", resolvedAddr)
		if err != nil {
			return nil, fmt.Errorf("dial anytls server through proxy error: %w", err)
		}
	} else {
		dialer := &net.Dialer{Timeout: 5 * time.Second}
		conn, err = dialer.DialContext(ctx, "tcp", resolvedAddr)
		if err != nil {
			return nil, fmt.Errorf("dial anytls server error: %w", err)
		}
	}

	if d.config.TLS {
		tlsConfig := &tls.Config{
			ServerName:         d.config.SNI,
			InsecureSkipVerify: d.config.SkipCertVerify,
		}
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			return nil, fmt.Errorf("tls handshake error: %w", err)
		}
		conn = tlsConn
	}

	if d.config.Network == "ws" || d.config.Network == "websocket" {
		wsConn, err := upgradeToWebSocketForAnyTLS(conn, d.config)
		if err != nil {
			conn.Close()
			return nil, err
		}
		conn = wsConn
	}

	return conn, nil
}

func upgradeToWebSocketForAnyTLS(conn net.Conn, config AnyTLSConfig) (net.Conn, error) {
	wsPath := config.WSPath
	if wsPath == "" {
		wsPath = "/"
	}
	if !strings.HasPrefix(wsPath, "/") {
		wsPath = "/" + wsPath
	}

	wsHeaders := make(map[string]string)
	if config.WSHeaders != nil {
		for k, v := range config.WSHeaders {
			wsHeaders[k] = v
		}
	}
	if _, ok := wsHeaders["User-Agent"]; !ok {
		wsHeaders["User-Agent"] = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	}

	scheme := "ws"
	if config.TLS {
		scheme = "wss"
	}
	wsURL := fmt.Sprintf("%s://%s:%d%s", scheme, config.Server, config.Port, wsPath)

	wsConfig, err := websocket.NewConfig(wsURL, "http://localhost")
	if err != nil {
		return nil, fmt.Errorf("create websocket config error: %w", err)
	}
	for k, v := range wsHeaders {
		wsConfig.Header.Set(k, v)
	}
	wsConfig.Header.Set("Host", fmt.Sprintf("%s:%d", config.Server, config.Port))

	wsConn, err := websocket.NewClient(wsConfig, conn)
	if err != nil {
		return nil, fmt.Errorf("websocket handshake error: %w", err)
	}
	return wsConn, nil
}
