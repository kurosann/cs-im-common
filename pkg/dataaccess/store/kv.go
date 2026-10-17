package store

import (
	"context"
	"errors"
	"time"

	"vane-common/pkg/dataaccess/internal/kv"
	"vane-common/pkg/dataaccess/model"
)

var ErrDataNotFound = errors.New("kv data not found")

// true代表保留 false 代表要被过滤
type FilterFunc func(k string, v []byte) bool

type IKVStore interface {
	Get(ctx context.Context, key []byte) (value []byte, err error)
	Put(ctx context.Context, key []byte, value []byte) (err error)
	PutWithTtl(ctx context.Context, key []byte, value []byte, ttl time.Duration) (err error)
	Scan(ctx context.Context, prefix []byte, filter FilterFunc) (kv map[string][]byte, err error)
	Count(ctx context.Context, prefix []byte, filter FilterFunc) (count int64, err error)
	Del(ctx context.Context, key []byte) (err error)
	StartTx(ctx context.Context) context.Context
	Transaction(ctx context.Context, fn func(ctx context.Context) error) (err error)
	Clean(ctx context.Context) (err error)
	Close() (err error)
}

type NotFoundClassifier interface {
	IsNotFound(error) bool
}

// ListStore 是支持 Redis List 语义的 KV store 可选能力。
//
// IKVStore 只保证 KV 语义(Get/Put/Scan);支持 List 的存储(如 Redis)额外实现本接口。
// 业务侧用 GetKV(source).(ListStore) 断言获取,无此能力的 store 保持 IKVStore 行为不变。
type ListStore interface {
	// RPush 从尾部追加多个元素(时间正序:新消息在末尾)。
	RPush(ctx context.Context, key []byte, values ...[]byte) (err error)
	// LPush 从头部插入多个元素。
	LPush(ctx context.Context, key []byte, values ...[]byte) (err error)
	// LRange 返回 [start, stop] 闭区间元素;stop 为 -1 表示到末尾。
	LRange(ctx context.Context, key []byte, start, stop int64) (values [][]byte, err error)
	// LLen 返回列表长度。
	LLen(ctx context.Context, key []byte) (length int64, err error)
	// LTrim 只保留 [start, stop] 闭区间元素,用于限制列表长度。
	LTrim(ctx context.Context, key []byte, start, stop int64) (err error)
	// LSet 覆写指定下标元素,用于原地更新列表中元素(如消费标记)。
	// 下标越界返回 error。
	LSet(ctx context.Context, key []byte, index int64, value []byte) (err error)
	// LRem 从列表删除 count 个与 value 相等的元素(count>0 从头,count<0 从尾,count=0 全部)。
	// 返回实际删除个数;元素含唯一 id 时可安全用于"按内容移动/删除"。
	LRem(ctx context.Context, key []byte, count int64, value []byte) (removed int64, err error)
	// Expire 给 key 设置过期时间(TTL)。
	Expire(ctx context.Context, key []byte, ttl time.Duration) (err error)
}

func RegisterKVDB(name model.DbSource, kvdb IKVStore) {
	kv.Register(name, kvdb)
}
