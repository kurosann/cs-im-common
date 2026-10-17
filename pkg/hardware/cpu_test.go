package hardware

import (
	"testing"
	"time"
)

func TestCPUCoreCount(t *testing.T) {
	count := CPUCoreCount()

	// CPU核心数应该大于0，除非获取失败返回-1
	if count != -1 && count <= 0 {
		t.Errorf("CPU核心数应该大于0，实际得到: %d", count)
	}

	// 测试是否能正常获取CPU信息
	if count == -1 {
		t.Log("警告: 无法获取CPU信息，可能是权限问题或环境限制")
	} else {
		t.Logf("成功获取CPU核心数: %d", count)
	}
}

func TestCPUUsagePercent(t *testing.T) {
	usage := CPUUsagePercent()

	// CPU使用率应该在0-100之间，除非获取失败返回-1
	if usage != -1 && (usage < 0 || usage > 100) {
		t.Errorf("CPU使用率应该在0-100之间，实际得到: %.2f%%", usage)
	}

	// 测试是否能正常获取CPU使用率
	if usage == -1 {
		t.Log("警告: 无法获取CPU使用率，可能是权限问题或环境限制")
	} else {
		t.Logf("成功获取CPU使用率: %.2f%%", usage)
	}
}

func TestCPUUsagePercentWithTime(t *testing.T) {
	// 测试不同的时间间隔
	testCases := []time.Duration{
		time.Millisecond * 100,
		time.Millisecond * 500,
		time.Second * 1,
	}

	for _, interval := range testCases {
		t.Run(interval.String(), func(t *testing.T) {
			usage := CPUUsagePercentWithTime(interval)

			// CPU使用率应该在0-100之间，除非获取失败返回-1
			if usage != -1 && (usage < 0 || usage > 100) {
				t.Errorf("CPU使用率应该在0-100之间，实际得到: %.2f%%", usage)
			}

			// 测试是否能正常获取CPU使用率
			if usage == -1 {
				t.Logf("警告: 无法获取CPU使用率(间隔: %s)，可能是权限问题或环境限制", interval)
			} else {
				t.Logf("成功获取CPU使用率(间隔: %s): %.2f%%", interval, usage)
			}
		})
	}
}
