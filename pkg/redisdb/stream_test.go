package redisdb

import (
	"context"
	"testing"
	"time"

	"vane-common/pkg/dataaccess/model"
	"vane-common/pkg/dataaccess/store"

	"github.com/redis/go-redis/v9"
)

// 依赖本地 Redis(127.0.0.1:6379);用例使用独立 key 前缀并在结束时清理。
// Redis 不可达时 t.Skip,保证无 Redis 的环境不会误报失败。
func newTestStream(t *testing.T, source string) (store.StreamStore, *RedisDB) {
	t.Helper()
	ctx := context.Background()
	rdb, err := NewRedisDB(&redis.Options{Addr: "127.0.0.1:6379"})
	if err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	if err := rdb.db.Ping(ctx).Err(); err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	store.RegisterKVDB(model.DbSource(source), rdb)
	ss, err := store.GetStream(model.DbSource(source))
	if err != nil {
		t.Fatalf("GetStream: %v", err)
	}
	return ss, rdb
}

func TestStreamStoreAppendRangeAndTrim(t *testing.T) {
	ctx := context.Background()
	ss, rdb := newTestStream(t, "test_stream")

	key := []byte("test:stream:append")
	t.Cleanup(func() { _ = rdb.db.Del(ctx, string(key)) })
	if err := rdb.Del(ctx, key); err != nil {
		t.Fatalf("clean: %v", err)
	}

	// 空 key:XLen=0,XRange 返回空且不报错。
	if n, err := ss.XLen(ctx, key); err != nil || n != 0 {
		t.Fatalf("XLen empty = (%d, %v), want (0, nil)", n, err)
	}
	entries, err := ss.XRange(ctx, key, "-", "+", 0)
	if err != nil {
		t.Fatalf("XRange empty: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("XRange empty len = %d, want 0", len(entries))
	}

	// XAdd 三条,ID 应单调递增且可当游标用。
	var ids []string
	for i, body := range []string{"m1", "m2", "m3"} {
		id, err := ss.XAdd(ctx, key, 0, map[string]string{"data": body})
		if err != nil {
			t.Fatalf("XAdd #%d: %v", i, err)
		}
		if id == "" {
			t.Fatalf("XAdd #%d returned empty id", i)
		}
		ids = append(ids, id)
	}
	if !(ids[0] < ids[1] && ids[1] < ids[2]) {
		t.Fatalf("stream ids not increasing: %v", ids)
	}
	if n, err := ss.XLen(ctx, key); err != nil || n != 3 {
		t.Fatalf("XLen = (%d, %v), want (3, nil)", n, err)
	}

	// 全量正序读取。
	entries, err = ss.XRange(ctx, key, "-", "+", 0)
	if err != nil {
		t.Fatalf("XRange: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("XRange len = %d, want 3", len(entries))
	}
	for i, want := range []string{"m1", "m2", "m3"} {
		if got := entries[i].Values["data"]; got != want {
			t.Fatalf("entry %d data = %q, want %q", i, got, want)
		}
		if entries[i].ID != ids[i] {
			t.Fatalf("entry %d id = %q, want %q", i, entries[i].ID, ids[i])
		}
	}

	// count 限流 + 增量游标:从最后一条之后读,应为空。
	head, err := ss.XRange(ctx, key, "-", "+", 2)
	if err != nil {
		t.Fatalf("XRange count: %v", err)
	}
	if len(head) != 2 {
		t.Fatalf("XRange count len = %d, want 2", len(head))
	}
	tail, err := ss.XRange(ctx, key, "("+ids[2], "+", 0)
	if err != nil {
		t.Fatalf("XRange after cursor: %v", err)
	}
	if len(tail) != 0 {
		t.Fatalf("XRange after last cursor len = %d, want 0", len(tail))
	}

	// 倒序读取最新一条。
	rev, err := ss.XRevRange(ctx, key, "+", "-", 1)
	if err != nil {
		t.Fatalf("XRevRange: %v", err)
	}
	if len(rev) != 1 || rev[0].Values["data"] != "m3" {
		t.Fatalf("XRevRange = %+v, want latest m3", rev)
	}

	// MINID 裁剪:丢掉前两条。
	removed, err := ss.XTrimMinID(ctx, key, ids[2])
	if err != nil {
		t.Fatalf("XTrimMinID: %v", err)
	}
	if removed != 2 {
		t.Fatalf("XTrimMinID removed = %d, want 2", removed)
	}
	if n, _ := ss.XLen(ctx, key); n != 1 {
		t.Fatalf("XLen after trim = %d, want 1", n)
	}
}

func TestStreamStoreXAddMinIDTrimsOnWrite(t *testing.T) {
	ctx := context.Background()
	ss, rdb := newTestStream(t, "test_stream_minid")

	key := []byte("test:stream:minid")
	t.Cleanup(func() { _ = rdb.db.Del(ctx, string(key)) })
	if err := rdb.Del(ctx, key); err != nil {
		t.Fatalf("clean: %v", err)
	}

	// 直接以显式旧 ID 植入一条真正的"过期"条目(XADD * 只能生成当前时间的 ID,
	// 没法造出历史数据;这里绕过业务接口,只为本用例构造前置状态)。
	// ID 前缀是毫秒时间戳,1000-0 相当于 1970 年。
	const oldID = "1000-0"
	if err := rdb.db.XAdd(ctx, &redis.XAddArgs{
		Stream: string(key),
		ID:     oldID,
		Values: map[string]string{"data": "old"},
	}).Err(); err != nil {
		t.Fatalf("plant old entry: %v", err)
	}

	// 以"1 小时前"为 MINID 写入新条目,旧条目应被顺带裁掉。
	windowStart := store.StreamIDAt(time.Now().Add(-time.Hour))
	newID, err := ss.XAddMinID(ctx, key, windowStart, 0, map[string]string{"data": "new"})
	if err != nil {
		t.Fatalf("XAddMinID: %v", err)
	}
	entries, err := ss.XRange(ctx, key, "-", "+", 0)
	if err != nil {
		t.Fatalf("XRange: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != newID {
		t.Fatalf("after XAddMinID entries = %+v, want only %q (old was %q)", entries, newID, oldID)
	}

	// TTL:刷新后应能读到过期时间。
	if err := ss.Expire(ctx, key, time.Hour); err != nil {
		t.Fatalf("Expire: %v", err)
	}
	if ttl := rdb.db.TTL(ctx, string(key)).Val(); ttl <= 0 {
		t.Fatalf("TTL = %v, want > 0", ttl)
	}
}
