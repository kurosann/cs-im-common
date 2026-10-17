package network

import (
	"context"
	"net"
	"strings"
)

func GetSelfIP(ctx context.Context) (string, error) {
	ip, err := GetRealLocalIP(ctx)
	if err != nil {
		return "", err
	}
	return ip, nil
}

func GetSelfIPByUDP(ctx context.Context) (string, error) {
	conn, err := net.Dial("udp", "1.1.1.1:53")
	if err != nil {
		return "", err
	}
	defer conn.Close()
	addr := conn.LocalAddr().(*net.UDPAddr)
	return addr.IP.String(), nil
}

func GetRealLocalIP(ctx context.Context) (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		if isVPNInterface(iface.Name) {
			continue
		}

		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addresses {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip != nil && ip.To4() != nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
				return ip.String(), nil
			}
		}
	}

	return "", net.ErrWriteToConnected
}

func isVPNInterface(name string) bool {
	vpnPrefixes := []string{
		"tun", "tap", "utun", "ppp", "wg", "vpn",
		"ipsec", "l2tp", "pptp", "openvpn", "wireguard",
	}

	nameLower := strings.ToLower(name)
	for _, prefix := range vpnPrefixes {
		if strings.HasPrefix(nameLower, prefix) {
			return true
		}
	}

	return false
}

func GetAllLocalIPs(ctx context.Context) ([]string, error) {
	var ips []string
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addresses {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip != nil && ip.To4() != nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
				ips = append(ips, ip.String())
			}
		}
	}

	return ips, nil
}
