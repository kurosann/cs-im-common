package resolver

const (
	DnsScheme      = "dns"
	DiscoverScheme = "discover"
	FailOverScheme = "failover"
)

func BuildDnsTarget(dns string) string {
	return DnsScheme + "://" + dns
}

func BuildDiscoverTarget(svcName string) string {
	return DiscoverScheme + "://" + svcName
}

func BuildFailOverTarget(svcName string) string {
	return FailOverScheme + "://" + svcName
}
