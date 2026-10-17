package redisdb

import (
	"context"
	"errors"
	"time"
	"vane-common/pkg/dataaccess/store"

	"github.com/redis/go-redis/v9"
)

var _ store.IKVStore = (*RedisDB)(nil)
var _ store.ListStore = (*RedisDB)(nil)
var _ store.NotFoundClassifier = (*RedisDB)(nil)

type RedisDB struct {
	db *redis.Client
}

func NewRedisDB(opt *redis.Options) (*RedisDB, error) {
	db := redis.NewClient(opt)
	return &RedisDB{db: db}, nil
}

func (r *RedisDB) Clean(ctx context.Context) (err error) {
	return nil
}

func (r *RedisDB) Close() (err error) {
	return r.db.Close()
}

func (r *RedisDB) Count(ctx context.Context, prefix []byte, filter store.FilterFunc) (count int64, err error) {
	prefixStr := string(prefix)
	iter := r.db.Scan(ctx, 0, prefixStr+"*", 0).Iterator()
	for iter.Next(ctx) {
		value, err := r.db.Get(ctx, iter.Val()).Bytes()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return 0, store.ErrDataNotFound
			}
			return 0, err
		}
		if filter != nil && !filter(iter.Val(), value) {
			continue
		}
		count++
	}
	return count, iter.Err()
}

func (r *RedisDB) Del(ctx context.Context, key []byte) (err error) {
	return r.db.Del(ctx, string(key)).Err()
}

func (r *RedisDB) Get(ctx context.Context, key []byte) (value []byte, err error) {
	if val, err := r.db.Get(ctx, string(key)).Bytes(); err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, store.ErrDataNotFound
		}
		return nil, err
	} else {
		return val, nil
	}
}

func (r *RedisDB) Put(ctx context.Context, key []byte, value []byte) (err error) {
	return r.db.Set(ctx, string(key), value, 0).Err()
}

func (r *RedisDB) PutWithTtl(ctx context.Context, key []byte, value []byte, ttl time.Duration) (err error) {
	return r.db.Set(ctx, string(key), value, ttl).Err()
}

func (r *RedisDB) Scan(ctx context.Context, prefix []byte, filter store.FilterFunc) (kv map[string][]byte, err error) {
	prefixStr := string(prefix)
	kv = make(map[string][]byte)

	// 使用 SCAN 命令扫描所有匹配的键
	iter := r.db.Scan(ctx, 0, prefixStr+"*", 0).Iterator()
	for iter.Next(ctx) {
		value, err := r.db.Get(ctx, iter.Val()).Bytes()
		if err != nil {
			if errors.Is(err, redis.Nil) {
				return nil, store.ErrDataNotFound
			}
			return nil, err
		}
		if filter != nil && !filter(iter.Val(), value) {
			continue
		}
		kv[iter.Val()] = value
	}

	if err = iter.Err(); err != nil {
		return nil, err
	}

	return kv, nil
}

func (r *RedisDB) IsNotFound(err error) bool {
	return errors.Is(err, redis.Nil) || errors.Is(err, store.ErrDataNotFound)
}

func (r *RedisDB) StartTx(ctx context.Context) context.Context {
	// redis 不支持事务
	return ctx
}

func (r *RedisDB) Transaction(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	// redis 不支持事务
	return fn(ctx)
}

// ---- Redis List 能力(store.ListStore) ----

// RPush 从列表尾部追加多个元素(消息时间正序:新消息在末尾)。
func (r *RedisDB) RPush(ctx context.Context, key []byte, values ...[]byte) (err error) {
	return r.db.RPush(ctx, string(key), bytesToAny(values)...).Err()
}

// LPush 从列表头部插入多个元素。
func (r *RedisDB) LPush(ctx context.Context, key []byte, values ...[]byte) (err error) {
	return r.db.LPush(ctx, string(key), bytesToAny(values)...).Err()
}

// LRange 返回列表 [start, stop] 闭区间内的元素;stop 为 -1 表示到列表末尾。
// 列表为空或 key 不存在时返回空切片(nil error)。
func (r *RedisDB) LRange(ctx context.Context, key []byte, start, stop int64) (values [][]byte, err error) {
	items, err := r.db.LRange(ctx, string(key), start, stop).Result()
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	values = make([][]byte, 0, len(items))
	for _, item := range items {
		values = append(values, []byte(item))
	}
	return values, nil
}

// LLen 返回列表长度;key 不存在时返回 0。
func (r *RedisDB) LLen(ctx context.Context, key []byte) (length int64, err error) {
	return r.db.LLen(ctx, string(key)).Result()
}

// LTrim 只保留列表 [start, stop] 闭区间内的元素,用于限制列表最大长度。
func (r *RedisDB) LTrim(ctx context.Context, key []byte, start, stop int64) (err error) {
	return r.db.LTrim(ctx, string(key), start, stop).Err()
}

// LSet 覆写列表指定下标元素(用于消费标记等原地更新);下标越界返回 error。
func (r *RedisDB) LSet(ctx context.Context, key []byte, index int64, value []byte) (err error) {
	return r.db.LSet(ctx, string(key), index, value).Err()
}

// LRem 从列表删除 count 个与 value 相等的元素,返回实际删除个数。
func (r *RedisDB) LRem(ctx context.Context, key []byte, count int64, value []byte) (removed int64, err error) {
	return r.db.LRem(ctx, string(key), count, value).Result()
}

// Expire 设置 key 的过期时间(TTL)。
func (r *RedisDB) Expire(ctx context.Context, key []byte, ttl time.Duration) (err error) {
	return r.db.Expire(ctx, string(key), ttl).Err()
}

func bytesToAny(values [][]byte) []any {
	args := make([]any, 0, len(values))
	for _, v := range values {
		args = append(args, v)
	}
	return args
}
