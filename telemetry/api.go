package telemetry

import (
	"context"
	"net/http"
	"time"

	"vane-common/pkg/safe"

	"vane-common/pkg/logger"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	dto "github.com/prometheus/client_model/go"
)

type PushGateway struct {
	Enabled  bool
	URL      string
	JobName  string
	Username string
	Password string
	PushGap  int // 推送间隔时间，单位：秒
}

type Config struct {
	PushGateway        PushGateway
	DisableHealthCheck bool
	DisablePrometheus  bool
}

type globalGatherer struct {
	gatherer prometheus.Gatherer
}

func (ig *globalGatherer) Gather() ([]*dto.MetricFamily, error) {
	mfs, err := ig.gatherer.Gather()
	if err != nil {
		return nil, err
	}

	// 获取默认的 label 值
	defaultLabels := globalInfo.Get()
	if len(defaultLabels) == 0 {
		return mfs, nil
	}

	// 为每个 metric 添加默认 labels
	for _, mf := range mfs {
		for _, m := range mf.Metric {
			// 检查每个默认 label
			for labelName, labelValue := range defaultLabels {
				if labelValue == "" {
					continue
				}

				// 检查是否已经有这个 label
				hasLabel := false
				for _, label := range m.Label {
					if label.GetName() == labelName {
						hasLabel = true
						// 如果 label 值为空，则设置为默认值
						if label.GetValue() == "" {
							label.Value = &labelValue
						}
						break
					}
				}
				// 如果没有这个 label，则添加
				if !hasLabel {
					m.Label = append(m.Label, &dto.LabelPair{
						Name:  &labelName,
						Value: &labelValue,
					})
				}
			}
		}
	}

	return mfs, nil
}

func SetupAPI(eg *http.ServeMux, cfg ...Config) {
	config := Config{}
	if len(cfg) > 0 {
		config = cfg[0]
	}

	if !config.DisableHealthCheck {
		registerHealthCheck(eg)
	}
	if !config.DisablePrometheus {
		registerPrometheus(eg, config.PushGateway)
	}
}

func FlushPush() {
	select {
	case ImmediatelyPush <- struct{}{}:
	default:
	}
}

var pusher *push.Pusher

func ClosePush() {
	if pusher != nil {
		pusher.Delete()
	}
}

var ImmediatelyPush = make(chan struct{}, 1)

func registerPrometheus(eg *http.ServeMux, pushGateway PushGateway) {
	eg.Handle("/metrics", CC.MetricsHandlerFunc())

	if pushGateway.Enabled {

		if pushGateway.PushGap <= 0 {
			pushGateway.PushGap = 3
		}
		safe.GoKeep(context.Background(), 0, func() {
			ticker := time.NewTicker(time.Duration(pushGateway.PushGap) * time.Second)
			for {
				select {
				case <-ticker.C:
				case <-ImmediatelyPush:
				}
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				pusher = push.New(pushGateway.URL, pushGateway.JobName).
					BasicAuth(pushGateway.Username, pushGateway.Password).
					Gatherer(prometheus.DefaultGatherer)
				defaultLabels := globalInfo.Get()
				if len(defaultLabels) > 0 {
					for labelName, labelValue := range defaultLabels {
						pusher.Grouping(labelName, labelValue)
					}
				}

				if err := pusher.PushContext(ctx); err != nil {
					logger.Errorf(context.Background(), "push metrics to pushGateway error: %v", err)
				}
				cancel()
			}
		})
	}
}

func registerHealthCheck(eg *http.ServeMux) {
	// 注册健康检查的HTTP处理器
	eg.Handle("/healthz", CC.HealthHandlerFunc())
}
