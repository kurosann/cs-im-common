package hardware

import (
	"context"
	"math"
	"vane-common/pkg/logger"

	"github.com/shirou/gopsutil/v4/mem"
)

// MemorySizeInGB 获取内存大小（GB）
func MemorySizeInGB() int {
	memory, err := mem.VirtualMemory()
	if err != nil {
		logger.Errorf(context.Background(), "[hardware] mem.VirtualMemory err: %v", err)
		return -1
	}
	size := float64(memory.Total) / 1024 / 1024 / 1024
	return int(math.Ceil(size))
}

// MemoryUsagePercent 获取内存使用率
func MemoryUsagePercent() (usage float64) {
	memory, err := mem.VirtualMemory()
	if err != nil {
		logger.Errorf(context.Background(), "[hardware] mem.VirtualMemory err: %v", err)
		return -1
	}
	return memory.UsedPercent
}
