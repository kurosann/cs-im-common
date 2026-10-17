package proxies

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"vane-common/pkg/logger"

	"github.com/google/uuid"
	"github.com/sagernet/sing-vmess/vless"
	vlessLogger "github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	"go.uber.org/zap"
)

var _ Dialer = (*VlessDialer)(nil)

// VlessConfig VLESS 配置结构
type VlessConfig struct {
	Server         string `json:"server" yaml:"server"`
	Port           int    `json:"port" yaml:"port"`
	UUID           string `json:"uuid" yaml:"uuid"`             // 用户 ID
	Flow           string `json:"flow" yaml:"flow"`             // 流控模式: "", "xtls-rprx-vision"
	Encryption     string `json:"encryption" yaml:"encryption"` // 加密方式: "none"
	Cipher         string `json:"cipher" yaml:"cipher"`         // 加密方式别名，与 encryption 同义
	Network        string `json:"network" yaml:"network"`       // 传输协议: tcp, ws
	UDP            bool   `json:"udp" yaml:"udp"`               // 是否启用 UDP
	TLS            bool   `json:"tls" yaml:"tls"`
	SkipCertVerify bool   `json:"skip-cert-verify" yaml:"skip-cert-verify"`
	SNI            string `json:"sni" yaml:"sni"`               // TLS ServerName
	ServerName     string `json:"servername" yaml:"servername"` // SNI 别名
	AlterId        int    `json:"alterId" yaml:"alterId"`       // 兼容字段，VLESS 不使用
	// WebSocket 相关
	WSPath    string            `json:"ws-path" yaml:"ws-path"`
	WSHeaders map[string]string `json:"ws-headers" yaml:"ws-headers"`
}

type VlessDialer struct {
	addr   string
	config VlessConfig
	uuid   uuid.UUID
	dialer Dialer
}

// NewVLESSDialer 使用配置对象创建 VLESS 代理客户端
func NewVlessDialer(config VlessConfig) (*VlessDialer, error) {
	// 验证必要参数
	if config.Server == "" {
		return nil, fmt.Errorf("vless server is required")
	}
	if config.UUID == "" {
		return nil, fmt.Errorf("vless uuid is required")
	}
	if config.Port <= 0 || config.Port > 65535 {
		return nil, fmt.Errorf("invalid port: %d", config.Port)
	}

	// 解析 UUID
	parsedUUID, err := uuid.Parse(config.UUID)
	if err != nil {
		return nil, fmt.Errorf("invalid uuid: %w", err)
	}

	// 处理别名字段
	if config.Encryption == "" && config.Cipher != "" {
		config.Encryption = config.Cipher
	}
	if config.SNI == "" && config.ServerName != "" {
		config.SNI = config.ServerName
	}

	// VLESS 协议只支持 "none" 加密，强制设置
	// 任何其他值（包括 "auto"）都应该被设置为 "none"
	config.Encryption = "none"
	if config.Network == "" {
		config.Network = "tcp"
	}
	if config.SNI == "" {
		config.SNI = config.Server
	}

	// 构建地址字符串用于显示
	addr := fmt.Sprintf("vless://%s@%s:%d", config.UUID, config.Server, config.Port)

	return &VlessDialer{
		addr:   addr,
		config: config,
		uuid:   parsedUUID,
		dialer: nil,
	}, nil
}

// NewVLESSDialerWithUrl 使用 URL 创建 VLESS 代理客户端
// URL 格式: vless://uuid@server:port?encryption=none&type=ws&security=tls
func NewVlessDialerWithUrl(proxyAddr string) (*VlessDialer, error) {
	u, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("parse vless url error: %w", err)
	}

	// 解析 UUID
	var userID string
	if u.User != nil {
		userID = u.User.Username()
	}
	if userID == "" {
		return nil, fmt.Errorf("vless uuid is required")
	}

	// 解析服务器和端口
	host := u.Hostname()
	portStr := u.Port()
	if host == "" {
		return nil, fmt.Errorf("vless server is required")
	}

	port := 443 // 默认端口
	if portStr != "" {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
	}

	// 从 query 参数解析配置
	query := u.Query()

	// 加密方式（encryption 或 cipher）
	encryption := query.Get("encryption")
	if encryption == "" {
		encryption = query.Get("cipher")
	}
	if encryption == "" {
		encryption = "none"
	}

	// 网络类型
	network := query.Get("type")
	if network == "" {
		network = "tcp"
	}

	// UDP 支持
	udp := false
	if query.Get("udp") == "true" || query.Get("udp") == "1" {
		udp = true
	}

	// TLS 配置
	security := query.Get("security")
	useTLS := security == "tls" || security == "xtls"

	skipVerify := false
	if query.Get("allowInsecure") == "1" || query.Get("allowInsecure") == "true" ||
		query.Get("skip-cert-verify") == "1" || query.Get("skip-cert-verify") == "true" {
		skipVerify = true
	}

	// SNI（sni 或 servername）
	sni := query.Get("sni")
	if sni == "" {
		sni = query.Get("servername")
	}
	if sni == "" {
		sni = host
	}

	// 流控
	flow := query.Get("flow")

	// WebSocket 配置
	wsPath := query.Get("path")
	wsHost := query.Get("host")
	wsHeaders := make(map[string]string)
	if wsHost != "" {
		wsHeaders["Host"] = wsHost
	}

	config := VlessConfig{
		Server:         host,
		Port:           port,
		UUID:           userID,
		Flow:           flow,
		Encryption:     encryption,
		Network:        network,
		UDP:            udp,
		TLS:            useTLS,
		SkipCertVerify: skipVerify,
		SNI:            sni,
		WSPath:         wsPath,
		WSHeaders:      wsHeaders,
	}

	return NewVlessDialer(config)
}

func (v *VlessDialer) Addr() string {
	return v.addr
}

func (v *VlessDialer) SetDialer(dialer Dialer) {
	v.dialer = dialer
}

func (v *VlessDialer) ProxyHost() string {
	return v.config.Server
}

func (v *VlessDialer) Scheme() string {
	return "vless"
}

func (v *VlessDialer) Dial(network string, targetAddr string) (net.Conn, error) {
	return v.DialContext(context.Background(), network, targetAddr)
}
func (v *VlessDialer) DialContext(ctx context.Context, network, targetAddr string) (net.Conn, error) {
	// UDP 检查
	if network == "udp" && !v.config.UDP {
		return nil, fmt.Errorf("UDP is not enabled for this vless proxy")
	}

	// 连接到 VLESS 服务器
	serverAddr := net.JoinHostPort(v.config.Server, fmt.Sprintf("%d", v.config.Port))
	// 使用全局 DNS 解析代理服务器地址
	resolvedServerAddr := GetGlobalDNSResolver().ResolveAddr(ctx, serverAddr)
	var serverConn net.Conn
	var err error

	if v.dialer != nil {
		// 通过上游代理连接
		serverConn, err = v.dialer.DialContext(ctx, "tcp", resolvedServerAddr)
		if err != nil {
			return nil, fmt.Errorf("dial vless server through proxy error: %w", err)
		}
	} else {
		// 直接连接
		d := net.Dialer{}
		serverConn, err = d.DialContext(ctx, "tcp", resolvedServerAddr)
		if err != nil {
			return nil, fmt.Errorf("dial vless server error: %w", err)
		}
	}

	release := bindConnToDialContext(ctx, serverConn)
	defer release()

	// TLS 握手
	if v.config.TLS {
		tlsConfig := &tls.Config{
			ServerName:         v.config.SNI,
			InsecureSkipVerify: v.config.SkipCertVerify,
		}
		tlsConn := tls.Client(serverConn, tlsConfig)
		if err := tlsConn.Handshake(); err != nil {
			serverConn.Close()
			return nil, fmt.Errorf("vless tls handshake error: %w", err)
		}
		serverConn = tlsConn
	}

	// 使用 sing-vmess 的 VLESS 客户端
	// 创建 VLESS 客户端（参考 sing-box 的实现）
	vlessClient, err := vless.NewClient(
		v.uuid.String(),
		v.config.Flow,
		NewVlessLogger(), // logger，可以为 nil
	)
	if err != nil {
		serverConn.Close()
		return nil, fmt.Errorf("create vless client error: %w", err)
	}

	// 解析目标地址
	destination := M.ParseSocksaddr(targetAddr)

	var vlessConn net.Conn
	if network == "udp" {
		vlessConn, err = vlessClient.DialEarlyPacketConn(serverConn, destination)
		if err != nil {
			serverConn.Close()
			return nil, fmt.Errorf("dial vless packet connection error: %w", err)
		}
	} else {
		// 建立 VLESS 连接
		vlessConn, err = vlessClient.DialEarlyConn(serverConn, destination)
		if err != nil {
			serverConn.Close()
			return nil, fmt.Errorf("dial vless connection error: %w", err)
		}
	}
	if vlessConn == nil {
		serverConn.Close()
		return nil, fmt.Errorf("vless returned nil connection")
	}
	release()
	return NewProxyConn(vlessConn, "vless"), nil
}

var _ vlessLogger.Logger = (*VlessLogger)(nil)

type VlessLogger struct {
	l *zap.Logger
}

func NewVlessLogger() *VlessLogger {
	return &VlessLogger{
		l: logger.GetInternalLogger().WithOptions(zap.AddCallerSkip(1)),
	}
}
func (v *VlessLogger) formatArgs(args ...any) string {
	msg := strings.Builder{}
	for _, arg := range args {
		msg.WriteString(fmt.Sprintf("%v", arg))
	}
	return msg.String()
}

func (v *VlessLogger) Debug(args ...any) {
	v.l.Sugar().Debugf(v.formatArgs(args...))
}

func (v *VlessLogger) Error(args ...any) {
	v.l.Sugar().Errorf(v.formatArgs(args...))
}

func (v *VlessLogger) Fatal(args ...any) {
	v.l.Sugar().Fatalf(v.formatArgs(args...))
}

func (v *VlessLogger) Info(args ...any) {
	v.l.Sugar().Infof(v.formatArgs(args...))
}

func (v *VlessLogger) Panic(args ...any) {
	v.l.Sugar().Panicf(v.formatArgs(args...))
}

func (v *VlessLogger) Trace(args ...any) {
	v.l.Sugar().Debugf(v.formatArgs(args...))
}

func (v *VlessLogger) Warn(args ...any) {
	v.l.Sugar().Warnf(v.formatArgs(args...))
}
