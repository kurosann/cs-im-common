package metric

import (
	"vane-common/module/monitor"
	"vane-common/pkg/hardware"
)

const MetricCPUUsagePercent = "cpu_usage_percent"
const MetricCPUCoreCount = "cpu_core_count"

var _ monitor.IMonitor = (*CPUStatus)(nil)

type CPUStatus struct {
	// metric
	cpuStatus    *monitor.GaugeMetric
	cpuCoreCount *monitor.GaugeMetric
}

func NewCPUStatus() *CPUStatus {
	status_ := &CPUStatus{
		// metric
		cpuStatus:    monitor.NewGaugeMetric(MetricCPUUsagePercent, monitor.LabelValues{}),
		cpuCoreCount: monitor.NewGaugeMetric(MetricCPUCoreCount, monitor.LabelValues{}),
	}

	return status_
}

func (s *CPUStatus) Collect() {
	s.cpuStatus.Set(hardware.CPUUsagePercent())
	s.cpuCoreCount.Set(float64(hardware.CPUCoreCount()))
}

func (s *CPUStatus) Metrics() []*monitor.MetricRegInfo {
	return []*monitor.MetricRegInfo{
		monitor.NewGaugeRegInfo(s.cpuStatus),
		monitor.NewGaugeRegInfo(s.cpuCoreCount),
	}
}
