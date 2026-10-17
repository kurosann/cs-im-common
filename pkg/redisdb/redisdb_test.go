package redisdb

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"
	"vane-common/pkg/dataaccess/store"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) (*RedisDB, func()) {
	// 启动一个测试用的 Redis 服务器
	s := miniredis.RunT(t)
	go func() {
		for {
			s.FastForward(time.Microsecond * 100)
			time.Sleep(time.Microsecond * 100)
		}
	}()

	// 创建 RedisDB 实例
	rdb, err := NewRedisDB(&redis.Options{
		Addr: s.Addr(),
	})
	require.NoError(t, err)

	// 返回清理函数
	cleanup := func() {
		s.Close()
	}

	return rdb, cleanup
}

func TestRedisDB_GetPut(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	key := []byte("test_key")
	value := []byte("test_value")

	// 测试 Put
	err := rdb.Put(ctx, key, value)
	assert.NoError(t, err)

	// 测试 Get
	result, err := rdb.Get(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, value, result)

	// 测试不存在的键
	_, err = rdb.Get(ctx, []byte("non_existent_key"))
	assert.Error(t, err)
}

func TestRedisDB_GetMissingReturnsStoreErrDataNotFound(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	_, err := rdb.Get(context.Background(), []byte("non_existent_key"))
	assert.Error(t, err)
	assert.True(t, errors.Is(err, store.ErrDataNotFound))
	assert.True(t, rdb.IsNotFound(err))
}

func TestRedisDB_PutWithTtl(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	key := []byte("ttl_key")
	value := []byte("ttl_value")
	ttl := 500 * time.Millisecond

	// 测试带TTL的Put
	err := rdb.PutWithTtl(ctx, key, value, ttl)
	assert.NoError(t, err)

	// 立即获取应该成功
	result, err := rdb.Get(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, value, result)

	// 等待TTL过期
	time.Sleep(1 * time.Second)

	// 过期后获取应该失败
	_, err = rdb.Get(ctx, key)
	assert.Error(t, err)
}

func TestRedisDB_Del(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	key := []byte("del_key")
	value := []byte("del_value")

	// 先存储一个值
	err := rdb.Put(ctx, key, value)
	assert.NoError(t, err)

	// 确认存在
	result, err := rdb.Get(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, value, result)

	// 删除
	err = rdb.Del(ctx, key)
	assert.NoError(t, err)

	// 确认已删除
	_, err = rdb.Get(ctx, key)
	assert.Error(t, err)
}

func TestRedisDB_Scan(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	prefix := "user:"

	// 准备测试数据
	testData := map[string]string{
		"user:1":    "alice",
		"user:2":    "bob",
		"user:3":    "charlie",
		"order:1":   "order_data",   // 不匹配前缀
		"product:1": "product_data", // 不匹配前缀
	}

	// 存储测试数据
	for k, v := range testData {
		err := rdb.Put(ctx, []byte(k), []byte(v))
		assert.NoError(t, err)
	}

	// 测试 Scan
	kv, err := rdb.Scan(ctx, []byte(prefix), func(k string, v []byte) bool {
		return true
	})
	assert.NoError(t, err)

	// 验证结果数量
	assert.Equal(t, 3, len(kv))

	// 验证返回的键值对
	expectedKeys := []string{"user:1", "user:2", "user:3"}
	for _, key := range expectedKeys {
		value, exists := kv[key]
		assert.True(t, exists, "key %s should exist in scan result", key)
		assert.Equal(t, []byte(testData[key]), value)
	}

	// 确认不匹配的键不在结果中
	_, exists := kv["order:1"]
	assert.False(t, exists, "order:1 should not be in scan result")
	_, exists = kv["product:1"]
	assert.False(t, exists, "product:1 should not be in scan result")
}

func TestRedisDB_Count(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	prefix := "count_test:"

	// 测试空前缀
	count, err := rdb.Count(ctx, []byte(prefix), func(k string, v []byte) bool {
		return false
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// 添加一些测试数据
	for i := 1; i <= 5; i++ {
		key := prefix + strconv.Itoa(i)
		err := rdb.Put(ctx, []byte(key), []byte("value"+strconv.Itoa(i)))
		assert.NoError(t, err)
	}

	// 添加一些不匹配的键
	err = rdb.Put(ctx, []byte("other:1"), []byte("other_value"))
	assert.NoError(t, err)

	// 测试计数
	count, err = rdb.Count(ctx, []byte(prefix), func(k string, v []byte) bool {
		return true
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(5), count)

	// 测试不存在的前缀
	count, err = rdb.Count(ctx, []byte("nonexistent:"), func(k string, v []byte) bool {
		return true
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestRedisDB_ScanEmpty(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	// 测试空数据库的扫描
	kv, err := rdb.Scan(ctx, []byte("nonexistent:"), func(k string, v []byte) bool {
		return true
	})
	assert.NoError(t, err)
	assert.NotNil(t, kv)
	assert.Equal(t, 0, len(kv))
}

func TestRedisDB_ScanLargeDataset(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()
	prefix := "large:"

	// 创建大量数据
	expectedCount := 100
	for i := 1; i <= expectedCount; i++ {
		key := prefix + strconv.Itoa(i)
		value := "value_" + strconv.Itoa(i)
		err := rdb.Put(ctx, []byte(key), []byte(value))
		assert.NoError(t, err)
	}

	// 添加一些噪音数据
	for i := 1; i <= 10; i++ {
		key := "other:" + strconv.Itoa(i)
		err := rdb.Put(ctx, []byte(key), []byte("other_value"))
		assert.NoError(t, err)
	}

	// 测试 Scan
	kv, err := rdb.Scan(ctx, []byte(prefix), func(k string, v []byte) bool {
		return true
	})
	assert.NoError(t, err)
	assert.Equal(t, expectedCount, len(kv))

	// 验证每个键值对
	for i := 1; i <= expectedCount; i++ {
		key := prefix + strconv.Itoa(i)
		expectedValue := "value_" + strconv.Itoa(i)
		value, exists := kv[key]
		assert.True(t, exists, "key %s should exist", key)
		assert.Equal(t, []byte(expectedValue), value)
	}

	// 测试 Count
	count, err := rdb.Count(ctx, []byte(prefix), func(k string, v []byte) bool {
		return true
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(expectedCount), count)
}

func TestRedisDB_Transaction(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	// 测试事务（实际上是简单的函数调用，因为当前实现不支持真正的事务）
	err := rdb.Transaction(ctx, func(txCtx context.Context) error {
		key := []byte("tx_key")
		value := []byte("tx_value")
		return rdb.Put(txCtx, key, value)
	})
	assert.NoError(t, err)

	// 验证事务中的操作已生效
	result, err := rdb.Get(ctx, []byte("tx_key"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("tx_value"), result)
}

func TestRedisDB_StartTx(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	// 测试 StartTx（当前实现返回原始context）
	txCtx := rdb.StartTx(ctx)
	assert.Equal(t, ctx, txCtx)
}

func TestRedisDB_Clean(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	ctx := context.Background()

	// Clean 方法在当前实现中是空操作
	err := rdb.Clean(ctx)
	assert.NoError(t, err)
}

func TestRedisDB_Close(t *testing.T) {
	rdb, cleanup := setupTestRedis(t)
	defer cleanup()

	// 测试关闭
	err := rdb.Close()
	assert.NoError(t, err)

	// 关闭后再次关闭应该会出错
	err = rdb.Close()
	assert.Error(t, err)
}

// 基准测试
func BenchmarkRedisDB_Put(b *testing.B) {
	rdb, cleanup := setupTestRedis(&testing.T{})
	defer cleanup()

	ctx := context.Background()
	key := []byte("bench_key")
	value := []byte("bench_value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rdb.Put(ctx, key, value)
	}
}

func BenchmarkRedisDB_Get(b *testing.B) {
	rdb, cleanup := setupTestRedis(&testing.T{})
	defer cleanup()

	ctx := context.Background()
	key := []byte("bench_key")
	value := []byte("bench_value")

	// 预先存储数据
	_ = rdb.Put(ctx, key, value)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rdb.Get(ctx, key)
	}
}

func BenchmarkRedisDB_Scan(b *testing.B) {
	rdb, cleanup := setupTestRedis(&testing.T{})
	defer cleanup()

	ctx := context.Background()
	prefix := "bench:"

	// 预先存储数据
	for i := 0; i < 100; i++ {
		key := prefix + strconv.Itoa(i)
		value := "value_" + strconv.Itoa(i)
		_ = rdb.Put(ctx, []byte(key), []byte(value))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = rdb.Scan(ctx, []byte(prefix), func(k string, v []byte) bool {
			return true
		})
	}
}
