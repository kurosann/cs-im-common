# cs-im-common

CS IM 全系列的共享 Go 基础库，从内部基础框架沉淀而来，供 `cs-im-server` 及后续服务复用。

当前 Go module 名为 `vane-common`，属于历史沿用；新增引用仍使用该 module 路径。

## 定位

- 提供通用能力，不包含 CS IM 业务领域模型。
- 作为服务框架的公共层，承载模块化注册、HTTP 服务、命令启动、数据库接入和遥测。
- 模块间共享的能力放在这里；某个服务专用的能力放在对应服务仓库。

## 主要能力

| 目录 | 用途 |
| --- | --- |
| `module/` | 模块接口、HTTP API 注册、路由分组、中间件声明 |
| `server/` | 服务生命周期、模块装配、standalone 运行基础 |
| `cmd/` | Cobra 命令入口和启动参数 |
| `httpserver/` | HTTP server、路由装配、优雅停机 |
| `dbserver/` | 数据库服务装配 |
| `telemetry/` | 健康检查、Prometheus、指标采集等遥测能力 |
| `pkg/dataaccess/` | BaseModel、DataAccessor、GORM 访问抽象 |
| `pkg/logger/` | 日志能力 |
| `alertlog/`, `oplog/` | 告警记录与操作记录 |
| `publish/` | 发布订阅能力 |
| `trpc/` | 内部 RPC 客户端/服务端辅助 |
| `task/` | 任务记录和任务基础 |
| `discover/`, `clusterprofile/` | 服务发现与集群配置辅助 |

## 使用方式

服务通过 `server.NewServer` 组装配置和模块，通过 `httpserver.EnableHTTPServer` 注册模块 HTTP 路由，通过 `dbserver.EnableDBServer` 接入关系库。业务模块实现 `IModule`；需要 HTTP 时同时实现 `IHTTPModule`，并返回 `HttpApiInfo`。

遥测能力由框架内置，服务不需要额外实现 `healthz` / `readyz`。

## 开发

```bash
make test
go build ./...
```

修改公共接口时，需要同步检查引用它的 CS IM 服务，并保持最小接口边界。
