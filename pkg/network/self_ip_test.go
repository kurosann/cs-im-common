package network

import (
	"context"
	"fmt"
	"testing"
)

func TestGetSelfIPByUDP(t *testing.T) {
	ip, err := GetSelfIP(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("获取自身IP:", ip)
}

func TestGetRealLocalIP(t *testing.T) {
	ip, err := GetRealLocalIP(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("真实本地网卡IP:", ip)
}

func TestGetAllLocalIPs(t *testing.T) {
	ips, err := GetAllLocalIPs(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("所有本地IP地址:", ips)
}

func TestCompareIPs(t *testing.T) {
	// 测试UDP方式和网卡方式获取的IP差异
	udpIP, err1 := GetSelfIP(context.Background())
	realIP, err2 := GetRealLocalIP(context.Background())

	if err1 != nil {
		t.Logf("UDP方式获取IP失败: %v", err1)
	} else {
		fmt.Printf("UDP方式IP: %s\n", udpIP)
	}

	if err2 != nil {
		t.Logf("网卡方式获取IP失败: %v", err2)
	} else {
		fmt.Printf("真实网卡IP: %s\n", realIP)
	}

	if err1 == nil && err2 == nil {
		if udpIP != realIP {
			fmt.Printf("检测到IP差异 - UDP: %s, 网卡: %s (可能开启了VPN)\n", udpIP, realIP)
		} else {
			fmt.Printf("两种方式获取的IP一致: %s\n", udpIP)
		}
	}
}
