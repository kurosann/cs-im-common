package proxies

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	shadowsocks2 "github.com/sagernet/sing-shadowsocks2"
	"github.com/sagernet/sing-shadowsocks2/shadowaead"
	"github.com/sagernet/sing-shadowsocks2/shadowaead_2022"
	"github.com/sagernet/sing-shadowsocks2/shadowstream"
	M "github.com/sagernet/sing/common/metadata"
)

var _ Dialer = (*SSDialer)(nil)

// SSConfig Shadowsocks 配置结构
type SSConfig struct {
	Server   string `json:"server" yaml:"server"`
	Port     int    `json:"port" yaml:"port"`
	Password string `json:"password" yaml:"password"`
	Method   string `json:"method" yaml:"method"` // 加密方法: aes-128-gcm, aes-256-gcm, chacha20-poly1305 等
	UDP      bool   `json:"udp" yaml:"udp"`
}

type SSDialer struct {
	addr   string
	config SSConfig
	dialer Dialer
}

func (s *SSDialer) Addr() string {
	return s.addr
}

func (s *SSDialer) SetDialer(dialer Dialer) {
	s.dialer = dialer
}

func (s *SSDialer) ProxyHost() string {
	return s.config.Server
}

func (s *SSDialer) Scheme() string {
	return "ss"
}

// NewSSDialer 使用配置对象创建 Shadowsocks 代理客户端
func NewSSDialer(config SSConfig) (*SSDialer, error) {
	// 验证必要参数
	if config.Server == "" {
		return nil, fmt.Errorf("shadowsocks server is required")
	}
	if config.Password == "" {
		return nil, fmt.Errorf("shadowsocks password is required")
	}
	if config.Port <= 0 || config.Port > 65535 {
		return nil, fmt.Errorf("invalid port: %d", config.Port)
	}
	if config.Method == "" {
		config.Method = "aes-256-gcm" // 默认加密方法
	}

	// 构建地址字符串用于显示
	addr := fmt.Sprintf("ss://%s@%s:%d", config.Method, config.Server, config.Port)

	return &SSDialer{
		addr:   addr,
		config: config,
		dialer: nil,
	}, nil
}

// NewSSDialerWithUrl 使用 URL 创建 Shadowsocks 代理客户端
// URL 格式: ss://base64(method:password)@server:port
func NewSSDialerWithUrl(proxyAddr string) (*SSDialer, error) {
	u, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("parse ss url error: %w", err)
	}

	// 解析服务器和端口
	host := u.Hostname()
	portStr := u.Port()
	if host == "" {
		return nil, fmt.Errorf("ss server is required")
	}

	port := 8388 // 默认端口
	if portStr != "" {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
	}

	// 解析 userinfo (method:password)，可能是 base64 编码的
	var method, password string
	if u.User != nil {
		userInfo := u.User.String()
		// 尝试 base64 解码
		decoded, err := base64.URLEncoding.DecodeString(userInfo)
		if err != nil {
			decoded, err = base64.StdEncoding.DecodeString(userInfo)
		}

		if err == nil {
			userInfo = string(decoded)
		}

		// 分割 method:password
		parts := strings.SplitN(userInfo, ":", 2)
		if len(parts) == 2 {
			method = parts[0]
			password = parts[1]
		} else {
			return nil, fmt.Errorf("invalid ss url format")
		}
	}

	if method == "" || password == "" {
		return nil, fmt.Errorf("ss method and password are required")
	}

	// 从 query 参数解析 UDP 支持
	query := u.Query()
	udp := false
	if query.Get("udp") == "true" || query.Get("udp") == "1" {
		udp = true
	}

	config := SSConfig{
		Server:   host,
		Port:     port,
		Password: password,
		Method:   method,
		UDP:      udp,
	}

	return NewSSDialer(config)
}

func (s *SSDialer) Dial(network string, targetAddr string) (net.Conn, error) {
	return s.DialContext(context.Background(), network, targetAddr)
}
func (s *SSDialer) DialContext(ctx context.Context, network, targetAddr string) (net.Conn, error) {
	// UDP 检查
	if network == "udp" && !s.config.UDP {
		return nil, fmt.Errorf("UDP is not enabled for this shadowsocks proxy")
	}

	// 连接到 Shadowsocks 服务器
	serverAddr := net.JoinHostPort(s.config.Server, fmt.Sprintf("%d", s.config.Port))
	// 使用全局 DNS 解析代理服务器地址
	resolvedServerAddr := GetGlobalDNSResolver().ResolveAddr(ctx, serverAddr)
	var serverConn net.Conn
	var err error

	if s.dialer != nil {
		// 通过上游代理连接
		serverConn, err = s.dialer.DialContext(ctx, "tcp", resolvedServerAddr)
		if err != nil {
			return nil, fmt.Errorf("dial ss server through proxy error: %w", err)
		}
	} else {
		d := net.Dialer{}
		// 直接连接
		if network == "udp" {
			serverConn, err = d.DialContext(ctx, "udp", resolvedServerAddr)
		} else {
			serverConn, err = d.DialContext(ctx, "tcp", resolvedServerAddr)
		}
		if err != nil {
			return nil, fmt.Errorf("dial ss server error: %w", err)
		}
	}

	release := bindConnToDialContext(ctx, serverConn)
	defer release()

	// 使用 sing-shadowsocks2 创建加密连接
	destination := M.ParseSocksaddr(targetAddr)

	// 创建 Shadowsocks 方法
	method, err := shadowsocks2.CreateMethod(ctx, s.config.Method, shadowsocks2.MethodOptions{
		Password: s.config.Password,
	})
	if err != nil {
		serverConn.Close()
		return nil, fmt.Errorf("create shadowsocks method error: %w", err)
	}

	// 创建加密连接
	var ssConn net.Conn
	switch m := method.(type) {
	case *shadowaead.Method:
		ssConn, err = m.DialConn(serverConn, destination)
	case *shadowaead_2022.Method:
		ssConn, err = m.DialConn(serverConn, destination)
	case *shadowstream.Method:
		ssConn, err = m.DialConn(serverConn, destination)
	default:
		serverConn.Close()
		return nil, fmt.Errorf("unsupported shadowsocks method type")
	}

	if err != nil {
		serverConn.Close()
		return nil, fmt.Errorf("create shadowsocks connection error: %w", err)
	}

	if ssConn == nil {
		serverConn.Close()
		return nil, fmt.Errorf("shadowsocks returned nil connection")
	}
	release()
	return NewProxyConn(ssConn, "shadowsocks"), nil
}
