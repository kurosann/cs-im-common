package discover

import (
	"context"
	"fmt"
	"time"

	statemgr "vane-common/module/statemgr"
	"vane-common/server/pkg/role"
	"vane-common/trpc"

	"vane-common/pkg/node"
	"vane-common/trpc/common"
	"vane-common/v1/pilot/system"

	"google.golang.org/protobuf/types/known/emptypb"
)

var _ statemgr.IPushStateSyncer[*system.ServiceInfo] = (*Discover)(nil)

type Discover struct {
	name          string
	port          int
	telemetryPort int
	node          *node.ContainerNode
}

func NewDiscover(name string, port int, telemetryPort int) *Discover {
	return &Discover{
		name:          name,
		port:          port,
		telemetryPort: telemetryPort,
		node:          node.NewContainerNode(name),
	}
}

func (d *Discover) IsReady(ctx context.Context) bool {
	return true
}

func (d *Discover) ListLocal(ctx context.Context) ([]*system.ServiceInfo, error) {
	containerID, err := d.node.GetContainerID(ctx)
	if err != nil {
		return nil, err
	}
	ip, err := d.node.GetContainerIp(ctx)
	if err != nil {
		return nil, err
	}
	vip, _ := d.node.GetContainerVip(ctx)
	port, _ := d.node.GetContainerPort(ctx)
	serviceVipEndpoint := ""
	if vip != "" {
		serviceVipEndpoint = fmt.Sprintf("%s:%d", vip, port)
	}
	return []*system.ServiceInfo{
		{
			ServiceName:        d.name,
			ServiceId:          containerID,
			ServiceEndpoint:    fmt.Sprintf("%s:%d", ip[0], d.port),
			TelemetryEndpoint:  fmt.Sprintf("%s:%d", ip[0], d.telemetryPort),
			ServiceVipEndpoint: serviceVipEndpoint,
			ReplicationRole:    string(role.GetRole().ReplicationRole),
			NodeRole:           string(role.GetRole().NodeRole),
		},
	}, nil
}

func (d *Discover) Push(ctx context.Context, local *system.ServiceInfo) error {
	conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), trpc.CenterServer)
	if err != nil {
		return err
	}

	_, err = system.NewDiscoverServiceClient(conn).Sync(ctx, &system.ServiceInfo{
		ServiceName:        local.ServiceName,
		ServiceId:          local.ServiceId,
		ServiceEndpoint:    local.ServiceEndpoint,
		TelemetryEndpoint:  local.TelemetryEndpoint,
		ServiceVipEndpoint: local.ServiceVipEndpoint,
		ReplicationRole:    local.ReplicationRole,
		NodeRole:           local.NodeRole,
	})
	return err
}

func RemoveSelf(ctx context.Context) {
	conn, err := trpc.GetSvcConn(trpc.GetInternalCtx(ctx), trpc.CenterServer)
	if err != nil {
		return
	}
	d := node.NewContainerNode("")
	serviceId, err := d.GetContainerID(ctx)
	if err != nil {
		return
	}
	_, _ = system.NewDiscoverServiceClient(conn).Remove(ctx, &system.ServiceRemoveReq{ServiceId: serviceId})
}

var _ statemgr.AliveEntity = (*DiscoverAliveEntity)(nil)

type DiscoverEntityQuery interface {
	List(ctx context.Context) ([]*DiscoverAliveEntity, error)
	Delete(ctx context.Context, item *DiscoverAliveEntity) error
}

type DiscoverAliveEntity struct {
	ServiceName        string
	ServiceId          string
	ServiceEndpoint    string
	TelemetryEndpoint  string
	ServiceVipEndpoint string
	LastTime           time.Time
	ReplicationRole    string
	NodeRole           string
}

type DiscoverAliveEntityActionType int32

const (
	DiscoverAliveEntityActionTypeAdd    DiscoverAliveEntityActionType = 0
	DiscoverAliveEntityActionTypeUpdate DiscoverAliveEntityActionType = 1
	DiscoverAliveEntityActionTypeDelete DiscoverAliveEntityActionType = 2
)

type DiscoverAliveEntityAction struct {
	Action  DiscoverAliveEntityActionType
	Service *system.ServiceInfo
}

func (d *DiscoverAliveEntity) GetTimestamp() int64 {
	return d.LastTime.Unix()
}

var _ statemgr.IAliveChecker[*DiscoverAliveEntity] = (*DiscoverAliveChecker)(nil)

type DiscoverAliveChecker struct {
	query DiscoverEntityQuery
}

func NewDiscoverAliveChecker(query DiscoverEntityQuery) *DiscoverAliveChecker {
	return &DiscoverAliveChecker{query: query}
}

func (d *DiscoverAliveChecker) Delete(ctx context.Context, item *DiscoverAliveEntity) error {
	return d.query.Delete(ctx, item)
}

func (d *DiscoverAliveChecker) IsReady(ctx context.Context) bool {
	return true
}

func (d *DiscoverAliveChecker) List(ctx context.Context) ([]*DiscoverAliveEntity, error) {
	return d.query.List(ctx)
}

type DiscoverAliveServerQuery interface {
	Sync(ctx context.Context, req *system.ServiceInfo) (*emptypb.Empty, error)
}

type DiscoverSaveQuery interface {
	Save(ctx context.Context, item *DiscoverAliveEntity) error
	List(ctx context.Context) ([]*DiscoverAliveEntity, error)
	Watch(ctx context.Context) (chan *DiscoverAliveEntityAction, error)
	Delete(ctx context.Context, item *DiscoverAliveEntity) error
}

var _ system.DiscoverServiceServer = (*DiscoverAliveServer)(nil)

type DiscoverAliveServer struct {
	system.UnimplementedDiscoverServiceServer
	saveQuery DiscoverSaveQuery
}

func NewDiscoverAliveServer(saveQuery DiscoverSaveQuery) *DiscoverAliveServer {
	return &DiscoverAliveServer{saveQuery: saveQuery}
}

func (d *DiscoverAliveServer) Sync(ctx context.Context, req *system.ServiceInfo) (*emptypb.Empty, error) {
	err := d.saveQuery.Save(ctx, &DiscoverAliveEntity{
		ServiceName:        req.ServiceName,
		ServiceId:          req.ServiceId,
		ServiceEndpoint:    req.ServiceEndpoint,
		TelemetryEndpoint:  req.TelemetryEndpoint,
		ServiceVipEndpoint: req.ServiceVipEndpoint,
		LastTime:           time.Now(),
		ReplicationRole:    req.ReplicationRole,
		NodeRole:           req.NodeRole,
	})
	return nil, err
}

func (d *DiscoverAliveServer) Remove(ctx context.Context, req *system.ServiceRemoveReq) (*emptypb.Empty, error) {
	err := d.saveQuery.Delete(ctx, &DiscoverAliveEntity{
		ServiceId: req.ServiceId,
	})
	return &emptypb.Empty{}, err
}

func (d *DiscoverAliveServer) List(ctx context.Context, req *emptypb.Empty) (*system.ServiceList, error) {
	svcList, err := d.saveQuery.List(ctx)
	if err != nil {
		return nil, err
	}

	// 获取调用者的服务名称
	callerServiceName := common.GetServiceName(ctx)

	services := make([]*system.ServiceInfo, 0)
	for _, svc := range svcList {
		if callerServiceName == svc.ServiceName && svc.NodeRole == string(role.NodeRoleWorker) {
			continue
		}

		services = append(services, &system.ServiceInfo{
			ServiceName:        svc.ServiceName,
			ServiceId:          svc.ServiceId,
			ServiceEndpoint:    svc.ServiceEndpoint,
			TelemetryEndpoint:  svc.TelemetryEndpoint,
			ServiceVipEndpoint: svc.ServiceVipEndpoint,
			ReplicationRole:    svc.ReplicationRole,
			NodeRole:           svc.NodeRole,
		})
	}
	return &system.ServiceList{
		Services: services,
	}, nil
}

func (d *DiscoverAliveServer) Watch(req *emptypb.Empty, stream system.DiscoverService_WatchServer) error {
	ch, err := d.saveQuery.Watch(stream.Context())
	if err != nil {
		return err
	}

	callerServiceName := common.GetServiceName(stream.Context())

	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case data := <-ch:
			if callerServiceName == data.Service.ServiceName && data.Service.NodeRole == string(role.NodeRoleWorker) {
				continue
			}

			action := system.ServiceAction_SERVICE_ACTION_ADD
			switch data.Action {
			case DiscoverAliveEntityActionTypeAdd:
				action = system.ServiceAction_SERVICE_ACTION_ADD
			case DiscoverAliveEntityActionTypeUpdate:
				action = system.ServiceAction_SERVICE_ACTION_UPDATE
			case DiscoverAliveEntityActionTypeDelete:
				action = system.ServiceAction_SERVICE_ACTION_DELETE
			}
			stream.Send(&system.ServiceActionInfo{
				Action: action,
				Service: &system.ServiceInfo{
					ServiceName:        data.Service.ServiceName,
					ServiceId:          data.Service.ServiceId,
					ServiceEndpoint:    data.Service.ServiceEndpoint,
					TelemetryEndpoint:  data.Service.TelemetryEndpoint,
					ServiceVipEndpoint: data.Service.ServiceVipEndpoint,
					ReplicationRole:    data.Service.ReplicationRole,
					NodeRole:           data.Service.NodeRole,
				},
			})
		}
	}
}
