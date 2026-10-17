package resolver

import (
	"fmt"
	"strings"

	"google.golang.org/grpc/resolver"
)

var _ resolver.Builder = (*DiscoverResolverBuilder)(nil)

type DiscoverResolverBuilder struct {
	currentServiceName     string
	currentServiceEndpoint string
	svcmap                 SvcDiscoverDepend
}

// SvcDiscoverDepend 服务发现依赖
type SvcDiscoverDepend interface {
	GetSvcmap() map[string][]string
	GetTelemetrySvcmap() map[string][]string
}

// eg: {"xxxx.xxx:8080", "xxxx.xxx:8081"}
func NewDiscoverResolverBuilder(serviceName, serviceEndpoint string, svcmap SvcDiscoverDepend) *DiscoverResolverBuilder {
	return &DiscoverResolverBuilder{
		currentServiceName:     serviceName,
		currentServiceEndpoint: serviceEndpoint,
		svcmap:                 svcmap,
	}
}

func (r *DiscoverResolverBuilder) Scheme() string {
	return DiscoverScheme
}

// updateState 提取了根据 svcmap 和 svcName 构建地址列表并更新 ClientConn 状态的逻辑
func (r *DiscoverResolverBuilder) updateState(cc resolver.ClientConn, svcmap SvcDiscoverDepend, svcName, currentServiceName, currentServiceEndpoint string) error {
	if svcName == currentServiceName {
		cc.UpdateState(resolver.State{Addresses: []resolver.Address{
			{Addr: currentServiceEndpoint},
		}})
		return nil
	}
	addrList := svcmap.GetSvcmap()[svcName]
	if len(addrList) == 0 {
		return fmt.Errorf("no address found for service: %s", svcName)
	}
	addresses := make([]resolver.Address, 0, len(addrList))
	for _, a := range addrList {
		host := extractHost(a)
		addresses = append(addresses, resolver.Address{Addr: a, ServerName: host})
	}
	return cc.UpdateState(resolver.State{Endpoints: []resolver.Endpoint{
		{Addresses: addresses},
	}})
}

func (r *DiscoverResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	svcName := getSvcName(target)

	if err := r.updateState(cc, r.svcmap, svcName, r.currentServiceName, r.currentServiceEndpoint); err != nil {
		return nil, err
	}
	return &Resolver{
		cc:                     cc,
		svcmap:                 r.svcmap,
		svcName:                svcName,
		currentServiceName:     r.currentServiceName,
		currentServiceEndpoint: r.currentServiceEndpoint,
		updateState:            r.updateState,
	}, nil
}

func getSvcName(target resolver.Target) string {
	return target.URL.Host
}

var _ resolver.Resolver = (*Resolver)(nil)

type Resolver struct {
	cc                     resolver.ClientConn
	svcmap                 SvcDiscoverDepend
	svcName                string
	currentServiceName     string
	currentServiceEndpoint string
	updateState            func(cc resolver.ClientConn, svcmap SvcDiscoverDepend, svcName, currentServiceName, currentServiceEndpoint string) error
}

func (r *Resolver) ResolveNow(resolver.ResolveNowOptions) {
	_ = r.updateState(r.cc, r.svcmap, r.svcName, r.currentServiceName, r.currentServiceEndpoint)
}

func (r *Resolver) Close() {
}

func extractHost(addr string) string {
	if strings.HasPrefix(addr, "[") {
		endIdx := strings.Index(addr, "]")
		if endIdx != -1 {
			return addr[1:endIdx]
		}
	}

	if colonIdx := strings.LastIndex(addr, ":"); colonIdx != -1 {
		return addr[:colonIdx]
	}

	return addr
}
