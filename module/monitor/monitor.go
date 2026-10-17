package monitor

import "time"

type LabelValues map[string]string

type IMonitor interface {
	Collect()
	Metrics() []*MetricRegInfo
}
type MonitorRegInfo struct {
	Name    string
	Spec    string
	Monitor IMonitor
}

func NewMonitorRegInfo(name string, moniter IMonitor, options ...MonitorRegInfoOption) MonitorRegInfo {
	info := MonitorRegInfo{
		Name:    name,
		Spec:    "@every 10s",
		Monitor: moniter,
	}
	for _, option := range options {
		option.apply(&info)
	}
	return info
}

type MonitorRegInfoOption interface {
	apply(info *MonitorRegInfo)
}

type monitorRegInfoSpecOption struct {
	spec string
}

func (o *monitorRegInfoSpecOption) apply(info *MonitorRegInfo) {
	info.Spec = o.spec
}

// WithSpec 设置监控项的执行周期
// 默认值为@every 30s
func WithSpec(spec string) MonitorRegInfoOption {
	return &monitorRegInfoSpecOption{spec: spec}
}

type MetricRegInfo struct {
	Collector Metric
}

var _ Metric = (*GaugeMetric)(nil)

type GaugeMetric struct {
	Name        string
	LabelValues LabelValues
	value       float64
}

func (g *GaugeMetric) GetLabelValues() LabelValues {
	return g.LabelValues
}

func (g *GaugeMetric) GetName() string {
	return g.Name
}

func NewGaugeMetric(name string, labelValues LabelValues) *GaugeMetric {
	return &GaugeMetric{
		Name:        name,
		LabelValues: labelValues,
	}
}

func (g *GaugeMetric) GetValue() float64 {
	return g.value
}

func (g *GaugeMetric) WithLabelValues(labelValues LabelValues) *GaugeMetric {
	g.LabelValues = labelValues
	return g
}

func (g *GaugeMetric) Set(value float64) {
	g.value = value
}

var _ Metric = (*CounterMetric)(nil)

type CounterMetric struct {
	Name        string
	LabelValues LabelValues
	count       float64
	incFunc     func(LabelValues)
}

func (c *CounterMetric) GetLabelValues() LabelValues {
	return c.LabelValues
}

func (c *CounterMetric) WithLabelValues(labelValues LabelValues) *CounterMetric {
	c.LabelValues = labelValues
	return c
}

func (c *CounterMetric) GetName() string {
	return c.Name
}

func (c *CounterMetric) Inc() {
	c.count++
	if c.incFunc != nil {
		c.incFunc(c.LabelValues)
	}
}

func (c *CounterMetric) SetIncFunc(incFunc func(LabelValues)) {
	c.incFunc = incFunc
}

func (c *CounterMetric) GetValue() float64 {
	defer func() { c.count = 0 }()
	return c.count
}

var _ Metric = (*HistogramMetric)(nil)

type HistogramMetric struct {
	Name        string
	LabelValues LabelValues
	Buckets     []float64
	value       float64
}

func (h *HistogramMetric) GetLabelValues() LabelValues {
	return h.LabelValues
}

func (h *HistogramMetric) WithLabelValues(labelValues LabelValues) *HistogramMetric {
	h.LabelValues = labelValues
	return h
}

func (h *HistogramMetric) GetName() string {
	return h.Name
}

func (h *HistogramMetric) GetValue() float64 {
	return h.value
}

var _ Metric = (*SummaryMetric)(nil)

type SummaryMetric struct {
	Name        string
	LabelValues LabelValues
	MaxAge      time.Duration
	AgeBuckets  uint32
	value       float64
}

func (s *SummaryMetric) GetLabelValues() LabelValues {
	return s.LabelValues
}

func (s *SummaryMetric) WithLabelValues(labelValues LabelValues) *SummaryMetric {
	s.LabelValues = labelValues
	return s
}

func (s *SummaryMetric) GetName() string {
	return s.Name
}

func (s *SummaryMetric) GetValue() float64 {
	return s.value
}

type Metric interface {
	GetValue() float64
	GetName() string
	GetLabelValues() LabelValues
}

type MetricT interface {
	GaugeMetric | CounterMetric | HistogramMetric | SummaryMetric
}

func NewGaugeRegInfo[T MetricT](collector *T) *MetricRegInfo {
	switch collector := any(collector).(type) {
	case Metric:
		info := &MetricRegInfo{
			Collector: collector,
		}
		return info
	default:
		panic("invalid metric type")
	}
}

type HealthChecker interface {
	IsOk() (ok bool, info map[string]any)
}

type HealthCheckerFunc func() (bool, map[string]any)

func (h HealthCheckerFunc) IsOk() (ok bool, info map[string]any) {
	return h()
}

type Revertable interface {
	Revert() error
}

type RevertableFunc func() error

func (r RevertableFunc) Revert() error {
	return r()
}

type HealthCheckerAndRevertable interface {
	HealthChecker
	Revertable
}

type HealthCheckerAndRevertableFunc struct {
	HealthCheckerFunc
	RevertableFunc
}

type HealthCheckerMetric struct {
	Metric
	HealthChecker
}

type HealthCheckerAndRevertableMetric struct {
	Metric
	HealthCheckerAndRevertable
}

func (h *HealthCheckerMetric) IsOk() (ok bool, info map[string]any) {
	return h.HealthChecker.IsOk()
}

func NewHealthCheckerMetric[T MetricT](collector *T, h HealthChecker) *MetricRegInfo {
	switch collector := any(collector).(type) {
	case Metric:
		info := &MetricRegInfo{
			Collector: &HealthCheckerMetric{
				Metric:        collector,
				HealthChecker: h,
			},
		}
		return info
	default:
		panic("invalid metric type")
	}
}

func NewHealthCheckerAndRevertableMetric[T MetricT](collector *T, hr HealthCheckerAndRevertable) *MetricRegInfo {
	switch collector := any(collector).(type) {
	case Metric:
		info := &MetricRegInfo{
			Collector: &HealthCheckerAndRevertableMetric{
				Metric:                     collector,
				HealthCheckerAndRevertable: hr,
			},
		}
		return info
	default:
		panic("invalid metric type")
	}
}
