package proxies

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
)

var _ Dialer = (*TrojanDialer)(nil)

const (
	// Trojan 命令
	commandTCP byte = 1
	commandUDP byte = 3

	// 地址类型
	atypIPv4       byte = 1
	atypDomainName byte = 3
	atypIPv6       byte = 4
)

// TrojanConfig Trojan 配置结构
type TrojanConfig struct {
	Server         string `json:"server" yaml:"server"`
	Port           int    `json:"port" yaml:"port"`
	Password       string `json:"password" yaml:"password"`
	UDP            bool   `json:"udp" yaml:"udp"`
	SkipCertVerify bool   `json:"skip-cert-verify" yaml:"skip-cert-verify"`
	SNI            string `json:"sni" yaml:"sni"` // 可选的 SNI
}

type TrojanDialer struct {
	addr     string
	url      *url.URL
	password string
	key      [56]byte // sha224 的结果是 28 字节，hex 编码后是 56 字节

	dialer Dialer
	config TrojanConfig
}

// NewTrojanDialerWithUrl 使用 URL 创建 Trojan 代理客户端
// proxyAddr 格式: trojan://password@server:port?udp=true&skip-cert-verify=true&sni=example.com
func NewTrojanDialerWithUrl(proxyAddr string) (*TrojanDialer, error) {
	u, err := url.Parse(proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("parse trojan url error: %w", err)
	}

	// 解析密码
	password := ""
	if u.User != nil {
		password = u.User.Username()
	}
	if password == "" {
		return nil, fmt.Errorf("trojan password is required")
	}

	// 解析服务器和端口
	host := u.Hostname()
	portStr := u.Port()
	if host == "" {
		return nil, fmt.Errorf("trojan server is required")
	}

	port := 443 // 默认端口
	if portStr != "" {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return nil, fmt.Errorf("invalid port: %w", err)
		}
	}

	query := u.Query()

	udp := false
	if query.Get("udp") == "true" || query.Get("udp") == "1" {
		udp = true
	}

	skipVerify := false
	if query.Get("allowInsecure") == "1" || query.Get("allowInsecure") == "true" ||
		query.Get("skip-cert-verify") == "1" || query.Get("skip-cert-verify") == "true" {
		skipVerify = true
	}

	sni := query.Get("sni")
	if sni == "" {
		if peer := query.Get("peer"); peer != "" {
			sni = peer
		}
	}
	if sni == "" {
		sni = host
	}

	// 构建配置对象
	config := TrojanConfig{
		Server:         host,
		Port:           port,
		Password:       password,
		UDP:            udp,
		SkipCertVerify: skipVerify,
		SNI:            sni,
	}

	key := generateTrojanKey(password)

	return &TrojanDialer{
		addr:     proxyAddr,
		url:      u,
		password: password,
		key:      key,
		dialer:   nil,
		config:   config,
	}, nil
}

// NewTrojanDialer 使用配置对象创建 Trojan 代理客户端
func NewTrojanDialer(config TrojanConfig) (*TrojanDialer, error) {
	if config.Server == "" {
		return nil, fmt.Errorf("trojan server is required")
	}
	if config.Password == "" {
		return nil, fmt.Errorf("trojan password is required")
	}
	if config.Port <= 0 || config.Port > 65535 {
		return nil, fmt.Errorf("invalid port: %d", config.Port)
	}

	key := generateTrojanKey(config.Password)

	addr := fmt.Sprintf("trojan://%s@%s:%d", config.Password, config.Server, config.Port)

	if config.SNI == "" {
		config.SNI = config.Server
	}

	return &TrojanDialer{
		addr:     addr,
		password: config.Password,
		key:      key,
		dialer:   nil,
		config:   config,
	}, nil
}

func (t *TrojanDialer) Addr() string {
	return t.addr
}

func (t *TrojanDialer) SetDialer(dialer Dialer) {
	t.dialer = dialer
}

func (t *TrojanDialer) Dial(network string, targetAddr string) (net.Conn, error) {
	return t.DialContext(context.Background(), network, targetAddr)
}
func (t *TrojanDialer) DialContext(ctx context.Context, network, targetAddr string) (net.Conn, error) {
	// 连接到 Trojan 服务器
	serverAddr := net.JoinHostPort(t.config.Server, fmt.Sprintf("%d", t.config.Port))
	// 使用全局 DNS 解析代理服务器地址
	resolvedServerAddr := GetGlobalDNSResolver().ResolveAddr(ctx, serverAddr)
	var serverConn net.Conn
	var err error

	if t.dialer != nil {
		serverConn, err = t.dialer.DialContext(ctx, "tcp", resolvedServerAddr)
		if err != nil {
			return nil, fmt.Errorf("dial trojan server through proxy error: %w", err)
		}
	} else {
		d := net.Dialer{}
		serverConn, err = d.DialContext(ctx, "tcp", resolvedServerAddr)
		if err != nil {
			return nil, fmt.Errorf("dial trojan server error: %w", err)
		}
	}

	release := bindConnToDialContext(ctx, serverConn)
	defer release()

	tlsConfig := &tls.Config{
		ServerName:         t.config.SNI,
		InsecureSkipVerify: t.config.SkipCertVerify,
	}
	tlsConn := tls.Client(serverConn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		serverConn.Close()
		return nil, fmt.Errorf("trojan tls handshake error: %w", err)
	}

	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("invalid target address: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("invalid port: %w", err)
	}

	var command byte
	if network == "udp" {
		if !t.config.UDP {
			tlsConn.Close()
			return nil, fmt.Errorf("UDP is not enabled for this trojan proxy")
		}
		command = commandUDP
	} else {
		command = commandTCP
	}

	if err := t.writeTrojanRequest(tlsConn, command, host, uint16(port)); err != nil {
		tlsConn.Close()
		return nil, fmt.Errorf("write trojan request error: %w", err)
	}
	if tlsConn == nil {
		return nil, errors.New("trojan conn is nil")
	}
	if network == "udp" {
		release()
		return NewProxyConn(&TrojanUDPConn{
			Conn:       tlsConn,
			targetHost: host,
			targetPort: uint16(port),
		}, "trojan"), nil
	}

	release()
	return NewProxyConn(tlsConn, "trojan"), nil
}

func (t *TrojanDialer) ProxyHost() string {
	return t.config.Server
}

func (t *TrojanDialer) Scheme() string {
	return "trojan"
}

// writeTrojanRequest 写入 Trojan 协议请求头
// 格式: [密码(56字节hex)][CRLF][命令(1字节)][地址类型(1字节)][地址][端口(2字节)][CRLF]
func (t *TrojanDialer) writeTrojanRequest(conn net.Conn, command byte, host string, port uint16) error {
	buf := make([]byte, 0, 256)

	// 写入密码的 hex 编码
	buf = append(buf, t.key[:]...)

	// 写入 CRLF
	buf = append(buf, '\r', '\n')

	// 写入命令
	buf = append(buf, command)

	// 写入地址
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			// IPv4
			buf = append(buf, atypIPv4)
			buf = append(buf, ip4...)
		} else {
			// IPv6
			buf = append(buf, atypIPv6)
			buf = append(buf, ip...)
		}
	} else {
		// 域名
		buf = append(buf, atypDomainName)
		buf = append(buf, byte(len(host)))
		buf = append(buf, []byte(host)...)
	}

	// 写入端口（大端序）
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, port)
	buf = append(buf, portBytes...)

	// 写入 CRLF
	buf = append(buf, '\r', '\n')

	// 发送请求
	_, err := conn.Write(buf)
	return err
}

// generateTrojanKey 生成 Trojan 密钥
// Trojan 使用 SHA224(password) 然后转换为 hex 字符串
func generateTrojanKey(password string) [56]byte {
	hash := sha256.Sum224([]byte(password))
	hexStr := hex.EncodeToString(hash[:])
	var key [56]byte
	copy(key[:], hexStr)
	return key
}

// TrojanUDPConn 包装 Trojan UDP 连接
// UDP 数据包格式: [ATYP][地址][端口][长度(2字节)][UDP数据][CRLF]
type TrojanUDPConn struct {
	net.Conn
	targetHost string
	targetPort uint16
}

// Read 读取 UDP 数据包
// Trojan UDP 响应格式: [ATYP][地址][端口][长度(2字节)][数据][CRLF]
func (c *TrojanUDPConn) Read(b []byte) (n int, err error) {
	// 读取地址类型
	atyp := make([]byte, 1)
	if _, err := io.ReadFull(c.Conn, atyp); err != nil {
		return 0, err
	}

	// 根据地址类型读取地址
	var addrLen int
	switch atyp[0] {
	case atypIPv4:
		addrLen = 4
	case atypIPv6:
		addrLen = 16
	case atypDomainName:
		lenByte := make([]byte, 1)
		if _, err := io.ReadFull(c.Conn, lenByte); err != nil {
			return 0, err
		}
		addrLen = int(lenByte[0])
	default:
		return 0, fmt.Errorf("unknown address type: %d", atyp[0])
	}

	// 跳过地址和端口
	skipBytes := make([]byte, addrLen+2) // 地址 + 2字节端口
	if _, err := io.ReadFull(c.Conn, skipBytes); err != nil {
		return 0, err
	}

	// 读取数据长度
	lenBytes := make([]byte, 2)
	if _, err := io.ReadFull(c.Conn, lenBytes); err != nil {
		return 0, err
	}
	dataLen := binary.BigEndian.Uint16(lenBytes)

	// 读取实际数据
	if int(dataLen) > len(b) {
		return 0, fmt.Errorf("buffer too small: need %d, have %d", dataLen, len(b))
	}
	if _, err := io.ReadFull(c.Conn, b[:dataLen]); err != nil {
		return 0, err
	}

	// 读取 CRLF
	crlf := make([]byte, 2)
	if _, err := io.ReadFull(c.Conn, crlf); err != nil {
		return 0, err
	}

	return int(dataLen), nil
}

// Write 写入 UDP 数据包
// Trojan UDP 请求格式: [ATYP][地址][端口][长度(2字节)][数据][CRLF]
func (c *TrojanUDPConn) Write(b []byte) (n int, err error) {
	buf := make([]byte, 0, 256+len(b))

	// 写入地址类型和地址
	if ip := net.ParseIP(c.targetHost); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			// IPv4
			buf = append(buf, atypIPv4)
			buf = append(buf, ip4...)
		} else {
			// IPv6
			buf = append(buf, atypIPv6)
			buf = append(buf, ip...)
		}
	} else {
		// 域名
		buf = append(buf, atypDomainName)
		buf = append(buf, byte(len(c.targetHost)))
		buf = append(buf, []byte(c.targetHost)...)
	}

	// 写入端口
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, c.targetPort)
	buf = append(buf, portBytes...)

	// 写入数据长度
	lenBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(lenBytes, uint16(len(b)))
	buf = append(buf, lenBytes...)

	// 写入数据
	buf = append(buf, b...)

	// 写入 CRLF
	buf = append(buf, '\r', '\n')

	// 发送完整的 UDP 包
	if _, err := c.Conn.Write(buf); err != nil {
		return 0, err
	}

	return len(b), nil
}

func (c *TrojanUDPConn) Close() error {
	return c.Conn.Close()
}
