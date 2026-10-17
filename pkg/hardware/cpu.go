package hardware

import (
	"context"
	"time"
	"vane-common/pkg/logger"

	"github.com/shirou/gopsutil/v4/cpu"
)

// CPUCoreCount 获取CPU核心数
func CPUCoreCount() (count int) {
	info, err := cpu.Info()
	if err != nil {
		logger.Errorf(context.Background(), "[hardware] cpu.info err: %v", err)
		return -1
	}
	return len(info)
}

// CPUUsagePercent 获取CPU使用率
func CPUUsagePercent() (usage float64) {
	return CPUUsagePercentWithTime(time.Millisecond * 300)
}

// CPUUsagePercent 获取CPU使用率
func CPUUsagePercentWithTime(interval time.Duration) (usage float64) {
	info, err := cpu.Percent(interval, false)
	if err != nil {
		logger.Errorf(context.Background(), "[hardware] cpu.Percent err: %v", err)
		return -1
	}
	return info[0]
}
