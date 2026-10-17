package ws

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/proxy"
	"golang.org/x/net/websocket"
)

type Dialer struct {
	Conn    net.Conn
	Tls     *tls.Config
	Header  http.Header
	MsgType MsgType

	Marshal func(any) ([]byte, error)
}

func (d *Dialer) Dial(ctx context.Context, address string) (*Conn, error) {
	if d.Header == nil {
		d.Header = http.Header{}
	}
	if d.Tls == nil {
		d.Tls = &tls.Config{}
	}
	if d.Marshal == nil {
		d.Marshal = json.Marshal
	}
	return d.newConn(ctx, address, d.Header, d.Tls)
}

type MsgType string

const (
	MsgTypeText   MsgType = "Text"
	MsgTypeBinary MsgType = "Binary"
)

type Conn struct {
	MsgType MsgType // text or binary (default: text)
	Marshal func(any) ([]byte, error)

	conn   *websocket.Conn
	status string
}

var _ proxy.Dialer = &proxyDialer{}

type proxyDialer struct {
	conn net.Conn
}

func (d *proxyDialer) Dial(network, addr string) (net.Conn, error) {
	return d.conn, nil
}

func (d *Dialer) newConn(ctx context.Context, address string, header http.Header, tlsConfig *tls.Config) (*Conn, error) {
	config, err := websocket.NewConfig(address, "http://localhost")
	if err != nil {
		return nil, err
	}
	for k, v := range header {
		config.Header.Set(k, v[0])
	}
	config.Header.Set("Sec-WebSocket-Extensions", "permessage-deflate; client_max_window_bits")
	config.TlsConfig = tlsConfig

	requestURI, err := url.ParseRequestURI(address)
	if err != nil {
		return nil, err
	}
	// 自动补全端口
	dialAddr := requestURI.Host
	if requestURI.Port() == "" {
		switch requestURI.Scheme {
		case "wss":
			dialAddr = net.JoinHostPort(requestURI.Hostname(), "443")
		case "ws":
			dialAddr = net.JoinHostPort(requestURI.Hostname(), "80")
		}
	}
	var conn net.Conn
	if d.Conn == nil {
		proxyDialer := proxy.FromEnvironment()
		if proxyCtxDialer, ok := proxyDialer.(proxy.ContextDialer); ok {
			conn, err = proxyCtxDialer.DialContext(ctx, "tcp", dialAddr)
		} else {
			conn, err = proxyDialer.Dial("tcp", dialAddr)
		}
		if err != nil {
			return nil, err
		}
	} else {
		// 使用已有的连接
		conn = d.Conn
	}
	switch requestURI.Scheme {
	case "wss":
		tlsConn := tls.Client(conn, config.TlsConfig)
		if err := tlsConn.Handshake(); err != nil {
			return nil, err
		}
		conn = tlsConn
	}

	wsconn, err := websocket.NewClient(config, conn)
	if err != nil {
		return nil, err
	}
	return &Conn{conn: wsconn, status: "alive", MsgType: d.MsgType, Marshal: d.Marshal}, nil
}

func (c *Conn) Send(message any) (err error) {
	var msg any
	switch c.MsgType {
	case MsgTypeText, "":
		msg, err = transferText[string](message, c.Marshal)
	case MsgTypeBinary:
		msg, err = transferText[[]byte](message, c.Marshal)
	default:
		return fmt.Errorf("unsupported message type: %s", c.MsgType)
	}
	if err != nil {
		return err
	}
	if err = websocket.Message.Send(c.conn, msg); err != nil {
		c.status = "dead"
		return err
	}
	c.status = "alive"
	return nil
}

func transferText[T string | []byte](message any, marshal func(any) ([]byte, error)) (T, error) {
	switch v := message.(type) {
	case string:
		return T(v), nil
	case []byte:
		return T(v), nil
	default:
		var t T
		msg, err := marshal(v)
		if err != nil {
			return t, err
		}
		return T(msg), nil
	}
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}

func (c *Conn) Read() (string, error) {
	var message string
	err := websocket.Message.Receive(c.conn, &message)
	if err != nil {
		c.status = "dead"
		return "", err
	}
	c.status = "alive"
	return message, nil
}

func (c *Conn) IsAlive() bool {
	return c.status == "alive"
}

func (c *Conn) Close() error {
	c.status = "dead"
	return c.conn.Close()
}
