package resolver

import (
	"fmt"

	"google.golang.org/grpc/resolver"
)

var _ resolver.Builder = (*DiscoverResolverBuilder)(nil)

type FailoverResolverBuilder struct {
	currentServiceName     string
	currentServiceEndpoint string
	svcmap                 SvcDiscoverDepend
}

// eg: {"xxxx.xxx:8080", "xxxx.xxx:8081"}
func NewFailoverResolverBuilder(serviceName, serviceEndpoint string, svcmap SvcDiscoverDepend) *DiscoverResolverBuilder {
	return &DiscoverResolverBuilder{
		currentServiceName:     serviceName,
		currentServiceEndpoint: serviceEndpoint,
		svcmap:                 svcmap,
	}
}

func (r *FailoverResolverBuilder) Scheme() string {
	return FailOverScheme
}

// updateState 提取了根据 svcmap 和 svcName 构建地址列表并更新 ClientConn 状态的逻辑
func (r *FailoverResolverBuilder) updateState(cc resolver.ClientConn, svcmap SvcDiscoverDepend, svcName, currentServiceName, currentServiceEndpoint string) error {
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
	return cc.UpdateState(resolver.State{Addresses: addresses})
}

func (r *FailoverResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
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
