package hardware

import (
	"testing"
)

func TestMemorySizeInGB(t *testing.T) {
	size := MemorySizeInGB()

	// 内存大小应该大于0，除非获取失败返回-1
	if size != -1 && size <= 0 {
		t.Errorf("内存大小应该大于0，实际得到: %d GB", size)
	}

	// 测试是否能正常获取内存信息
	if size == -1 {
		t.Log("警告: 无法获取内存信息，可能是权限问题或环境限制")
	} else {
		t.Logf("成功获取内存大小: %d GB", size)

		// 现代系统通常至少有1GB内存
		if size < 1 {
			t.Log("警告: 检测到的内存大小小于1GB，请确认是否正确")
		}
	}
}

func TestMemoryUsagePercent(t *testing.T) {
	usage := MemoryUsagePercent()

	// 内存使用率应该在0-100之间，除非获取失败返回-1
	if usage != -1 && (usage < 0 || usage > 100) {
		t.Errorf("内存使用率应该在0-100之间，实际得到: %.2f%%", usage)
	}

	// 测试是否能正常获取内存使用率
	if usage == -1 {
		t.Log("警告: 无法获取内存使用率，可能是权限问题或环境限制")
	} else {
		t.Logf("成功获取内存使用率: %.2f%%", usage)

		// 内存使用率通常不会为0（系统总是使用一些内存）
		if usage == 0 {
			t.Log("警告: 内存使用率为0%，请确认是否正确")
		}

		// 内存使用率通常不会接近100%（系统会保留一些内存）
		if usage > 95 {
			t.Log("警告: 内存使用率超过95%，系统可能内存不足")
		}
	}
}
