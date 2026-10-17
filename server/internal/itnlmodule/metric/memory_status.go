package metric

import (
	"vane-common/module/monitor"
	"vane-common/pkg/hardware"
)

const MetricMemoryUsagePercent = "memory_usage_percent"
const MetricMemorySizeGB = "memory_size_gb"

var _ monitor.IMonitor = (*MemoryStatus)(nil)

type MemoryStatus struct {
	// metric
	memoryStatus *monitor.GaugeMetric
	memorySizeGB *monitor.GaugeMetric
}

func NewMemoryStatus() *MemoryStatus {
	status_ := &MemoryStatus{
		// metric
		memoryStatus: monitor.NewGaugeMetric(MetricMemoryUsagePercent, monitor.LabelValues{}),
		memorySizeGB: monitor.NewGaugeMetric(MetricMemorySizeGB, monitor.LabelValues{}),
	}

	return status_
}

func (s *MemoryStatus) Collect() {
	s.memoryStatus.Set(hardware.MemoryUsagePercent())
	s.memorySizeGB.Set(float64(hardware.MemorySizeInGB()))
}

func (s *MemoryStatus) Metrics() []*monitor.MetricRegInfo {
	return []*monitor.MetricRegInfo{
		monitor.NewGaugeRegInfo(s.memoryStatus),
		monitor.NewGaugeRegInfo(s.memorySizeGB),
	}
}
