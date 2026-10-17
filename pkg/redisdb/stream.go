package redisdb

import (
	"context"
	"fmt"
	"vane-common/pkg/dataaccess/store"

	"github.com/redis/go-redis/v9"
)

var _ store.StreamStore = (*RedisDB)(nil)

// XAdd 追加一条记录,返回生成的条目 ID(形如 "1758000000000-0")。
// maxLen > 0 时按 MAXLEN ~ 近似裁剪:给日志一个条数上限兜底,避免异常流量把内存打满。
func (r *RedisDB) XAdd(ctx context.Context, key []byte, maxLen int64, values map[string]string) (string, error) {
	args := &redis.XAddArgs{Stream: string(key), Values: values}
	if maxLen > 0 {
		args.MaxLen = maxLen
		args.Approx = true
	}
	return r.db.XAdd(ctx, args).Result()
}

// XAddMinID 追加一条记录并按 MINID 裁剪,minID 通常取"保留窗口起点"对应的 ID。
//
// 两个实现要点:
//
//  1. MINID 必须用**精确**裁剪(=),不能用 ~ 近似:近似裁剪是整块 macro node 丢弃,
//     小 stream 只有一个 node 时会整块保留,过期数据永远删不掉(已由单测覆盖)。
//  2. 精确裁剪不能带 LIMIT(Redis 直接报 "LIMIT cannot be used without the special ~ option"),
//     所以单次写入的裁剪量不设上限。正常情况下每次写入只顺带清掉刚过期的那几条,
//     成本被摊薄;只有长期停写后恢复时才会有一次集中清理,属于可接受的一次性开销。
//
// XADD 的 MAXLEN 与 MINID 在 Redis 协议里互斥(go-redis 也只下发其一),
// 因此这里优先用 minID 表达"按时间保留",minID 为空才退回 maxLen(近似)。
func (r *RedisDB) XAddMinID(ctx context.Context, key []byte, minID string, maxLen int64, values map[string]string) (string, error) {
	args := &redis.XAddArgs{Stream: string(key), Values: values}
	switch {
	case minID != "":
		args.MinID = minID
	case maxLen > 0:
		args.MaxLen = maxLen
		args.Approx = true
	}
	return r.db.XAdd(ctx, args).Result()
}

// XRange 正序读取 [start, stop];count <= 0 表示不限条数。
// key 不存在时 Redis 返回空数组,这里如实返回空切片。
func (r *RedisDB) XRange(ctx context.Context, key []byte, start, stop string, count int64) ([]store.StreamEntry, error) {
	var (
		msgs []redis.XMessage
		err  error
	)
	if count > 0 {
		msgs, err = r.db.XRangeN(ctx, string(key), start, stop, count).Result()
	} else {
		msgs, err = r.db.XRange(ctx, string(key), start, stop).Result()
	}
	if err != nil {
		return nil, err
	}
	return toStreamEntries(msgs), nil
}

// XRevRange 倒序读取 [start, stop](start 为较大的一端,通常传 "+")。
func (r *RedisDB) XRevRange(ctx context.Context, key []byte, start, stop string, count int64) ([]store.StreamEntry, error) {
	var (
		msgs []redis.XMessage
		err  error
	)
	if count > 0 {
		msgs, err = r.db.XRevRangeN(ctx, string(key), start, stop, count).Result()
	} else {
		msgs, err = r.db.XRevRange(ctx, string(key), start, stop).Result()
	}
	if err != nil {
		return nil, err
	}
	return toStreamEntries(msgs), nil
}

// XLen 返回条目数;key 不存在时 Redis 返回 0。
func (r *RedisDB) XLen(ctx context.Context, key []byte) (int64, error) {
	return r.db.XLen(ctx, string(key)).Result()
}

// XTrimMinID 丢弃 ID < minID 的条目,返回被删除的条数。
func (r *RedisDB) XTrimMinID(ctx context.Context, key []byte, minID string) (int64, error) {
	return r.db.XTrimMinID(ctx, string(key), minID).Result()
}

// toStreamEntries 把 go-redis 的 XMessage 归一化成 store.StreamEntry。
//
// 字段值统一成 string:写入侧(业务)只写字符串,但 Redis 客户端在某些路径下会回 []byte,
// 两种都接受,其余类型按文本兜底,避免因类型抖动整批读取失败。
func toStreamEntries(msgs []redis.XMessage) []store.StreamEntry {
	if len(msgs) == 0 {
		return nil
	}
	out := make([]store.StreamEntry, 0, len(msgs))
	for _, m := range msgs {
		values := make(map[string]string, len(m.Values))
		for k, v := range m.Values {
			switch typed := v.(type) {
			case string:
				values[k] = typed
			case []byte:
				values[k] = string(typed)
			case nil:
				values[k] = ""
			default:
				values[k] = fmt.Sprintf("%v", typed)
			}
		}
		out = append(out, store.StreamEntry{ID: m.ID, Values: values})
	}
	return out
}
