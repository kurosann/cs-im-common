package proxies

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	ssr "github.com/fengxuway/shadowsocksR"
	"github.com/sun8911879/shadowsocksR/obfs"
	"github.com/sun8911879/shadowsocksR/protocol"
	ssrlib "github.com/sun8911879/shadowsocksR/ssr"
)

var _ Dialer = (*SSRDialer)(nil)

// SSRConfig ShadowsocksR 配置结构
type SSRConfig struct {
	Server        string `json:"server" yaml:"server"`
	Port          int    `json:"port" yaml:"port"`
	Password      string `json:"password" yaml:"password"`
	Method        string `json:"method" yaml:"method"`                 // 加密方法: aes-128-cfb, aes-256-cfb, chacha20 等
	Protocol      string `json:"protocol" yaml:"protocol"`             // 协议插件: origin, auth_sha1_v4, auth_aes128_md5 等
	ProtocolParam string `json:"protocol_param" yaml:"protocol_param"` // 协议参数
	Obfs          string `json:"obfs" yaml:"obfs"`                     // 混淆插件: plain, http_simple, tls1.2_ticket_auth 等
	ObfsParam     string `json:"obfs_param" yaml:"obfs_param"`         // 混淆参数
	UDP           bool   `json:"udp" yaml:"udp"`
}

type SSRDialer struct {
	addr   string
	config SSRConfig
	dialer Dialer
}

func (s *SSRDialer) Addr() string {
	return s.addr
}

func (s *SSRDialer) SetDialer(dialer Dialer) {
	s.dialer = dialer
}

func (s *SSRDialer) ProxyHost() string {
	return s.config.Server
}

func (s *SSRDialer) Scheme() string {
	return "ssr"
}

// NewSSRDialer 使用配置对象创建 ShadowsocksR 代理客户端
func NewSSRDialer(config SSRConfig) (*SSRDialer, error) {
	// 验证必要参数
	if config.Server == "" {
		return nil, fmt.Errorf("shadowsocksr server is required")
	}
	if config.Password == "" {
		return nil, fmt.Errorf("shadowsocksr password is required")
	}
	if config.Port <= 0 || config.Port > 65535 {
		return nil, fmt.Errorf("invalid port: %d", config.Port)
	}
	if config.Method == "" {
		config.Method = "aes-256-cfb" // 默认加密方法
	}
	if config.Protocol == "" {
		config.Protocol = "origin" // 默认协议插件
	}
	if config.Obfs == "" {
		config.Obfs = "plain" // 默认混淆插件
	}

	// 构建地址字符串用于显示
	addr := fmt.Sprintf("ssr://%s@%s:%d", config.Method, config.Server, config.Port)

	return &SSRDialer{
		addr:   addr,
		config: config,
		dialer: nil,
	}, nil
}

// NewSSRDialerWithUrl 使用 URL 创建 ShadowsocksR 代理客户端
// URL 格式: ssr://base64(server:port:protocol:method:obfs:base64(password)/?obfsparam=base64(obfs_param)&protoparam=base64(protocol_param))
func NewSSRDialerWithUrl(proxyAddr string) (*SSRDialer, error) {
	u, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("parse ssr url error: %w", err)
	}

	// SSR URL 通常是 base64 编码的完整配置
	var decoded string
	if u.Host != "" {
		// 尝试解码 host 部分
		decodedBytes, err := base64.URLEncoding.DecodeString(u.Host)
		if err != nil {
			decodedBytes, err = base64.RawURLEncoding.DecodeString(u.Host)
		}
		if err != nil {
			decodedBytes, err = base64.StdEncoding.DecodeString(u.Host)
		}
		if err == nil {
			decoded = string(decodedBytes)
		} else {
			decoded = u.Host
		}
	} else {
		// 尝试从 Path 获取
		path := strings.TrimPrefix(u.Path, "/")
		decodedBytes, err := base64.URLEncoding.DecodeString(path)
		if err != nil {
			decodedBytes, err = base64.RawURLEncoding.DecodeString(path)
		}
		if err != nil {
			decodedBytes, err = base64.StdEncoding.DecodeString(path)
		}
		if err != nil {
			return nil, fmt.Errorf("invalid ssr url format: cannot decode")
		}
		decoded = string(decodedBytes)
	}

	// 解析 SSR URL 格式: server:port:protocol:method:obfs:base64(password)
	parts := strings.Split(decoded, ":")
	if len(parts) < 6 {
		return nil, fmt.Errorf("invalid ssr url format: insufficient parts")
	}

	server := parts[0]
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	protocol := parts[2]
	method := parts[3]
	obfs := parts[4]

	// 密码部分可能包含 base64 编码和参数
	passwordPart := strings.Join(parts[5:], ":")
	passwordAndParams := strings.SplitN(passwordPart, "/?", 2)

	// 解码密码
	passwordBytes, err := base64.URLEncoding.DecodeString(passwordAndParams[0])
	if err != nil {
		passwordBytes, err = base64.RawURLEncoding.DecodeString(passwordAndParams[0])
	}
	if err != nil {
		passwordBytes, err = base64.StdEncoding.DecodeString(passwordAndParams[0])
	}
	if err != nil {
		return nil, fmt.Errorf("invalid password encoding: %w", err)
	}
	password := string(passwordBytes)

	// 解析参数
	var protocolParam, obfsParam string
	if len(passwordAndParams) > 1 {
		params := strings.TrimPrefix(passwordAndParams[1], "?")
		paramPairs := strings.Split(params, "&")
		for _, pair := range paramPairs {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) == 2 {
				key := kv[0]
				value := kv[1]

				// 解码参数值
				decodedValue, err := base64.URLEncoding.DecodeString(value)
				if err != nil {
					decodedValue, err = base64.RawURLEncoding.DecodeString(value)
				}
				if err != nil {
					decodedValue, err = base64.StdEncoding.DecodeString(value)
				}
				if err == nil {
					value = string(decodedValue)
				}

				switch key {
				case "protoparam":
					protocolParam = value
				case "obfsparam":
					obfsParam = value
				}
			}
		}
	}

	// 从 query 参数解析 UDP 支持
	query := u.Query()
	udp := false
	if query.Get("udp") == "true" || query.Get("udp") == "1" {
		udp = true
	}

	config := SSRConfig{
		Server:        server,
		Port:          port,
		Password:      password,
		Method:        method,
		Protocol:      protocol,
		ProtocolParam: protocolParam,
		Obfs:          obfs,
		ObfsParam:     obfsParam,
		UDP:           udp,
	}

	return NewSSRDialer(config)
}

func (s *SSRDialer) Dial(network string, targetAddr string) (net.Conn, error) {
	return s.DialContext(context.Background(), network, targetAddr)
}
func (s *SSRDialer) DialContext(ctx context.Context, network, targetAddr string) (net.Conn, error) {
	// UDP 检查
	if network == "udp" && !s.config.UDP {
		return nil, fmt.Errorf("UDP is not enabled for this shadowsocksr proxy")
	}

	// 连接到 ShadowsocksR 服务器
	serverAddr := net.JoinHostPort(s.config.Server, fmt.Sprintf("%d", s.config.Port))
	// 使用全局 DNS 解析代理服务器地址
	resolvedServerAddr := GetGlobalDNSResolver().ResolveAddr(ctx, serverAddr)
	var serverConn net.Conn
	var err error

	if s.dialer != nil {
		// 通过上游代理连接
		serverConn, err = s.dialer.DialContext(ctx, "tcp", resolvedServerAddr)
		if err != nil {
			return nil, fmt.Errorf("dial ssr server through proxy error: %w", err)
		}
	} else {
		d := net.Dialer{}
		// 直接连接
		serverConn, err = d.DialContext(ctx, "tcp", resolvedServerAddr)
		if err != nil {
			return nil, fmt.Errorf("dial ssr server error: %w", err)
		}
	}

	release := bindConnToDialContext(ctx, serverConn)
	defer release()

	// 创建 StreamCipher
	cipher, err := ssr.NewStreamCipher(s.config.Method, s.config.Password)
	if err != nil {
		serverConn.Close()
		return nil, fmt.Errorf("create stream cipher error: %w", err)
	}

	// 创建 SSTCPConn
	ssrConn := ssr.NewSSTCPConn(serverConn, cipher)
	if ssrConn == nil || ssrConn.Conn == nil {
		serverConn.Close()
		return nil, fmt.Errorf("create ssr connection failed")
	}

	// 设置混淆插件
	ssrConn.IObfs = obfs.NewObfs(s.config.Obfs)
	if ssrConn.IObfs == nil {
		serverConn.Close()
		return nil, fmt.Errorf("unsupported obfs: %s", s.config.Obfs)
	}

	// 设置协议插件
	ssrConn.IProtocol = protocol.NewProtocol(s.config.Protocol)
	if ssrConn.IProtocol == nil {
		serverConn.Close()
		return nil, fmt.Errorf("unsupported protocol: %s", s.config.Protocol)
	}

	// 解析目标地址为 SOCKS5 格式
	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		ssrConn.Close()
		return nil, fmt.Errorf("invalid target address: %w", err)
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		ssrConn.Close()
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	// 构建 SOCKS5 地址格式
	var rawaddr []byte
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			// IPv4
			rawaddr = append([]byte{0x01}, ip4...)
		} else {
			// IPv6
			rawaddr = append([]byte{0x04}, ip...)
		}
	} else {
		// 域名
		rawaddr = append([]byte{0x03, byte(len(host))}, []byte(host)...)
	}
	// 添加端口
	rawaddr = append(rawaddr, byte(port>>8), byte(port&0xff))

	// 初始化混淆插件的 data 字段（某些混淆插件需要）
	if ssrConn.IObfs.GetData() == nil {
		// GetData 会自动初始化 data 字段
		_ = ssrConn.IObfs.GetData()
	}

	// 初始化协议插件的 data 字段（某些协议插件需要）
	if ssrConn.IProtocol.GetData() == nil {
		// GetData 会自动初始化 data 字段
		_ = ssrConn.IProtocol.GetData()
	}

	// 先设置基本的 ServerInfo
	obfsServerInfo := &ssrlib.ServerInfoForObfs{
		Host:   s.config.Server,
		Port:   uint16(s.config.Port),
		TcpMss: 1460,
		Param:  s.config.ObfsParam,
	}
	ssrConn.IObfs.SetServerInfo(obfsServerInfo)

	protocolServerInfo := &ssrlib.ServerInfoForObfs{
		Host:   s.config.Server,
		Port:   uint16(s.config.Port),
		TcpMss: 1460,
		Param:  s.config.ProtocolParam,
	}
	ssrConn.IProtocol.SetServerInfo(protocolServerInfo)

	// 写入目标地址（这会触发 initEncryptor 来初始化 Key 和 IV）
	if _, err := ssrConn.Write(rawaddr); err != nil {
		ssrConn.Close()
		return nil, fmt.Errorf("write target address error: %w", err)
	}

	if ssrConn == nil {
		return nil, errors.New("ssrConn is nil")
	}

	release()
	return NewProxyConn(ssrConn, "shadowsocksr"), nil
}
