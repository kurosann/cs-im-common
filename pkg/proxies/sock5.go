package proxies

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"

	"golang.org/x/net/proxy"
)

var _ Dialer = (*Socks5Dialer)(nil)

type Socks5DialerConfig struct {
	Network  string
	Host     string
	Port     string
	Username string
	Password string
}

type Socks5Dialer struct {
	config Socks5DialerConfig

	addr string

	dialer Dialer
}

func NewSocks5Dialer(config Socks5DialerConfig) (*Socks5Dialer, error) {
	if config.Network == "" {
		config.Network = "tcp"
	}
	switch config.Network {
	case "tcp":
	case "udp":
	default:
		return nil, fmt.Errorf("invalid network: %s", config.Network)
	}
	if config.Host == "" {
		return nil, fmt.Errorf("host is required")
	}
	if config.Port == "" {
		return nil, fmt.Errorf("port is required")
	}
	_, err := strconv.Atoi(config.Port)
	if err != nil {
		return nil, fmt.Errorf("invalid port: %s", config.Port)
	}
	addr := net.JoinHostPort(config.Host, config.Port)
	if config.Username == "" {
		config.Username = config.Host
	}
	if config.Password == "" {
		config.Password = config.Host
	}

	return &Socks5Dialer{
		config: config,
		addr:   addr,
	}, nil
}

func NewSocks5DialerWithUrl(proxyAddr string) (*Socks5Dialer, error) {
	u, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, err
	}
	password, _ := u.User.Password()
	return NewSocks5Dialer(Socks5DialerConfig{
		Network:  "tcp",
		Host:     u.Hostname(),
		Port:     u.Port(),
		Username: u.User.Username(),
		Password: password,
	})
}
func (s *Socks5Dialer) SetDialer(dialer Dialer) {
	s.dialer = dialer
}

func (s *Socks5Dialer) Addr() string {
	return s.addr
}

func (s *Socks5Dialer) Dial(network, targetAddr string) (net.Conn, error) {
	return s.DialContext(context.Background(), network, targetAddr)
}

// Dial 通过SOCKS代理连接目标地址
func (s *Socks5Dialer) DialContext(ctx context.Context, network, targetAddr string) (net.Conn, error) {
	var upstreamDialer proxy.Dialer = proxy.Direct
	if s.dialer != nil {
		upstreamDialer = s.dialer
	}
	
	// 使用全局 DNS 解析代理服务器地址
	resolvedAddr := GetGlobalDNSResolver().ResolveAddr(ctx, s.addr)
	
	// 连接到 SOCKS5 代理服务器时总是使用 tcp 网络
	dialer, err := proxy.SOCKS5("tcp", resolvedAddr, &proxy.Auth{User: s.config.Username, Password: s.config.Password}, upstreamDialer)
	if err != nil {
		return nil, err
	}
	var conn net.Conn
	if d, ok := dialer.(proxy.ContextDialer); ok {
		conn, err = d.DialContext(ctx, network, targetAddr)
	} else {
		conn, err = dialer.Dial(network, targetAddr)
		release := bindConnToDialContext(ctx, conn)
		defer release()
	}
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, fmt.Errorf("socks5 dial conn is nil")
	}
	return NewProxyConn(conn, s.Scheme()), nil
}

func (s *Socks5Dialer) ProxyHost() string {
	return s.config.Host
}

func (s *Socks5Dialer) Scheme() string {
	return "socks5"
}
