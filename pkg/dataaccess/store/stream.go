package store

import (
	"context"
	"fmt"
	"time"

	"vane-common/pkg/dataaccess/internal/kv"
	"vane-common/pkg/dataaccess/model"
)

// StreamEntry 是 Stream 中的一条记录。
//
// ID 是 Redis Stream ID(形如 "1758000000000-0",前缀为毫秒时间戳),天然按时间单调递增,
// 可直接用作跨服务共享的增量游标;Values 是该条目的字段表(字符串键值)。
type StreamEntry struct {
	ID     string
	Values map[string]string
}

// StreamStore 是数据源的 Redis Stream 能力(XADD / XRANGE / XLEN / XTRIM / EXPIRE)。
//
// 它解决的是 KV 与 List 都覆盖不了的场景:**跨服务共享的追加型消息日志**。
//
//   - 写入方只管 XADD,读取方各自按游标 XRANGE;
//   - 读取**不消耗**条目(区别于 List 的 LRem),因此多个服务可以同时读同一份数据,
//     各自维护自己的进度,互不影响;
//   - 条目 ID 自带毫秒时间戳,可直接当 since 游标,无需额外的自增列。
//
// IKVStore 只覆盖单键读写,ListStore 的读取会消费数据,都不适合本场景。
// 数据源(如 pkg/redisdb.RedisDB)按需额外实现本接口,业务侧用 GetStream 获取。
//
// 硬约束:业务代码禁止为此直接 new redis client,必须经本接口取源。
type StreamStore interface {
	// XAdd 追加一条记录,返回生成的条目 ID。
	// maxLen > 0 时按 MAXLEN ~ maxLen 近似裁剪,给日志一个条数上限兜底。
	XAdd(ctx context.Context, key []byte, maxLen int64, values map[string]string) (string, error)

	// XAddMinID 追加一条记录,并按 MINID 近似裁剪(minID 通常取"保留窗口起点"的时间戳 ID),
	// 用于按时间保留(如只留最近 1 个月)。minID 为空时退化为 maxLen 语义。
	// 注意 XADD 的 MAXLEN 与 MINID 互斥,不能同时下达,故单独开一个方法而不是加参数。
	XAddMinID(ctx context.Context, key []byte, minID string, maxLen int64, values map[string]string) (string, error)

	// XRange 正序读取 [start, stop]。
	// start/stop 支持 Redis 原生语法:"-" 最小、"+" 最大、"(<id>" 表示独占。
	// count <= 0 表示不限条数;key 不存在时返回空切片而非错误。
	XRange(ctx context.Context, key []byte, start, stop string, count int64) ([]StreamEntry, error)

	// XRevRange 倒序读取 [start, stop](start 为较大的一端,通常传 "+")。
	XRevRange(ctx context.Context, key []byte, start, stop string, count int64) ([]StreamEntry, error)

	// XLen 返回条目数;key 不存在时返回 0。
	XLen(ctx context.Context, key []byte) (int64, error)

	// XTrimMinID 丢弃 ID < minID 的条目,返回删除条数(用于兜底的时间保留策略)。
	XTrimMinID(ctx context.Context, key []byte, minID string) (int64, error)

	// Expire 设置 key 的 TTL;每次写入后刷新即可让空闲的 stream 自动过期。
	Expire(ctx context.Context, key []byte, ttl time.Duration) error
}

// StreamIDAt 把时间点换算成 Redis Stream ID(形如 "1758000000000-0")。
//
// Stream ID 的前缀就是毫秒时间戳,因此"保留最近 N 天"这类策略可以直接表达成
// MINID = StreamIDAt(now - N*24h),不需要额外维护时间索引。
func StreamIDAt(t time.Time) string {
	return fmt.Sprintf("%d-0", t.UnixMilli())
}

// ErrStreamUnsupported 表示数据源已注册但不具备 Stream 能力。
type ErrStreamUnsupported struct {
	Source model.DbSource
}

func (e *ErrStreamUnsupported) Error() string {
	return fmt.Sprintf("kv source %q does not implement StreamStore", e.Source)
}

// GetKV 返回已注册的 KV 数据源。
//
// 未注册时 panic(与框架其它取源行为一致):取不到源属于装配错误,
// 应当在启动/首次使用时立刻暴露,而不是降级成静默的空数据。
func GetKV(source model.DbSource) IKVStore {
	return kv.Get(source).(IKVStore)
}

// GetStream 返回已注册数据源的 Stream 能力。
//
// 未注册或该数据源未实现 StreamStore 时返回 error(不 panic):
// "这个数据源支不支持 Stream"是可以按配置判断的运行时条件,
// 调用方通常要把它翻译成一个明确的启动期报错。
func GetStream(source model.DbSource) (StreamStore, error) {
	raw, ok := kv.Lookup(source)
	if !ok {
		return nil, fmt.Errorf("kv source %q is not registered", source)
	}
	s, ok := raw.(StreamStore)
	if !ok {
		return nil, &ErrStreamUnsupported{Source: source}
	}
	return s, nil
}
