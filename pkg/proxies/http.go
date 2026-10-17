package proxies

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strings"
)

var _ Dialer = (*HttpDialer)(nil)

// HttpDialerConfig HTTP/HTTPS 代理配置
type HttpDialerConfig struct {
	// 地址信息
	Scheme string // http 或 https
	Host   string
	Port   string

	// 认证信息
	Username string
	Password string
	TLS      bool

	// TLS 配置
	// TLS: 是否使用 TLS 连接（如果为 true 且 Scheme 为 https，会将 Scheme 变更为 http）
	// TLSInsecureSkipVerify: 是否跳过 TLS 证书验证
	TLSInsecureSkipVerify bool
}

type HttpDialer struct {
	config HttpDialerConfig

	addr string
	url  *url.URL

	dialer Dialer
}

func (h *HttpDialer) ProxyHost() string {
	return h.config.Host
}

func (h *HttpDialer) Scheme() string {
	return "http"
}

// NewHttpDialer 使用配置对象创建 HTTP/HTTPS 代理客户端
func NewHttpDialer(config HttpDialerConfig) (*HttpDialer, error) {
	if config.Scheme == "" {
		config.Scheme = "http"
	}
	switch strings.ToLower(config.Scheme) {
	case "http":
		if config.Port == "" {
			config.Port = "80"
		}
	case "https":
		if config.Port == "" {
			config.Port = "443"
		}
		config.Scheme = "http"
		config.TLS = true
	default:
		return nil, fmt.Errorf("unsupported http proxy scheme: %s", config.Scheme)
	}

	if config.Host == "" {
		return nil, fmt.Errorf("http proxy host is required")
	}

	addr := fmt.Sprintf("%s://%s:%s", config.Scheme, config.Host, config.Port)
	u, err := url.Parse(addr)
	if err != nil {
		return nil, fmt.Errorf("parse http proxy addr error: %w", err)
	}

	if config.Username != "" {
		if config.Password == "" {
			config.Password = config.Username
		}
		u.User = url.UserPassword(config.Username, config.Password)
	}

	return &HttpDialer{
		config: config,
		addr:   addr,
		url:    u,
	}, nil
}

// NewHttpDialerWithUrl 使用 URL 创建 HTTP/HTTPS 代理客户端
// 支持的 URL 示例:
//
//	http://user:pass@proxy.example.com:8080
//	https://user:pass@proxy.example.com:8443
func NewHttpDialerWithUrl(proxyAddr string) (*HttpDialer, error) {
	u, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, err
	}

	scheme := u.Scheme
	if scheme == "" {
		scheme = "http"
	}

	host := u.Hostname()
	port := u.Port()
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	var username, password string
	if u.User != nil {
		username = u.User.Username()
		password, _ = u.User.Password()
	}
	t := false
	tlsStr := u.Query().Get("tls")
	if tlsStr == "true" {
		t = true
	}

	config := HttpDialerConfig{
		Scheme: scheme,
		Host:   host,
		Port:   port,

		Username: username,
		Password: password,
		TLS:      t,
	}

	return NewHttpDialer(config)
}

func (h *HttpDialer) SetDialer(dialer Dialer) {
	h.dialer = dialer
}

func (h *HttpDialer) Addr() string {
	return h.addr
}

func (h *HttpDialer) Dial(network, targetAddr string) (net.Conn, error) {
	return h.DialContext(context.Background(), network, targetAddr)
}

// Dial 通过 HTTP/HTTPS 代理连接目标地址
func (h *HttpDialer) DialContext(ctx context.Context, network, targetAddr string) (net.Conn, error) {
	if network == "" {
		network = "tcp"
	}

	// 使用全局 DNS 解析代理服务器地址
	proxyHostPort := GetGlobalDNSResolver().ResolveAddr(ctx, net.JoinHostPort(h.config.Host, h.config.Port))

	var (
		conn net.Conn
		err  error
	)

	if h.dialer != nil {
		conn, err = h.dialer.DialContext(ctx, "tcp", proxyHostPort)
		if err != nil {
			return nil, fmt.Errorf("dial http proxy through upstream error: %w", err)
		}
	} else {
		d := net.Dialer{}
		conn, err = d.DialContext(ctx, "tcp", proxyHostPort)
		if err != nil {
			return nil, fmt.Errorf("dial http proxy error: %w", err)
		}
	}

	release := bindConnToDialContext(ctx, conn)
	defer release()

	// 根据 TLS 参数决定是否使用 TLS 连接
	if h.config.TLS {
		tlsConfig := &tls.Config{
			ServerName:         h.config.Host,
			InsecureSkipVerify: h.config.TLSInsecureSkipVerify,
		}
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.Handshake(); err != nil {
			conn.Close()
			return nil, fmt.Errorf("tls handshake with https proxy error: %w", err)
		}
		conn = tlsConn
	}

	_, _, err = net.SplitHostPort(targetAddr)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("invalid target address: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "CONNECT %s HTTP/1.1\r\n", targetAddr)
	fmt.Fprintf(&b, "Host: %s\r\n", targetAddr)
	fmt.Fprintf(&b, "Proxy-Connection: keep-alive\r\n")

	if h.config.Username != "" {
		cred := h.config.Username + ":" + h.config.Password
		auth := base64.StdEncoding.EncodeToString([]byte(cred))
		fmt.Fprintf(&b, "Proxy-Authorization: Basic %s\r\n", auth)
	}

	b.WriteString("\r\n")

	if _, err := conn.Write([]byte(b.String())); err != nil {
		conn.Close()
		return nil, fmt.Errorf("write http CONNECT request error: %w", err)
	}

	br := bufio.NewReader(conn)

	statusLine, err := br.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("read http CONNECT response error: %w", err)
	}

	if !strings.Contains(statusLine, "200") {
		conn.Close()
		return nil, fmt.Errorf("http CONNECT failed: %s", strings.TrimSpace(statusLine))
	}

	for {
		line, err := br.ReadString('\n')
		if err != nil {
			conn.Close()
			return nil, fmt.Errorf("read http CONNECT headers error: %w", err)
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			break
		}
	}

	// 根据 TLS 参数决定连接类型
	typ := "http"
	if h.config.TLS {
		typ = "https"
	}
	if conn == nil {
		return nil, fmt.Errorf("http proxy conn is nil")
	}

	return NewProxyConn(conn, typ), nil
}
