package proxies

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"time"

	"golang.org/x/net/proxy"
)

func NewProxyDialerWithUrl(proxyUrl string) (Dialer, error) {
	//logger.Debugf(context.Background(), "[DefaultProxyDialer] GetSetting systemProxyDialer NewProxyDialerWithUrl:%s", proxyUrl)
	u, err := url.Parse(proxyUrl)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "socks5", "socks5h":
		return NewSocks5DialerWithUrl(proxyUrl)
	case "vmess":
		return NewVMessDialerWithUrl(proxyUrl)
	case "vless":
		return NewVlessDialerWithUrl(proxyUrl)
	case "trojan":
		return NewTrojanDialerWithUrl(proxyUrl)
	case "http":
		return NewHttpDialerWithUrl(proxyUrl)
	case "https":
		return NewHttpDialerWithUrl(proxyUrl)
	case "ss":
		return NewSSDialerWithUrl(proxyUrl)
	case "ssr":
		return NewSSRDialerWithUrl(proxyUrl)
	case "anytls":
		return NewAnyTLSDialerWithUrl(proxyUrl)
	default:
		return nil, fmt.Errorf("unknown proxy scheme: %s", u.Scheme)
	}
}

var _ proxy.Dialer = (*ProxyChain)(nil)

type ProxyChain struct {
	dialers []Dialer
}

func NewProxyChain(dialers []Dialer) *ProxyChain {
	return &ProxyChain{dialers: dialers}
}

func (p *ProxyChain) Dial(network string, addr string) (c net.Conn, err error) {
	return p.DialContext(context.Background(), network, addr)
}

func (p *ProxyChain) DialContext(ctx context.Context, network string, addr string) (c net.Conn, err error) {
	if len(p.dialers) == 0 {
		return proxy.Direct.DialContext(ctx, network, addr)
	}
	// 将上一个代理设置在当前代理上
	for i := len(p.dialers) - 1; i >= 0; i-- {
		if i != 0 {
			p.dialers[i].SetDialer(p.dialers[i-1])
		}
	}
	return p.dialers[len(p.dialers)-1].Dial(network, addr)
}

// Dialer 代理连接器
type Dialer interface {
	proxy.Dialer
	proxy.ContextDialer
	Addr() string
	Scheme() string
	SetDialer(dialer Dialer)
}

// ProxyHostResolver 可获取代理服务器主机名的接口（非 Dialer 必须实现，用于诊断等场景）
type ProxyHostResolver interface {
	ProxyHost() string
}

// Conn 代理连接
type Conn interface {
	net.Conn
	// Type 连接类型
	Type() string
	// RemoteAddr 目标地址
	RemoteAddr() net.Addr
	// LocalAddr 本地地址
	LocalAddr() net.Addr
	// Status 连接链路状态
	Status() (map[string]ConnStatus, []string)
	NetConn() net.Conn
}

var _ Conn = (*ProxyConn)(nil)

// ProxyConn 适配conn.Conn接口
type ProxyConn struct {
	net.Conn
	cs  ConnStatus
	typ string
}

// UnwrapConn 获取真实的底层连接，跳过 ProxyConn 包装
func UnwrapConn(conn net.Conn) net.Conn {
	if pc, ok := conn.(*ProxyConn); ok {
		return pc.NetConn()
	}
	return conn
}

func (s *ProxyConn) NetConn() net.Conn {
	return s.Conn
}

func (s *ProxyConn) LocalAddr() net.Addr {
	return s.Conn.LocalAddr()
}
func (s *ProxyConn) RemoteAddr() net.Addr {
	return s.Conn.RemoteAddr()
}

func (s *ProxyConn) SetDeadline(t time.Time) error {
	return s.Conn.SetDeadline(t)
}

func (s *ProxyConn) Status() (status map[string]ConnStatus, seq []string) {
	if stat, ok := s.Conn.(interface {
		Status() (map[string]ConnStatus, []string)
	}); ok {
		status, seq = stat.Status()
	}
	if status == nil {
		status = map[string]ConnStatus{}
	}
	status[s.Conn.RemoteAddr().String()] = s.cs
	// 头插
	seq = append([]string{s.Conn.RemoteAddr().String()}, seq...)
	return status, seq
}

func NewProxyConn(conn net.Conn, typ string) *ProxyConn {
	return &ProxyConn{Conn: conn, cs: ConnStatusTimeout, typ: typ}
}

func (s *ProxyConn) SetReadDeadline(t time.Time) error {
	return s.Conn.SetReadDeadline(t)
}

func (s *ProxyConn) SetWriteDeadline(t time.Time) error {
	return s.Conn.SetWriteDeadline(t)
}

func (s *ProxyConn) Close() error {
	err := s.Conn.Close()
	return err
}

func (s *ProxyConn) Read(p []byte) (n int, err error) {
	n, err = s.Conn.Read(p)
	if isRealError(err) {
		s.cs = ConnStatusTimeout
		return
	}
	s.cs = ConnStatusOK
	return
}

func (s *ProxyConn) Write(p []byte) (n int, err error) {
	n, err = s.Conn.Write(p)
	if isRealError(err) {
		s.cs = ConnStatusTimeout
		return 0, err
	}
	s.cs = ConnStatusOK
	return
}

func (s *ProxyConn) Type() string {
	return s.typ
}

func (s *ProxyConn) Target() string {
	return s.Conn.RemoteAddr().String()
}

type ConnStatus string

const (
	ConnStatusTimeout ConnStatus = "timeout"
	ConnStatusOK      ConnStatus = "ok"
)

func isRealError(err error) (ok bool) {
	if err == nil {
		return false
	}
	v := reflect.ValueOf(err)
	if v.Kind() != reflect.Ptr || !v.IsNil() {
		return true
	}
	defer func() {
		// err 是第三方库反回来的时候 可能是type nil 这时候尝试使用is去 检测是否会panic 检测到的时候 必定是nil
		if r := recover(); r != nil {
			ok = false
		}
	}()
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return true
}
