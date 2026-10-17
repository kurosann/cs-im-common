package serverconfig

import (
	"context"

	"vane-common/telemetry"
)

type Config struct {
	Hosts           []string
	Port            int
	DisableGRPC     bool         // 是否禁用 gRPC server
	Reload          func() error // 重载配置
	EnableTelemetry bool         // 是否启用遥测
	TelemetryPort   int          // 遥测服务端口
	Telemetry       telemetry.Config
	DisableMonitor  bool // 是否禁用监控
	Standalone      bool // 是否以单机模式运行，禁用集群发现/同步等能力
}

type ctxKey string

const (
	ServerConfigKey ctxKey = "server.config"
)

func GetServerConfig(ctx context.Context) (*Config, bool) {
	serverConfig, ok := ctx.Value(ServerConfigKey).(*Config)
	if ok {
		return serverConfig, true
	}
	return nil, false
}
func SetServerConfig(ctx context.Context, config *Config) context.Context {
	return context.WithValue(ctx, ServerConfigKey, config)
}
