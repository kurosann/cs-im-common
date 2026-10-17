package hardware

import (
	"context"
	"fmt"
	"vane-common/pkg/logger"

	"github.com/shirou/gopsutil/v4/net"
)

// NetInfo 网络信息结构体
type NetInfo struct {
	BytesSent uint64
	BytesRecv uint64
}

// NetSentRecv 获取网络发送接收字节数
func NetSentRecv() map[string]NetInfo {
	info, err := net.IOCounters(true)
	if err != nil {
		logger.Errorf(context.Background(), "[hardware] net.IOCounters err: %v", err)
		return nil
	}

	res := make(map[string]NetInfo)
	for _, v := range info {
		res[v.Name] = NetInfo{
			BytesSent: v.BytesSent,
			BytesRecv: v.BytesRecv,
		}
	}

	return res
}

type NetStats struct {
	ListenMap      map[string]int
	EstablishedMap map[string]int
	StateCount     map[string]int
	Interfaces     []string
}

func NetworkStats() (*NetStats, error) {
	netStats := &NetStats{
		ListenMap:      make(map[string]int),
		EstablishedMap: make(map[string]int),
		StateCount:     make(map[string]int),
		Interfaces:     make([]string, 0),
	}

	// 获取网络连接统计
	connections, err := net.Connections("tcp")
	if err != nil {
		logger.Errorf(context.Background(), "[hardware] net.Connections err: %v", err)
		return nil, err
	}

	// 统计 LISTEN Map key 是本地地址，value 是连接数
	for _, conn := range connections {
		if conn.Status == "LISTEN" {
			netStats.ListenMap[fmt.Sprintf("%s:%v", conn.Laddr.IP, conn.Laddr.Port)]++
		}
	}

	// 统计 ESTABLISHED Map key 是远程地址，value 是连接数
	for _, conn := range connections {
		if conn.Status == "ESTABLISHED" {
			netStats.EstablishedMap[fmt.Sprintf("%s:%v", conn.Raddr.IP, conn.Raddr.Port)]++
		}
	}

	// 统计不同状态的连接
	for _, conn := range connections {
		netStats.StateCount[conn.Status]++
	}

	// 获取网络接口统计
	interfaces, err := net.Interfaces()
	if err != nil {
		logger.Errorf(context.Background(), "[hardware] net.Interfaces err: %v", err)
		return nil, err
	}

	for _, iface := range interfaces {
		ifaceStatus := fmt.Sprintf("interface %s: status %v", iface.Name, iface.Flags)
		netStats.Interfaces = append(netStats.Interfaces, ifaceStatus)
	}

	return netStats, nil
}
