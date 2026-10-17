package metric

import (
	"context"
	"slices"
	"vane-common/module/job"
	"vane-common/module/monitor"
	internalJob "vane-common/server/internal/itnlmodule/job"
	"vane-common/trpc"
	"vane-common/v1/pilot/system"

	"google.golang.org/protobuf/types/known/emptypb"
)

const (
	MetricDiscoverCount = "discover_count"
)

var _ monitor.IMonitor = (*DiscoverStatus)(nil)

type DiscoverStatus struct {
	discoverCount        *monitor.GaugeMetric
	discoverStatusHealth *DiscoverStatusHealthAndRevertable
}

func NewDiscoverStatus() *DiscoverStatus {
	return &DiscoverStatus{
		discoverCount:        monitor.NewGaugeMetric(MetricDiscoverCount, monitor.LabelValues{}),
		discoverStatusHealth: NewDiscoverStatusHealthAndRevertable(),
	}
}
func (d *DiscoverStatus) Collect() {
	svcmap := trpc.GetSvcmap().GetSvcmap()
	d.discoverCount.Set(float64(len(svcmap)))
}

func (d *DiscoverStatus) Metrics() []*monitor.MetricRegInfo {
	return []*monitor.MetricRegInfo{
		monitor.NewHealthCheckerAndRevertableMetric(d.discoverCount, d.discoverStatusHealth),
	}
}

var _ monitor.HealthCheckerAndRevertable = (*DiscoverStatusHealthAndRevertable)(nil)

type DiscoverStatusHealthAndRevertable struct {
}

func NewDiscoverStatusHealthAndRevertable() *DiscoverStatusHealthAndRevertable {
	return &DiscoverStatusHealthAndRevertable{}
}
func (d *DiscoverStatusHealthAndRevertable) IsOk() (ok bool, info map[string]any) {
	svcmap := trpc.GetSvcmap().GetSvcmap()
	conn, err := trpc.GetSvcConn(trpc.GetSystemCtx(context.Background()), trpc.CenterServer)
	if err != nil {
		return false, map[string]any{"error": err.Error()}
	}
	client := system.NewDiscoverServiceClient(conn)
	list, err := client.List(context.Background(), &emptypb.Empty{})
	if err != nil {
		return false, map[string]any{"error": err.Error()}
	}
	ok = true
	clusterList := make(map[string]*system.ServiceInfo)
	for _, v := range list.Services {
		clusterList[v.ServiceName] = v
	}
	for name, currentList := range svcmap {
		// 先判断vip是否存在 存在则跳过 不存在则判断endpoint是否完全一致
		if service, ok := clusterList[name]; ok {
			if service.ServiceVipEndpoint != "" {
				if !slices.Contains(currentList, service.ServiceVipEndpoint) {
					ok = false
					break
				}
			} else {
				if !slices.Contains(currentList, service.ServiceEndpoint) {
					ok = false
					break
				}
			}
		} else {
			ok = false
			break
		}
	}
	return ok, map[string]any{
		"service_list":  svcmap,
		"service_count": len(svcmap),
	}
}

func (d *DiscoverStatusHealthAndRevertable) Revert() error {
	job.Flush(context.Background(), internalJob.SyncSvcmapJobName)
	return nil
}
