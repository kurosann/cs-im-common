package trpc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// ClientTLSConfig mTLS 客户端配置（文件路径方式）
type ClientTLSConfig struct {
	// CertFile 客户端证书文件路径
	CertFile string
	// KeyFile 客户端私钥文件路径
	KeyFile string
	// CAFile 用于验证服务端的 CA 证书文件路径
	CAFile string
	// ServerName 服务端名称，用于 TLS 验证（可选，用于覆盖证书中的 CN）
	ServerName string
	// InsecureSkipVerify 跳过服务端证书验证（仅用于测试，生产环境勿用）
	InsecureSkipVerify bool
}

// ClientTLSPEMConfig mTLS 客户端配置（PEM 文本方式）
type ClientTLSPEMConfig struct {
	// CertPEM 客户端证书 PEM 文本
	CertPEM []byte
	// KeyPEM 客户端私钥 PEM 文本
	KeyPEM []byte
	// CAPEM 用于验证服务端的 CA 证书 PEM 文本
	CAPEM []byte
	// ServerName 服务端名称，用于 TLS 验证（可选，用于覆盖证书中的 CN）
	ServerName string
	// InsecureSkipVerify 跳过服务端证书验证（仅用于测试，生产环境勿用）
	InsecureSkipVerify bool
}

// ServerTLSConfig mTLS 服务端配置（文件路径方式）
type ServerTLSConfig struct {
	// CertFile 服务端证书文件路径
	CertFile string
	// KeyFile 服务端私钥文件路径
	KeyFile string
	// CAFile 用于验证客户端证书的 CA 证书文件路径（mTLS 必需）
	CAFile string
}

// ServerTLSPEMConfig mTLS 服务端配置（PEM 文本方式）
type ServerTLSPEMConfig struct {
	// CertPEM 服务端证书 PEM 文本
	CertPEM []byte
	// KeyPEM 服务端私钥 PEM 文本
	KeyPEM []byte
	// CAPEM 用于验证客户端证书的 CA 证书 PEM 文本
	CAPEM []byte
}

// NewClientCredentials 根据文件路径配置创建客户端 TLS credentials，用于 mTLS
func NewClientCredentials(cfg *ClientTLSConfig) (credentials.TransportCredentials, error) {
	tlsConfig, err := loadClientTLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	return credentials.NewTLS(tlsConfig), nil
}

// NewClientCredentialsFromPEM 根据 PEM 文本配置创建客户端 TLS credentials，用于 mTLS
func NewClientCredentialsFromPEM(cfg *ClientTLSPEMConfig) (credentials.TransportCredentials, error) {
	tlsConfig, err := loadClientTLSPEMConfig(cfg)
	if err != nil {
		return nil, err
	}
	return credentials.NewTLS(tlsConfig), nil
}

// NewServerCredentials 根据文件路径配置创建服务端 TLS credentials，用于 mTLS
func NewServerCredentials(cfg *ServerTLSConfig) (credentials.TransportCredentials, error) {
	tlsConfig, err := loadServerTLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	return credentials.NewTLS(tlsConfig), nil
}

// NewServerCredentialsFromPEM 根据 PEM 文本配置创建服务端 TLS credentials，用于 mTLS
func NewServerCredentialsFromPEM(cfg *ServerTLSPEMConfig) (credentials.TransportCredentials, error) {
	tlsConfig, err := loadServerTLSPEMConfig(cfg)
	if err != nil {
		return nil, err
	}
	return credentials.NewTLS(tlsConfig), nil
}

// 使用文件地址配置client证书
func InitClientMTLSForHost(cfg *ClientTLSConfig) error {
	creds, err := NewClientCredentials(cfg)
	if err != nil {
		return err
	}
	globalClientPool.SetClientCreds(cfg.ServerName, creds)
	globalSvcClientPool.SetClientCreds(cfg.ServerName, creds)
	return nil
}

// 使用raw文本配置client证书
func SetClientMTLSForHostFromPEM(cfg *ClientTLSPEMConfig) error {
	creds, err := NewClientCredentialsFromPEM(cfg)
	if err != nil {
		return err
	}
	globalClientPool.SetClientCreds(cfg.ServerName, creds)
	globalSvcClientPool.SetClientCreds(cfg.ServerName, creds)
	return nil
}

func SetServerMTLS(cfg *ServerTLSConfig) error {
	creds, err := NewServerCredentials(cfg)
	if err != nil {
		return err
	}
	SetServerCreds(creds)
	return nil
}

func SetServerMTLSFromPEM(cfg *ServerTLSPEMConfig) error {
	creds, err := NewServerCredentialsFromPEM(cfg)
	if err != nil {
		return err
	}
	SetServerCreds(creds)
	return nil
}

func ServerCreds(creds credentials.TransportCredentials) grpc.ServerOption {
	return grpc.Creds(creds)
}

func loadClientTLSConfig(cfg *ClientTLSConfig) (*tls.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("client tls config is nil")
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if cfg.CertFile != "" && cfg.KeyFile != "" {
		cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load client cert: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if cfg.CAFile != "" {
		caCert, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA cert")
		}
		tlsConfig.RootCAs = caPool
	}

	if cfg.ServerName != "" {
		tlsConfig.ServerName = cfg.ServerName
	}

	if cfg.InsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	}

	return tlsConfig, nil
}

func loadClientTLSPEMConfig(cfg *ClientTLSPEMConfig) (*tls.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("client tls PEM config is nil")
	}

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if len(cfg.CertPEM) > 0 && len(cfg.KeyPEM) > 0 {
		cert, err := tls.X509KeyPair(cfg.CertPEM, cfg.KeyPEM)
		if err != nil {
			return nil, fmt.Errorf("load client cert from PEM: %w", err)
		}
		tlsConfig.Certificates = []tls.Certificate{cert}
	}

	if len(cfg.CAPEM) > 0 {
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(cfg.CAPEM) {
			return nil, fmt.Errorf("failed to append CA cert from PEM")
		}
		tlsConfig.RootCAs = caPool
	}

	if cfg.ServerName != "" {
		tlsConfig.ServerName = cfg.ServerName
	}

	if cfg.InsecureSkipVerify {
		tlsConfig.InsecureSkipVerify = true
	}

	return tlsConfig, nil
}

func loadServerTLSConfig(cfg *ServerTLSConfig) (*tls.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("server tls config is nil")
	}

	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("load server cert: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	if cfg.CAFile != "" {
		caCert, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to append CA cert")
		}
		tlsConfig.ClientCAs = caPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsConfig, nil
}

func loadServerTLSPEMConfig(cfg *ServerTLSPEMConfig) (*tls.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("server tls PEM config is nil")
	}

	cert, err := tls.X509KeyPair(cfg.CertPEM, cfg.KeyPEM)
	if err != nil {
		return nil, fmt.Errorf("load server cert from PEM: %w", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}

	if len(cfg.CAPEM) > 0 {
		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM(cfg.CAPEM) {
			return nil, fmt.Errorf("failed to append CA cert from PEM")
		}
		tlsConfig.ClientCAs = caPool
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	}

	return tlsConfig, nil
}
