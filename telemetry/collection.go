package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"sync"
	"time"

	"vane-common/module/monitor"
	"vane-common/pkg/logger"
	"vane-common/pkg/safe"

	gosundheit "github.com/AppsFlyer/go-sundheit"
	healthhttp "github.com/AppsFlyer/go-sundheit/http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const DefaultMaxGoroutine = 10

var (
	CC   *CollectCenter
	once sync.Once
)

type Hook struct {
	MonitorName string
	Spec        string
	Func        func()
}

type CollectCenter struct {
	gauge     map[string]*Gauge
	counter   map[string]*Counter
	histogram map[string]*Histogram
	summary   map[string]*Summary

	mCollector map[string][]monitor.Metric
	mSpec      map[string]string

	rw           sync.RWMutex
	maxGoroutine int

	hooks     map[string]Hook
	hooksLock sync.RWMutex

	recoverLock sync.RWMutex
	recovering  map[string]bool
	recoverErr  map[string]error

	healthChecker gosundheit.Health
	revertCh      chan Revertable
}

type Revertable struct {
	Name string
	monitor.Revertable
}

func InitCollectCenter(ctx context.Context, maxGoroutine int) *CollectCenter {
	once.Do(func() {
		recovering := make(map[string]bool)
		recoverErr := make(map[string]error)
		revertCh := make(chan Revertable, 10)
		safe.GoKeep(ctx, 0, func() {
			for {
				select {
				case <-ctx.Done():
					return
				case revert := <-revertCh:
					logger.Infof(ctx, "health check start revert: %s", revert.Name)
					if err := safe.TryE(revert.Revert); err != nil {
						logger.Errorf(context.Background(), "revert failed: %v", err)
						recoverErr[revert.Name] = err
					}
					recovering[revert.Name] = false
				}
			}
		})
		CC = &CollectCenter{
			gauge:         make(map[string]*Gauge),
			counter:       make(map[string]*Counter),
			histogram:     make(map[string]*Histogram),
			summary:       make(map[string]*Summary),
			mCollector:    make(map[string][]monitor.Metric),
			mSpec:         make(map[string]string),
			maxGoroutine:  maxGoroutine,
			hooks:         make(map[string]Hook),
			healthChecker: NewNoScheduleHealthChecker(),
			revertCh:      revertCh,
			recovering:    recovering,
			recoverErr:    recoverErr,
		}
	})
	return CC
}

func (r *CollectCenter) RegisterHook(name string, spec string, f func()) {
	r.hooksLock.Lock()
	defer r.hooksLock.Unlock()
	if _, ok := r.hooks[name]; ok {
		panic(fmt.Sprintf("monitor hook [%s] already registered", name))
	}
	r.hooks[name] = Hook{
		MonitorName: name,
		Spec:        spec,
		Func:        f,
	}
}

func (r *CollectCenter) RemoveHook(name string) {
	r.hooksLock.Lock()
	defer r.hooksLock.Unlock()
	delete(r.hooks, name)
}

func (r *CollectCenter) Register(monitorName string, c monitor.Metric) (string, error) {
	if reflect.ValueOf(c).IsNil() {
		return "", fmt.Errorf("metric is nil")
	}
	switch collector := c.(type) {
	case *monitor.GaugeMetric:
		r.rw.Lock()
		defer r.rw.Unlock()
		if err := r.checkCollectorExist(collector.Name); err != nil {
			return "", fmt.Errorf("gauge register: %w", err)
		}
		r.gauge[collector.Name] = NewGauge(collector.Name, getLabelKeys(collector.LabelValues))
		r.mCollector[monitorName] = append(r.mCollector[monitorName], collector)
		return collector.Name, nil
	case *monitor.CounterMetric:
		r.rw.Lock()
		defer r.rw.Unlock()
		if err := r.checkCollectorExist(collector.Name); err != nil {
			return "", fmt.Errorf("counter register: %w", err)
		}
		counter := NewCounter(collector.Name, getLabelKeys(collector.LabelValues))
		collector.SetIncFunc(func(labelValues monitor.LabelValues) {
			counter.WithLabelValues(labelValues).Inc()
		})
		r.counter[collector.Name] = counter
		r.mCollector[monitorName] = append(r.mCollector[monitorName], collector)
		return collector.Name, nil
	case *monitor.HistogramMetric:
		r.rw.Lock()
		defer r.rw.Unlock()
		if err := r.checkCollectorExist(collector.Name); err != nil {
			return "", fmt.Errorf("histogram register: %w", err)
		}
		r.histogram[collector.Name] = NewHistogram(collector.Name, getLabelKeys(collector.LabelValues), collector.Buckets)
		r.mCollector[monitorName] = append(r.mCollector[monitorName], collector)
		return collector.Name, nil
	case *monitor.SummaryMetric:
		r.rw.Lock()
		defer r.rw.Unlock()
		if err := r.checkCollectorExist(collector.Name); err != nil {
			return "", fmt.Errorf("summary register: %w", err)
		}
		r.summary[collector.Name] = NewSummary(collector.Name, getLabelKeys(collector.LabelValues), collector.MaxAge, collector.AgeBuckets)
		r.mCollector[monitorName] = append(r.mCollector[monitorName], collector)
		return collector.Name, nil
	case *monitor.HealthCheckerMetric:
		name, err := r.Register(monitorName, collector.Metric)
		if err != nil {
			return "", fmt.Errorf("health checker register: %w", err)
		}
		r.rw.Lock()
		defer r.rw.Unlock()

		r.mCollector[name] = append(r.mCollector[name], collector)
		if err := r.healthChecker.RegisterCheck(NewHealthChecker(name, HealthCheckerFunc(func(ctx context.Context) (CheckerItem, error) {
			ok, info := r.checkHealthDataRevert(collector)
			if !ok {
				return info, fmt.Errorf("health check failed")
			}
			return info, nil
		})), gosundheit.ExecutionPeriod(time.Second*10)); err != nil {
			return "", fmt.Errorf("register health checker failed: %w", err)
		}
		return name, nil
	case *monitor.HealthCheckerAndRevertableMetric:
		name, err := r.Register(monitorName, collector.Metric)
		if err != nil {
			return "", fmt.Errorf("health checker and recoverable register: %w", err)
		}
		r.rw.Lock()
		defer r.rw.Unlock()

		r.mCollector[name] = append(r.mCollector[name], collector)
		if err := r.healthChecker.RegisterCheck(NewHealthChecker(name, HealthCheckerFunc(func(ctx context.Context) (CheckerItem, error) {
			ok, info := r.checkHealthDataRevert(collector)
			if !ok {
				return info, fmt.Errorf("health check failed")
			}
			return info, nil
		}))); err != nil {
			return "", fmt.Errorf("register health checker failed: %w", err)
		}
		return name, nil
	}
	return "", fmt.Errorf("unknown metric type: %T", c)
}
func (r *CollectCenter) HealthHandlerFunc() http.Handler {
	handler := healthhttp.HandleHealthJSON(r.healthChecker)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Query().Get("now") == "1" {
			ctx, cancel := context.WithTimeout(req.Context(), 3*time.Second)
			defer cancel()
			CC.CollectData(ctx)
		}

		handler.ServeHTTP(w, req)
	})
}
func (r *CollectCenter) MetricsHandlerFunc() http.Handler {
	handler := promhttp.InstrumentMetricHandler(prometheus.DefaultRegisterer,
		promhttp.HandlerFor(&globalGatherer{gatherer: prometheus.DefaultGatherer},
			promhttp.HandlerOpts{}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("now") == "1" {
			ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
			defer cancel()
			CC.CollectData(ctx)
			CC.CheckHealthAndRevert()
		}

		handler.ServeHTTP(w, r)
	})
}
func (r *CollectCenter) checkCollectorExist(name string) error {
	for _, collector := range r.mCollector[name] {
		if collector.GetName() == name {
			return fmt.Errorf("collector already registered")
		}
	}
	return nil
}

func getLabelKeys(labelValues monitor.LabelValues) []string {
	keys := make([]string, 0, len(labelValues))
	for k := range labelValues {
		keys = append(keys, k)
	}
	return keys
}

func (r *CollectCenter) CollectData(ctx context.Context) {
	r.rw.RLock()
	defer r.rw.RUnlock()
	wg := safe.NewPoolContext(ctx, r.maxGoroutine)
	r.hooksLock.RLock()
	defer r.hooksLock.RUnlock()
	for _, hook := range r.hooks {
		wg.Go(hook.Func)
	}
	for _, worker := range r.CollectDataWorkers() {
		wg.Go(worker.Worker)
	}
	wg.WaitAndRecover()
}

type CollectDataWorker struct {
	MonitorName string
	Spec        string
	Worker      func()
}

func (r *CollectCenter) CollectDataWorkers() []CollectDataWorker {
	r.rw.RLock()
	defer r.rw.RUnlock()
	workers := make([]CollectDataWorker, 0, len(r.hooks))

	for _, hook := range r.hooks {
		workers = append(workers, CollectDataWorker{
			MonitorName: hook.MonitorName,
			Spec:        hook.Spec,
			Worker: func() {
				hook.Func()
				r.collectSetData(hook.MonitorName)
			},
		})
	}
	return workers
}

func (r *CollectCenter) collectSetData(monitorName string) {
	r.rw.RLock()
	defer r.rw.RUnlock()
	collectors := r.mCollector[monitorName]
	for _, collector := range collectors {
		switch collector := collector.(type) {
		case *monitor.GaugeMetric:
			r.gauge[collector.GetName()].WithLabelValues(collector.LabelValues).Set(collector.GetValue())
		case *monitor.CounterMetric:
			r.counter[collector.GetName()].WithLabelValues(collector.LabelValues).Inc()
		case *monitor.HistogramMetric:
			r.histogram[collector.GetName()].WithLabelValues(collector.LabelValues).Observe(collector.GetValue())
		case *monitor.SummaryMetric:
			r.summary[collector.GetName()].WithLabelValues(collector.LabelValues).Observe(collector.GetValue())
		}
	}
}
func (r *CollectCenter) CheckHealthAndRevert() (infos map[string][]CheckerItem) {
	infos = make(map[string][]CheckerItem)
	for _, collectors := range r.mCollector {
		for _, collector := range collectors {
			_, info := r.checkHealthDataRevert(collector)
			infos[collector.GetName()] = append(infos[collector.GetName()], info)
		}
	}
	return infos
}

func (r *CollectCenter) checkHealthDataRevert(collector monitor.Metric) (ok bool, info CheckerItem) {
	info = make(CheckerItem)
	var collectorInfo map[string]any
	switch collector := collector.(type) {
	case *monitor.HealthCheckerMetric:
		ok, collectorInfo = collector.IsOk()
	case *monitor.HealthCheckerAndRevertableMetric:
		ok, collectorInfo = collector.IsOk()
		if !ok {
			r.recoverLock.Lock()
			defer r.recoverLock.Unlock()
			r.recovering[collector.GetName()] = true
			r.revertCh <- Revertable{
				Name:       collector.GetName(),
				Revertable: collector,
			}
		}
	default:
		return true, nil
	}
	info["info"] = collectorInfo
	info["label"] = collector.GetLabelValues()
	info["metric"] = collector.GetName()
	info["value"] = collector.GetValue()
	if r.recovering[collector.GetName()] {
		info["recovering"] = true
	}
	if r.recoverErr[collector.GetName()] != nil {
		info["recoverErr"] = r.recoverErr[collector.GetName()].Error()
	}
	info["ok"] = ok
	return
}

type GlobalInfo interface {
	Get() map[string]string
}

var globalInfo GlobalInfo = &defaultInstance{}

type defaultInstance struct {
}

func (d *defaultInstance) Get() map[string]string {
	return map[string]string{}
}

func SetGlobalInfo(def GlobalInfo) {
	globalInfo = def
}

func getLabelValues(key string, labelValues monitor.LabelValues) string {
	if _, ok := labelValues[key]; ok {
		return labelValues[key]
	}
	return ""
}

func isDistinct(labels []string) bool {
	set := make(map[string]struct{})
	for _, label := range labels {
		set[label] = struct{}{}
	}
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	return len(keys) == len(labels)
}

type Gauge struct {
	metric      string
	label       []string
	labelValues []string
	collector   *prometheus.GaugeVec
	once        sync.Once
}

func NewGauge(metric string, label []string) *Gauge {
	return &Gauge{
		metric: metric,
		label:  label,
	}
}
func (g *Gauge) init() {
	g.once.Do(func() {
		if !isDistinct(g.label) {
			panic("label must be distinct")
		}
		g.collector = prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: g.metric,
		}, g.label)
		prometheus.MustRegister(g.collector)
	})
}

func (g *Gauge) WithLabelValues(labelValues monitor.LabelValues) *Gauge {
	g.labelValues = withLabelValues(g.label, labelValues)
	return g
}

func (g *Gauge) Set(value float64) {
	g.init()
	g.collector.WithLabelValues(g.labelValues...).Set(value)
}

func (g *Gauge) Inc() {
	g.init()
	g.collector.WithLabelValues(g.labelValues...).Inc()
}

func (g *Gauge) Dec() {
	g.init()
	g.collector.WithLabelValues(g.labelValues...).Dec()
}

func (g *Gauge) Add(value float64) {
	g.init()
	g.collector.WithLabelValues(g.labelValues...).Add(value)
}

func (g *Gauge) Sub(value float64) {
	g.init()
	g.collector.WithLabelValues(g.labelValues...).Sub(value)
}

type Counter struct {
	metric      string
	label       []string
	labelValues []string
	collector   *prometheus.CounterVec
	once        sync.Once
}

func NewCounter(metric string, label []string) *Counter {
	return &Counter{
		metric: metric,
		label:  label,
	}
}

func (c *Counter) init() {
	c.once.Do(func() {
		if !isDistinct(c.label) {
			panic("label must be distinct")
		}
		c.collector = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: c.metric,
		}, c.label)
		prometheus.MustRegister(c.collector)
	})
}
func (c *Counter) WithLabelValues(labelValues monitor.LabelValues) *Counter {
	c.labelValues = withLabelValues(c.label, labelValues)
	return c
}
func (c *Counter) Inc() {
	c.init()
	c.collector.WithLabelValues(c.labelValues...).Inc()
}

func (c *Counter) Add(value float64) {
	c.init()
	c.collector.WithLabelValues(c.labelValues...).Add(value)
}

type CounterVec struct {
	metric      string
	label       []string
	labelValues []string
	collector   *prometheus.CounterVec
	once        sync.Once
}

func NewCounterVec(metric string, label []string) *CounterVec {
	return &CounterVec{
		metric: metric,
		label:  label,
	}
}
func (c *CounterVec) WithLabelValues(labelValues monitor.LabelValues) *CounterVec {
	c.labelValues = withLabelValues(c.label, labelValues)
	return c
}
func (c *CounterVec) init() {
	c.once.Do(func() {
		if !isDistinct(c.label) {
			panic("label must be distinct")
		}
		c.collector = prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: c.metric,
		}, c.label)
		prometheus.MustRegister(c.collector)
	})
}

func (c *CounterVec) Inc() {
	c.init()
	c.collector.WithLabelValues(c.labelValues...).Inc()
}

type Histogram struct {
	metric      string
	label       []string
	labelValues []string
	buckets     []float64
	collector   *prometheus.HistogramVec
	once        sync.Once
}

func NewHistogram(metric string, label []string, buckets []float64) *Histogram {
	return &Histogram{
		metric:  metric,
		label:   label,
		buckets: buckets,
	}
}
func (h *Histogram) WithLabelValues(labelValues monitor.LabelValues) *Histogram {
	h.labelValues = withLabelValues(h.label, labelValues)
	return h
}
func (h *Histogram) init() {
	h.once.Do(func() {
		if !isDistinct(h.label) {
			panic("label must be distinct")
		}
		h.collector = prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    h.metric,
			Buckets: h.buckets,
		}, h.label)
		prometheus.MustRegister(h.collector)
	})
}

func (h *Histogram) Observe(value float64) {
	h.init()
	h.collector.WithLabelValues(h.labelValues...).Observe(value)
}

type Summary struct {
	metric      string
	label       []string
	labelValues []string
	maxAge      time.Duration
	ageBuckets  uint32
	collector   *prometheus.SummaryVec
	once        sync.Once
}

func NewSummary(metric string, label []string, maxAge time.Duration, ageBuckets uint32) *Summary {
	return &Summary{
		metric:     metric,
		label:      label,
		maxAge:     maxAge,
		ageBuckets: ageBuckets,
	}
}
func (s *Summary) WithLabelValues(labelValues monitor.LabelValues) *Summary {
	s.labelValues = withLabelValues(s.label, labelValues)
	return s
}
func (s *Summary) init() {
	s.once.Do(func() {
		if !isDistinct(s.label) {
			panic("label must be distinct")
		}
		s.collector = prometheus.NewSummaryVec(prometheus.SummaryOpts{
			Name:       s.metric,
			MaxAge:     s.maxAge,
			AgeBuckets: s.ageBuckets,
		}, s.label)
		prometheus.MustRegister(s.collector)
	})
}

func (s *Summary) Observe(value float64) {
	s.init()
	s.collector.WithLabelValues(s.labelValues...).Observe(value)
}

func withLabelValues(label []string, labelValues monitor.LabelValues) []string {
	labelValuesSlice := make([]string, 0, len(labelValues))
	for _, k := range label {
		labelValuesSlice = append(labelValuesSlice, getLabelValues(k, labelValues))
	}
	return labelValuesSlice
}
