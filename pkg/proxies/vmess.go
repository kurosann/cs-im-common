package proxies

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	vmess "github.com/sagernet/sing-vmess"
	M "github.com/sagernet/sing/common/metadata"
	"golang.org/x/net/websocket"
)

var _ Dialer = (*VMessDialer)(nil)

// VMessConfig VMess 配置结构
type VMessConfig struct {
	Server         string `json:"server" yaml:"server"`
	Port           int    `json:"port" yaml:"port"`
	UUID           string `json:"uuid" yaml:"uuid"`
	AlterId        int    `json:"alterId" yaml:"alterId"` // 额外 ID，一般为 0
	Cipher         string `json:"cipher" yaml:"cipher"`   // 加密方式: auto, aes-128-gcm, chacha20-poly1305 等
	Network        string `json:"network" yaml:"network"` // 传输协议: tcp, ws
	TLS            bool   `json:"tls" yaml:"tls"`
	SkipCertVerify bool   `json:"skip-cert-verify" yaml:"skip-cert-verify"`
	SNI            string `json:"sni" yaml:"sni"`
	// WebSocket 相关
	WSPath    string            `json:"ws-path" yaml:"ws-path"`
	WSHeaders map[string]string `json:"ws-headers" yaml:"ws-headers"`
}

type VMessDialer struct {
	addr   string
	config VMessConfig
	dialer Dialer

	dnsResolver *DNSResolver
}

// NewVMessDialer 使用配置对象创建 VMess 代理客户端
func NewVMessDialer(config VMessConfig) (*VMessDialer, error) {
	// 验证必要参数
	if config.Server == "" {
		return nil, fmt.Errorf("vmess server is required")
	}
	if config.UUID == "" {
		return nil, fmt.Errorf("vmess uuid is required")
	}
	if config.Port <= 0 || config.Port > 65535 {
		return nil, fmt.Errorf("invalid port: %d", config.Port)
	}

	// 设置默认值
	if config.Cipher == "" {
		config.Cipher = "auto"
	}
	if config.Network == "" {
		config.Network = "tcp"
	}
	if config.SNI == "" {
		config.SNI = config.Server
	}

	// 构建地址字符串用于显示
	addr := fmt.Sprintf("vmess://%s@%s:%d", config.UUID, config.Server, config.Port)

	return &VMessDialer{
		addr:   addr,
		config: config,
		dialer: nil,
	}, nil
}

// NewVMessDialerWithUrl 使用 URL 创建 VMess 代理客户端
// URL 格式: vmess://base64(json配置)
func NewVMessDialerWithUrl(proxyAddr string) (*VMessDialer, error) {
	const prefix = "vmess://"
	if !strings.HasPrefix(proxyAddr, prefix) {
		return nil, fmt.Errorf("parse vmess url error: invalid vmess url")
	}

	// VMess 分享链接通常直接把 base64 payload 放在 scheme 后面，不能依赖 url.Parse，
	// 否则 payload 里的 '/' 会被错误拆成路径。
	b64 := strings.TrimSpace(proxyAddr[len(prefix):])
	if idx := strings.IndexAny(b64, "?#"); idx >= 0 {
		b64 = b64[:idx]
	}
	if b64 == "" {
		return nil, fmt.Errorf("invalid vmess url: missing config")
	}

	decoded, err := decodeVMessConfig(b64)
	if err != nil {
		return nil, fmt.Errorf("decode vmess config error: %w", err)
	}

	// 解析旧格式的 VMess 配置
	var oldConfig struct {
		Name     string      `json:"ps"`
		Server   string      `json:"add"`
		Port     interface{} `json:"port"` // 可能是字符串或数字
		Cipher   string      `json:"scy"`
		UUID     string      `json:"id"`
		Aid      interface{} `json:"aid"` // 可能是字符串或数字
		Network  string      `json:"net"`
		TLS      interface{} `json:"tls"`  // 可能是字符串或布尔
		Host     interface{} `json:"host"` // 可能是字符串或对象
		WsPath   string      `json:"path"`
		SNI      string      `json:"sni"`
		Insecure interface{} `json:"insecure"` // 可能是字符串、数字或布尔
	}

	if err := json.Unmarshal(decoded, &oldConfig); err != nil {
		return nil, fmt.Errorf("parse vmess config error: %w", err)
	}

	// 转换端口
	port := 0
	switch v := oldConfig.Port.(type) {
	case string:
		port, _ = strconv.Atoi(v)
	case float64:
		port = int(v)
	case int:
		port = v
	}

	// 转换 AlterId
	alterId := 0
	switch v := oldConfig.Aid.(type) {
	case string:
		alterId, _ = strconv.Atoi(v)
	case float64:
		alterId = int(v)
	case int:
		alterId = v
	}

	// 转换 TLS
	useTLS := false
	switch v := oldConfig.TLS.(type) {
	case string:
		useTLS = (v == "tls" || v == "true")
	case bool:
		useTLS = v
	}

	// 处理 WebSocket Headers
	wsHeaders := parseVMessWSHeaders(oldConfig.Host)

	config := VMessConfig{
		Server:         oldConfig.Server,
		Port:           port,
		UUID:           oldConfig.UUID,
		AlterId:        alterId,
		Cipher:         oldConfig.Cipher,
		Network:        oldConfig.Network,
		TLS:            useTLS,
		SkipCertVerify: parseVMessBool(oldConfig.Insecure),
		SNI:            oldConfig.SNI,
		WSPath:         oldConfig.WsPath,
		WSHeaders:      wsHeaders,
	}

	return NewVMessDialer(config)
}

func decodeVMessConfig(encoded string) ([]byte, error) {
	compact := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\r', '\n':
			return -1
		default:
			return r
		}
	}, encoded)

	var decodeErr error
	for _, encoding := range []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	} {
		decoded, err := encoding.DecodeString(compact)
		if err == nil {
			return decoded, nil
		}
		decodeErr = err
	}
	return nil, decodeErr
}

func parseVMessWSHeaders(hostField interface{}) map[string]string {
	wsHeaders := make(map[string]string)
	switch v := hostField.(type) {
	case string:
		if v != "" {
			wsHeaders["Host"] = v
		}
	case map[string]interface{}:
		for key, value := range v {
			str, ok := value.(string)
			if ok && str != "" {
				wsHeaders[key] = str
			}
		}
	case map[string]string:
		for key, value := range v {
			if value != "" {
				wsHeaders[key] = value
			}
		}
	}
	return wsHeaders
}

func parseVMessBool(value interface{}) bool {
	switch v := value.(type) {
	case string:
		switch strings.ToLower(v) {
		case "1", "true", "tls":
			return true
		}
	case bool:
		return v
	case float64:
		return v != 0
	case int:
		return v != 0
	}
	return false
}

func (v *VMessDialer) Addr() string {
	return v.addr
}

func (v *VMessDialer) SetDialer(dialer Dialer) {
	v.dialer = dialer
}

// upgradeToWebSocket 将已有的连接升级为 WebSocket 连接
func upgradeToWebSocket(conn net.Conn, config VMessConfig) (net.Conn, error) {
	wsPath := config.WSPath
	if wsPath == "" {
		wsPath = "/"
	}
	// 确保路径以 / 开头
	if !strings.HasPrefix(wsPath, "/") {
		wsPath = "/" + wsPath
	}

	// 准备 WebSocket headers
	wsHeaders := make(http.Header)
	if config.WSHeaders != nil {
		for k, v := range config.WSHeaders {
			wsHeaders.Set(k, v)
		}
	}
	// 设置默认的 User-Agent（如果没有指定）
	if wsHeaders.Get("User-Agent") == "" {
		wsHeaders.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	}

	// 构建 WebSocket URL（用于配置）
	scheme := "ws"
	if config.TLS {
		scheme = "wss"
	}
	wsURL := fmt.Sprintf("%s://%s:%d%s", scheme, config.Server, config.Port, wsPath)

	// 创建 WebSocket 配置
	wsConfig, err := websocket.NewConfig(wsURL, "http://localhost")
	if err != nil {
		return nil, fmt.Errorf("create websocket config error: %w", err)
	}

	// 设置 headers
	for k, values := range wsHeaders {
		for _, v := range values {
			wsConfig.Header.Add(k, v)
		}
	}

	// 设置 Host header
	wsConfig.Header.Set("Host", fmt.Sprintf("%s:%d", config.Server, config.Port))

	// 使用已有的连接进行 WebSocket 握手
	// websocket.NewClient 函数接受已有的 net.Conn 并升级为 WebSocket 连接
	wsConn, err := websocket.NewClient(wsConfig, conn)
	if err != nil {
		return nil, fmt.Errorf("websocket handshake error: %w", err)
	}

	// websocket.Conn 已经实现了 net.Conn 接口，可以直接使用
	return wsConn, nil
}

func (v *VMessDialer) Dial(network, targetAddr string) (net.Conn, error) {
	return v.DialContext(context.Background(), network, targetAddr)
}
func (v *VMessDialer) DialContext(ctx context.Context, network, targetAddr string) (net.Conn, error) {
	config := v.config

	// 设置 Security
	security := "auto"
	if config.Cipher != "" {
		security = config.Cipher
	}

	// 构建 VMess 客户端选项
	var clientOptions []vmess.ClientOption
	// 可以根据需要添加其他选项，如 ClientWithGlobalPadding() 等

	// 创建 VMess 客户端
	client, err := vmess.NewClient(config.UUID, security, config.AlterId, clientOptions...)
	if err != nil {
		return nil, fmt.Errorf("create vmess client error: %w", err)
	}

	// 先连接到 VMess 服务器
	serverAddr := net.JoinHostPort(config.Server, fmt.Sprintf("%d", config.Port))
	// 使用全局 DNS 解析代理服务器地址
	resolvedServerAddr := GetGlobalDNSResolver().ResolveAddr(ctx, serverAddr)
	var serverConn net.Conn

	if v.dialer != nil {
		// 通过上游代理连接
		serverConn, err = v.dialer.DialContext(ctx, "tcp", resolvedServerAddr)
		if err != nil {
			return nil, fmt.Errorf("dial vmess server through proxy error: %w", err)
		}
	} else {
		// 直接连接
		d := net.Dialer{}
		serverConn, err = d.DialContext(ctx, "tcp", resolvedServerAddr)
		if err != nil {
			return nil, fmt.Errorf("dial vmess server error: %w", err)
		}
	}

	release := bindConnToDialContext(ctx, serverConn)
	defer release()

	// 如果配置了 TLS，进行 TLS 握手
	if config.TLS {
		tlsConfig := &tls.Config{
			ServerName:         config.SNI,
			InsecureSkipVerify: config.SkipCertVerify,
		}
		tlsConn := tls.Client(serverConn, tlsConfig)
		if err := tlsConn.Handshake(); err != nil {
			serverConn.Close()
			return nil, fmt.Errorf("tls handshake error: %w", err)
		}
		serverConn = tlsConn
	}

	// 如果配置了 WebSocket，需要先建立 WebSocket 连接
	if config.Network == "ws" || config.Network == "websocket" {
		wsConn, err := upgradeToWebSocket(serverConn, config)
		if err != nil {
			serverConn.Close()
			return nil, err
		}
		serverConn = wsConn
	}

	// 解析目标地址
	host, port, err := net.SplitHostPort(targetAddr)
	if err != nil {
		serverConn.Close()
		return nil, err
	}
	portInt, err := strconv.Atoi(port)
	if err != nil {
		serverConn.Close()
		return nil, err
	}

	// 创建 destination
	destination := M.ParseSocksaddrHostPort(host, uint16(portInt))

	// 使用 VMess 客户端建立代理连接
	var conn net.Conn
	if network == "udp" {
		packetConn, err := client.DialPacketConn(serverConn, destination)
		if err != nil {
			serverConn.Close()
			return nil, fmt.Errorf("dial vmess packet conn error: %w", err)
		}
		conn = packetConn
	} else {
		extendedConn, err := client.DialConn(serverConn, destination)
		if err != nil {
			serverConn.Close()
			return nil, fmt.Errorf("dial vmess conn error: %w", err)
		}
		conn = extendedConn
	}
	if conn == nil {
		return nil, fmt.Errorf("vmess returned nil connection")
	}

	release()
	return NewProxyConn(conn, "vmess"), nil
}

func (v *VMessDialer) ProxyHost() string {
	return v.config.Server
}

func (v *VMessDialer) Scheme() string {
	return "vmess"
}
