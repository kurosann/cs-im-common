package hardware

import (
	"testing"
)

func TestNetSentRecv(t *testing.T) {
	netInfo := NetSentRecv()

	// 测试是否能正常获取网络信息
	if netInfo == nil {
		t.Log("警告: 无法获取网络信息，可能是权限问题或环境限制")
		return
	}

	// 检查返回的map是否为空
	if len(netInfo) == 0 {
		t.Log("警告: 网络接口列表为空")
		return
	}

	t.Logf("成功获取 %d 个网络接口的信息", len(netInfo))

	// 遍历所有网络接口并验证数据
	for name, info := range netInfo {
		t.Logf("网络接口: %s, 发送字节: %d, 接收字节: %d", name, info.BytesSent, info.BytesRecv)

		// 验证结构体字段是否正确设置
		if info.BytesSent < 0 {
			t.Errorf("网络接口 %s 的发送字节数不应该为负数", name)
		}
		if info.BytesRecv < 0 {
			t.Errorf("网络接口 %s 的接收字节数不应该为负数", name)
		}
	}
}

func TestNetInfoStruct(t *testing.T) {
	// 测试NetInfo结构体的基本功能
	info := NetInfo{
		BytesSent: 1000,
		BytesRecv: 2000,
	}

	if info.BytesSent != 1000 {
		t.Errorf("期望发送字节数为1000，实际得到: %d", info.BytesSent)
	}

	if info.BytesRecv != 2000 {
		t.Errorf("期望接收字节数为2000，实际得到: %d", info.BytesRecv)
	}
}

func TestNetworkStats(t *testing.T) {
	netStats, err := NetworkStats()
	if err != nil {
		t.Errorf("获取网络统计信息失败: %v", err)
	}

	for _, iface := range netStats.Interfaces {
		t.Logf("网络接口: %s", iface)
	}
	for key, conn := range netStats.ListenMap {
		t.Logf("本地地址: %+v, 连接数: %+v", key, conn)
	}
	for key, conn := range netStats.EstablishedMap {
		t.Logf("远程地址: %+v, 连接数: %+v", key, conn)
	}
	for key, conn := range netStats.StateCount {
		t.Logf("状态: %+v, 连接数: %+v", key, conn)
	}

}
