package proxies

import (
	"context"
	"net"
	"strings"
	"sync"
	"time"

	doh "github.com/shahradelahi/go-doh-client"

	"vane-common/pkg/safe"
)

// 全局 DNS 解析器
var (
	globalDNSResolver *DNSResolver
	dnsResolverMu     sync.RWMutex
)

// SetGlobalDNSResolver 设置全局 DNS 解析器
func SetGlobalDNSResolver(dnsServers []string) {
	dnsResolverMu.Lock()
	defer dnsResolverMu.Unlock()
	globalDNSResolver = NewDNSResolver(dnsServers)
}

// GetGlobalDNSResolver 获取全局 DNS 解析器
func GetGlobalDNSResolver() *DNSResolver {
	dnsResolverMu.RLock()
	defer dnsResolverMu.RUnlock()
	if globalDNSResolver == nil {
		return NewDNSResolver(nil) // 返回默认解析器
	}
	return globalDNSResolver
}

// DNSResolver DNS 解析器（支持标准 DNS 和 DoH）
type DNSResolver struct {
	dnsServers []string
	resolver   *net.Resolver // 标准 DNS
	dohClient  *doh.DoH      // DoH 客户端
}

// NewDNSResolver 创建 DNS 解析器，自动识别 DoH (https://) 和标准 DNS (IP:port)
func NewDNSResolver(dnsServers []string) *DNSResolver {
	if len(dnsServers) == 0 {
		return &DNSResolver{
			resolver: net.DefaultResolver,
		}
	}

	// 分离 DoH 和标准 DNS 服务器
	var dohURLs []string
	var stdServers []string
	for _, s := range dnsServers {
		if strings.HasPrefix(s, "https://") {
			dohURLs = append(dohURLs, s)
		} else {
			stdServers = append(stdServers, s)
		}
	}

	r := &DNSResolver{
		dnsServers: dnsServers,
	}

	// 配置 DoH 客户端
	if len(dohURLs) > 0 {
		r.dohClient = doh.New(doh.WithProviders(dohURLs), doh.WithTimeout(10*time.Second))
	}

	// 配置标准 DNS 解析器
	if len(stdServers) > 0 {
		dialer := &net.Dialer{
			Timeout: 5 * time.Second,
		}
		r.resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				for _, server := range stdServers {
					conn, err := dialer.DialContext(ctx, network, server)
					if err == nil {
						return conn, nil
					}
				}
				return dialer.DialContext(ctx, network, stdServers[len(stdServers)-1])
			},
		}
	} else if len(dohURLs) > 0 {
		// 只有 DoH，无标准 DNS，resolver 置 nil（走 DoH）
		r.resolver = nil
	}

	return r
}

// ResolveAddr 解析地址，如果解析失败则返回原始地址
func (r *DNSResolver) ResolveAddr(ctx context.Context, addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}

	if net.ParseIP(host) != nil {
		return addr
	}

	// 优先 DoH
	if r.dohClient != nil {
		ip, err := r.lookupIPDoH(ctx, host)
		if err == nil {
			return net.JoinHostPort(ip.String(), port)
		}
	}

	// 回退标准 DNS
	if r.resolver != nil {
		ipAddrs, err := r.resolver.LookupIPAddr(ctx, host)
		if err == nil && len(ipAddrs) > 0 {
			return net.JoinHostPort(ipAddrs[0].IP.String(), port)
		}
	}

	return addr
}

// LookupIP 使用 DNS 解析域名，返回 IP 列表（DoH 优先）
func (r *DNSResolver) LookupIP(ctx context.Context, host string) ([]net.IP, error) {
	// 优先 DoH
	if r.dohClient != nil {
		ips, err := r.lookupIPsDoH(ctx, host)
		if err == nil && len(ips) > 0 {
			return ips, nil
		}
	}

	// 回退标准 DNS
	if r.resolver != nil {
		ipAddrs, err := r.resolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, err
		}
		ips := make([]net.IP, len(ipAddrs))
		for i, addr := range ipAddrs {
			ips[i] = addr.IP
		}
		return ips, nil
	}

	return nil, &net.DNSError{Err: "no DNS resolver available", Name: host}
}

// lookupIPDoH 通过 DoH 查询 A 记录，返回第一个 IPv4
func (r *DNSResolver) lookupIPDoH(ctx context.Context, host string) (net.IP, error) {
	resp, err := r.dohClient.Query(ctx, doh.Domain(host), doh.TypeA)
	if err != nil {
		return nil, err
	}
	for _, a := range resp.Answer {
		if ip := net.ParseIP(a.Data); ip != nil {
			return ip, nil
		}
	}
	return nil, &net.DNSError{Err: "no A record found", Name: host}
}

// lookupIPsDoH 通过 DoH 查询 A + AAAA 记录，返回所有 IP
func (r *DNSResolver) lookupIPsDoH(ctx context.Context, host string) ([]net.IP, error) {
	var ips []net.IP

	// 并发查 A 和 AAAA
	type result struct {
		ips []net.IP
		err error
	}
	ch := make(chan result, 2)

	safe.Go(func() {
		resp, err := r.dohClient.Query(ctx, doh.Domain(host), doh.TypeA)
		if err != nil {
			ch <- result{err: err}
			return
		}
		var list []net.IP
		for _, a := range resp.Answer {
			if ip := net.ParseIP(a.Data); ip != nil {
				list = append(list, ip)
			}
		}
		ch <- result{ips: list}
	})

	safe.Go(func() {
		resp, err := r.dohClient.Query(ctx, doh.Domain(host), doh.TypeAAAA)
		if err != nil {
			ch <- result{err: err}
			return
		}
		var list []net.IP
		for _, a := range resp.Answer {
			if ip := net.ParseIP(a.Data); ip != nil {
				list = append(list, ip)
			}
		}
		ch <- result{ips: list}
	})

	// 收集结果，A 优先
	for i := 0; i < 2; i++ {
		r := <-ch
		if r.err == nil {
			ips = append(ips, r.ips...)
		}
	}

	if len(ips) == 0 {
		return nil, &net.DNSError{Err: "no records found via DoH", Name: host}
	}
	return ips, nil
}
