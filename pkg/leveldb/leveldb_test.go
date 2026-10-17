package leveldb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/syndtr/goleveldb/leveldb"
)

func TestLevelDB(t *testing.T) {

	db, err := NewLevelDB("test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		db.Close()
		os.RemoveAll("test.db")
	}()

	// 测试基本操作
	err = db.Put(context.Background(), []byte("test"), []byte("test"))
	if err != nil {
		t.Fatal(err)
	}

	value, err := db.Get(context.Background(), []byte("test"))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "test", string(value))

	err = db.Del(context.Background(), []byte("test"))
	if err != nil {
		t.Fatal(err)
	}

	value, err = db.Get(context.Background(), []byte("test"))
	assert.Error(t, err)
	assert.Equal(t, leveldb.ErrNotFound, err)
}

func TestLevelDBFileCount(t *testing.T) {
	// 清理旧的测试数据库文件
	os.RemoveAll("test_files.db")

	db, err := NewLevelDB("test_files.db")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		db.Close()
		os.RemoveAll("test_files.db")
	}()

	// 写入一些测试数据
	for i := 0; i < 100; i++ {
		key := []byte(fmt.Sprintf("key_%d", i))
		value := []byte(fmt.Sprintf("value_%d", i))
		err = db.Put(context.Background(), key, value)
		if err != nil {
			t.Fatal(err)
		}
	}

	// 检查文件数量
	files, err := filepath.Glob("test_files.db/*")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("数据库文件数量: %d", len(files))
	for _, file := range files {
		t.Logf("文件: %s", file)
	}

	// 执行压缩
	err = db.Compact()
	if err != nil {
		t.Fatal(err)
	}

	// 再次检查文件数量
	filesAfter, err := filepath.Glob("test_files.db/*")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("压缩后文件数量: %d", len(filesAfter))
	for _, file := range filesAfter {
		t.Logf("文件: %s", file)
	}

	// 验证数据完整性
	for i := 0; i < 100; i++ {
		key := []byte(fmt.Sprintf("key_%d", i))
		expectedValue := fmt.Sprintf("value_%d", i)

		value, err := db.Get(context.Background(), key)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expectedValue, string(value))
	}
}

// TestLevelDB_PutWithTtl 测试TTL功能
func TestLevelDB_PutWithTtl(t *testing.T) {
	// 清理旧的测试数据库文件
	os.RemoveAll("test_ttl.db")

	db, err := NewLevelDB("test_ttl.db")
	assert.NoError(t, err)
	defer func() {
		db.Close()
		os.RemoveAll("test_ttl.db")
	}()

	ctx := context.Background()

	// 测试带TTL的存储
	key := []byte("ttl_key")
	value := []byte("ttl_value")
	ttl := 100 * time.Millisecond

	err = db.PutWithTtl(ctx, key, value, ttl)
	assert.NoError(t, err)

	// 立即读取应该成功
	result, err := db.Get(ctx, key)
	assert.NoError(t, err)
	assert.Equal(t, value, result)

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	// 过期后读取应该失败
	result, err = db.Get(ctx, key)
	assert.Error(t, err)
	assert.Equal(t, leveldb.ErrNotFound, err)
	assert.Nil(t, result)
}

// TestLevelDB_Clean 测试清理过期数据功能
func TestLevelDB_Clean(t *testing.T) {
	// 清理旧的测试数据库文件
	os.RemoveAll("test_clean.db")

	db, err := NewLevelDB("test_clean.db")
	assert.NoError(t, err)
	defer func() {
		db.Close()
		os.RemoveAll("test_clean.db")
	}()

	ctx := context.Background()

	// 添加一些数据，部分带TTL
	err = db.Put(ctx, []byte("permanent_key"), []byte("permanent_value"))
	assert.NoError(t, err)

	err = db.PutWithTtl(ctx, []byte("ttl_key1"), []byte("ttl_value1"), 50*time.Millisecond)
	assert.NoError(t, err)

	err = db.PutWithTtl(ctx, []byte("ttl_key2"), []byte("ttl_value2"), 50*time.Millisecond)
	assert.NoError(t, err)

	// 等待TTL过期
	time.Sleep(100 * time.Millisecond)

	// 清理过期数据
	err = db.Clean(ctx)
	assert.NoError(t, err)

	// 永久数据应该还存在
	result, err := db.Get(ctx, []byte("permanent_key"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("permanent_value"), result)

	// TTL数据应该被清理了（通过直接数据库查询验证）
	_, err = db.db.Get([]byte("ttl_key1"), nil)
	assert.Error(t, err)
	assert.Equal(t, leveldb.ErrNotFound, err)

	_, err = db.db.Get([]byte("ttl_key2"), nil)
	assert.Error(t, err)
	assert.Equal(t, leveldb.ErrNotFound, err)
}

// TestLevelDB_Scan 测试扫描功能
func TestLevelDB_Scan(t *testing.T) {
	// 清理旧的测试数据库文件
	os.RemoveAll("test_scan.db")

	db, err := NewLevelDB("test_scan.db")
	assert.NoError(t, err)
	defer func() {
		db.Close()
		os.RemoveAll("test_scan.db")
	}()

	ctx := context.Background()

	// 添加一些测试数据（不包含过期数据）
	testData := map[string]string{
		"user:1": "alice",
		"user:2": "bob",
		"user:3": "charlie",
		"post:1": "hello world",
		"post:2": "leveldb test",
	}

	for key, value := range testData {
		t.Logf("写入数据: key='%s', value='%s'", key, value)
		err = db.Put(ctx, []byte(key), []byte(value))
		assert.NoError(t, err)

		// 立即验证写入是否正确
		result, err := db.Get(ctx, []byte(key))
		assert.NoError(t, err)
		assert.Equal(t, value, string(result), "键 %s 的值不匹配", key)
	}

	// 扫描user前缀的数据
	result, err := db.Scan(ctx, []byte("user:"), func(k string, v []byte) bool {
		return true
	})
	assert.NoError(t, err)
	t.Logf("user扫描结果: %+v", result)

	// 应该包含3个用户
	assert.Len(t, result, 3)
	if len(result) == 3 {
		assert.Equal(t, "alice", string(result["user:1"]))
		assert.Equal(t, "bob", string(result["user:2"]))
		assert.Equal(t, "charlie", string(result["user:3"]))
	}

	// 扫描post前缀的数据
	result, err = db.Scan(ctx, []byte("post:"), func(k string, v []byte) bool {
		return true
	})
	assert.NoError(t, err)
	t.Logf("post扫描结果: %+v", result)

	assert.Len(t, result, 2)
	if len(result) == 2 {
		assert.Equal(t, "hello world", string(result["post:1"]))
		assert.Equal(t, "leveldb test", string(result["post:2"]))
	}
}

// TestLevelDB_ScanWithTtl 测试包含TTL的扫描功能
func TestLevelDB_ScanWithTtl(t *testing.T) {
	// 清理旧的测试数据库文件
	os.RemoveAll("test_scan_ttl.db")

	db, err := NewLevelDB("test_scan_ttl.db")
	assert.NoError(t, err)
	defer func() {
		db.Close()
		os.RemoveAll("test_scan_ttl.db")
	}()

	ctx := context.Background()

	// 添加永久数据
	err = db.Put(ctx, []byte("user:1"), []byte("alice"))
	assert.NoError(t, err)

	// 添加即将过期的数据
	err = db.PutWithTtl(ctx, []byte("user:2"), []byte("expired_user"), 50*time.Millisecond)
	assert.NoError(t, err)

	// 立即扫描应该包含2个用户
	result, err := db.Scan(ctx, []byte("user:"), func(k string, v []byte) bool {
		return true
	})
	assert.NoError(t, err)
	assert.Len(t, result, 2)

	// 等待过期
	time.Sleep(100 * time.Millisecond)

	// 过期后扫描应该只包含1个用户
	result, err = db.Scan(ctx, []byte("user:"), func(k string, v []byte) bool {
		return true
	})
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "alice", string(result["user:1"]))
	assert.NotContains(t, result, "user:2")
}

// TestLevelDB_Count 测试计数功能
func TestLevelDB_Count(t *testing.T) {
	// 清理旧的测试数据库文件
	os.RemoveAll("test_count.db")

	db, err := NewLevelDB("test_count.db")
	assert.NoError(t, err)
	defer func() {
		db.Close()
		os.RemoveAll("test_count.db")
	}()

	ctx := context.Background()

	// 添加一些测试数据
	for i := 1; i <= 5; i++ {
		key := []byte(fmt.Sprintf("item:%d", i))
		value := []byte(fmt.Sprintf("value_%d", i))
		err = db.Put(ctx, key, value)
		assert.NoError(t, err)
	}

	// 添加过期数据
	err = db.PutWithTtl(ctx, []byte("item:6"), []byte("expired_value"), 50*time.Millisecond)
	assert.NoError(t, err)

	// 立即计数应该是6
	count, err := db.Count(ctx, []byte("item:"), func(k string, v []byte) bool {
		return true
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(6), count)

	// 等待过期
	time.Sleep(100 * time.Millisecond)

	// 过期后计数应该是5（过期数据被自动清理）
	count, err = db.Count(ctx, []byte("item:"), func(k string, v []byte) bool {
		return true
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(5), count)

	// 测试空前缀计数
	count, err = db.Count(ctx, []byte("nonexistent:"), func(k string, v []byte) bool {
		return true
	})
	assert.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

// TestLevelDB_Transaction 测试事务功能
func TestLevelDB_Transaction(t *testing.T) {
	// 清理旧的测试数据库文件
	os.RemoveAll("test_tx.db")

	db, err := NewLevelDB("test_tx.db")
	assert.NoError(t, err)
	defer func() {
		db.Close()
		os.RemoveAll("test_tx.db")
	}()

	ctx := context.Background()

	// 测试StartTx
	txCtx := db.StartTx(ctx)
	assert.NotNil(t, txCtx)

	// 测试Transaction执行
	executed := false
	err = db.Transaction(ctx, func(ctx context.Context) error {
		executed = true
		// 在事务中执行一些操作
		return db.Put(ctx, []byte("tx_key"), []byte("tx_value"))
	})

	assert.NoError(t, err)
	assert.True(t, executed)

	// 验证数据已写入
	value, err := db.Get(ctx, []byte("tx_key"))
	assert.NoError(t, err)
	assert.Equal(t, []byte("tx_value"), value)

	// 测试事务中的错误处理
	err = db.Transaction(ctx, func(ctx context.Context) error {
		return fmt.Errorf("test error")
	})
	assert.Error(t, err)
	assert.Equal(t, "test error", err.Error())
}

// TestLevelDB_EncodeDecode 测试编解码功能
func TestLevelDB_EncodeDecode(t *testing.T) {
	// 测试不带过期时间的编解码
	value := []byte("test value")
	encoded := encodeValueWithExpire(value, 0)

	decodedValue, expireAt := decodeValueWithExpire(encoded)
	assert.Equal(t, value, decodedValue)
	assert.Equal(t, int64(0), expireAt)

	// 测试带过期时间的编解码
	expireTime := time.Now().Add(time.Hour).UnixNano()
	encoded = encodeValueWithExpire(value, expireTime)

	decodedValue, decodedExpireAt := decodeValueWithExpire(encoded)
	assert.Equal(t, value, decodedValue)
	assert.Equal(t, expireTime, decodedExpireAt)

	// 测试兼容旧格式（数据长度小于8字节）
	shortData := []byte("short")
	decodedValue, expireAt = decodeValueWithExpire(shortData)
	assert.Equal(t, shortData, decodedValue)
	assert.Equal(t, int64(0), expireAt)
}

// TestLevelDB_ScanDebug 调试扫描功能
func TestLevelDB_ScanDebug(t *testing.T) {
	// 清理旧的测试数据库文件
	os.RemoveAll("test_scan_debug.db")

	db, err := NewLevelDB("test_scan_debug.db")
	assert.NoError(t, err)
	defer func() {
		db.Close()
		os.RemoveAll("test_scan_debug.db")
	}()

	ctx := context.Background()

	// 测试一个简单的键值对
	key := "user:1"
	value := "alice"

	t.Logf("原始数据: key='%s', value='%s'", key, value)

	err = db.Put(ctx, []byte(key), []byte(value))
	assert.NoError(t, err)

	// 直接读取验证
	result, err := db.Get(ctx, []byte(key))
	assert.NoError(t, err)
	t.Logf("直接读取结果: '%s' (长度: %d)", string(result), len(result))
	assert.Equal(t, value, string(result))

	// 通过Scan读取验证
	scanResult, err := db.Scan(ctx, []byte("user:"), func(k string, v []byte) bool {
		return true
	})
	assert.NoError(t, err)
	t.Logf("扫描结果: %+v", scanResult)

	if val, exists := scanResult[key]; exists {
		t.Logf("扫描到的值: '%s' (长度: %d)", string(val), len(val))
		assert.Equal(t, value, string(val))
	} else {
		t.Errorf("扫描结果中没有找到键 '%s'", key)
	}
}

func testKey() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(i)
	}
	return k
}

func testKey2() []byte {
	k := make([]byte, 32)
	for i := range k {
		k[i] = byte(255 - i)
	}
	return k
}

func TestLevelDB_Encryption_Basic(t *testing.T) {
	os.RemoveAll("test_encrypt.db")

	db, err := NewLevelDB("test_encrypt.db", WithEncryption(testKey()))
	assert.NoError(t, err)
	defer func() {
		db.Close()
		os.RemoveAll("test_encrypt.db")
	}()

	ctx := context.Background()

	// Put and Get
	err = db.Put(ctx, []byte("secret"), []byte("mysecret"))
	assert.NoError(t, err)

	value, err := db.Get(ctx, []byte("secret"))
	assert.NoError(t, err)
	assert.Equal(t, "mysecret", string(value))

	// Verify raw stored data contains magic prefix (is encrypted)
	raw, err := db.db.Get([]byte("secret"), nil)
	assert.NoError(t, err)
	assert.True(t, len(raw) >= 4 && string(raw[:4]) == magicEncrypted,
		"stored data should have LDBE magic prefix")

	// PutWithTtl
	err = db.PutWithTtl(ctx, []byte("ephemeral"), []byte("gone"), 50*time.Millisecond)
	assert.NoError(t, err)

	value, err = db.Get(ctx, []byte("ephemeral"))
	assert.NoError(t, err)
	assert.Equal(t, "gone", string(value))

	time.Sleep(100 * time.Millisecond)
	_, err = db.Get(ctx, []byte("ephemeral"))
	assert.Error(t, err)
}

func TestLevelDB_Encryption_ScanAndCount(t *testing.T) {
	os.RemoveAll("test_encrypt_scan.db")

	db, err := NewLevelDB("test_encrypt_scan.db", WithEncryption(testKey()))
	assert.NoError(t, err)
	defer func() {
		db.Close()
		os.RemoveAll("test_encrypt_scan.db")
	}()

	ctx := context.Background()

	for i := 1; i <= 5; i++ {
		k := []byte(fmt.Sprintf("item:%d", i))
		v := []byte(fmt.Sprintf("value_%d", i))
		err = db.Put(ctx, k, v)
		assert.NoError(t, err)
	}

	result, err := db.Scan(ctx, []byte("item:"), func(k string, v []byte) bool { return true })
	assert.NoError(t, err)
	assert.Len(t, result, 5)

	count, err := db.Count(ctx, []byte("item:"), func(k string, v []byte) bool { return true })
	assert.NoError(t, err)
	assert.Equal(t, int64(5), count)
}

func TestLevelDB_Encryption_WrongKey(t *testing.T) {
	os.RemoveAll("test_wrongkey.db")

	// Write with key1
	db1, err := NewLevelDB("test_wrongkey.db", WithEncryption(testKey()))
	assert.NoError(t, err)
	err = db1.Put(context.Background(), []byte("k"), []byte("v"))
	assert.NoError(t, err)
	db1.Close()

	// Read with key2 should fail
	db2, err := NewLevelDB("test_wrongkey.db", WithEncryption(testKey2()))
	assert.NoError(t, err)
	defer func() {
		db2.Close()
		os.RemoveAll("test_wrongkey.db")
	}()

	_, err = db2.Get(context.Background(), []byte("k"))
	assert.Error(t, err)
}

func TestLevelDB_Encryption_BackwardCompat(t *testing.T) {
	os.RemoveAll("test_backcompat.db")

	// Phase 1: write without encryption
	db1, err := NewLevelDB("test_backcompat.db")
	assert.NoError(t, err)
	err = db1.Put(context.Background(), []byte("plain"), []byte("data"))
	assert.NoError(t, err)
	db1.Close()

	// Phase 2: reopen with encryption, read old plaintext data
	db2, err := NewLevelDB("test_backcompat.db", WithEncryption(testKey()))
	assert.NoError(t, err)
	defer func() {
		db2.Close()
		os.RemoveAll("test_backcompat.db")
	}()

	value, err := db2.Get(context.Background(), []byte("plain"))
	assert.NoError(t, err)
	assert.Equal(t, "data", string(value))

	// Write new data with encryption
	err = db2.Put(context.Background(), []byte("secret"), []byte("encrypted"))
	assert.NoError(t, err)

	// Both old and new data readable
	value, err = db2.Get(context.Background(), []byte("plain"))
	assert.NoError(t, err)
	assert.Equal(t, "data", string(value))

	value, err = db2.Get(context.Background(), []byte("secret"))
	assert.NoError(t, err)
	assert.Equal(t, "encrypted", string(value))
}

func TestLevelDB_WithEncryption_InvalidKey(t *testing.T) {
	os.RemoveAll("test_invalid_key0.db")
	os.RemoveAll("test_invalid_key1.db")
	os.RemoveAll("test_invalid_key2.db")
	defer func() {
		os.RemoveAll("test_invalid_key0.db")
		os.RemoveAll("test_invalid_key1.db")
		os.RemoveAll("test_invalid_key2.db")
	}()

	// Empty key should panic
	assert.Panics(t, func() {
		NewLevelDB("test_invalid_key0.db", WithEncryption(nil))
	}, "should panic on empty key")

	// Short key is fine (hashed via SHA-256)
	db, err := NewLevelDB("test_invalid_key1.db", WithEncryption([]byte("short")))
	assert.NoError(t, err)
	db.Close()

	// Long key is fine (hashed via SHA-256)
	db, err = NewLevelDB("test_invalid_key2.db", WithEncryption(make([]byte, 100)))
	assert.NoError(t, err)
	db.Close()
}
