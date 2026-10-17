package leveldb

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	"vane-common/pkg/clock"
	"vane-common/pkg/dataaccess/store"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/util"
)

const (
	magicEncrypted = "\x4C\x44\x42\x45" // "LDBE" — LevelDB Encrypted
	nonceSize      = 12                 // AES-GCM standard nonce size
)

var _ store.IKVStore = (*LevelDB)(nil)

// Option configures a LevelDB instance.
type Option func(*LevelDB)

// WithEncryption enables AES-256-GCM encryption for stored values.
// key can be any length; the actual AES-256 key is derived via SHA-256(key).
func WithEncryption(key []byte) Option {
	return func(l *LevelDB) {
		if len(key) == 0 {
			panic("leveldb: encryption key must not be empty")
		}
		h := sha256.Sum256(key)
		l.encryptKey = make([]byte, len(h))
		copy(l.encryptKey, h[:])
	}
}

type LevelDB struct {
	db         *leveldb.DB
	encryptKey []byte // nil means no encryption (backward compatible)
}

// encodeValueWithExpire 编码值和过期时间
// 格式: [8 bytes: _expire_at][remaining bytes: value]
func encodeValueWithExpire(value []byte, expireAt int64) []byte {
	result := make([]byte, 8+len(value))
	binary.LittleEndian.PutUint64(result[:8], uint64(expireAt))
	copy(result[8:], value)
	return result
}

// decodeValueWithExpire 解码值和过期时间
// 返回: value, _expire_at
func decodeValueWithExpire(data []byte) ([]byte, int64) {
	if len(data) < 8 {
		return data, 0 // 兼容旧格式，没有过期时间
	}
	expireAt := int64(binary.LittleEndian.Uint64(data[:8]))
	value := data[8:]
	return value, expireAt
}

func NewLevelDB(path string, opts ...Option) (*LevelDB, error) {
	// 优化配置，减少文件数量
	options := &opt.Options{
		// 写缓冲区大小，减少到1MB
		WriteBuffer: 1 * 1024 * 1024,

		// 块大小
		BlockSize: 512 * 1024,

		// 压缩算法
		Compression: opt.SnappyCompression,

		// 缓存大小
		BlockCacheCapacity: 8 * 1024 * 1024, // 8MB

		// 打开文件数限制
		OpenFilesCacheCapacity: 100,
	}

	db, err := leveldb.OpenFile(path, options)
	if err != nil {
		return nil, err
	}

	l := &LevelDB{db: db}
	for _, opt := range opts {
		opt(l)
	}
	return l, nil
}

// encrypt encrypts data with AES-256-GCM. If no key is set, data is returned unchanged.
// Encrypted format: [4 bytes "LDBE" magic][12 bytes nonce][ciphertext]
func (l *LevelDB) encrypt(data []byte) ([]byte, error) {
	if l.encryptKey == nil {
		return data, nil
	}

	block, err := aes.NewCipher(l.encryptKey)
	if err != nil {
		return nil, fmt.Errorf("leveldb: aes.NewCipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("leveldb: cipher.NewGCM: %w", err)
	}

	nonce := make([]byte, nonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("leveldb: generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, data, nil)

	result := make([]byte, 4+nonceSize+len(ciphertext))
	copy(result[:4], magicEncrypted)
	copy(result[4:4+nonceSize], nonce)
	copy(result[4+nonceSize:], ciphertext)
	return result, nil
}

// decrypt decrypts data with AES-256-GCM. If no key is set, or the data lacks
// the "LDBE" magic prefix (old plaintext format), data is returned unchanged.
func (l *LevelDB) decrypt(data []byte) ([]byte, error) {
	if l.encryptKey == nil {
		return data, nil
	}
	if len(data) < 4 || string(data[:4]) != magicEncrypted {
		// Old plaintext format or too short — backward compatible
		return data, nil
	}
	if len(data) < 4+nonceSize {
		return nil, errors.New("leveldb: encrypted data too short")
	}

	block, err := aes.NewCipher(l.encryptKey)
	if err != nil {
		return nil, fmt.Errorf("leveldb: aes.NewCipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("leveldb: cipher.NewGCM: %w", err)
	}

	nonce := data[4 : 4+nonceSize]
	ciphertext := data[4+nonceSize:]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("leveldb: decrypt: %w", err)
	}

	return plaintext, nil
}

func IsNotFound(err error) bool {
	return errors.Is(err, leveldb.ErrNotFound) || errors.Is(err, store.ErrDataNotFound)
}

func (l *LevelDB) IsNotFound(err error) bool {
	return IsNotFound(err)
}

func (l *LevelDB) Close() error {
	l.Compact()
	return l.db.Close()
}

func (l *LevelDB) Del(ctx context.Context, key []byte) (err error) {
	return l.db.Delete(key, nil)
}

func (l *LevelDB) Get(ctx context.Context, key []byte) (value []byte, err error) {
	record, err := l.db.Get(key, nil)
	if err != nil {
		return nil, err
	}

	// 解密（兼容旧格式：未加密或加密密钥为空时自动跳过）
	record, err = l.decrypt(record)
	if err != nil {
		return nil, err
	}

	// 解码值和过期时间
	value, expireAt := decodeValueWithExpire(record)

	// 检查是否过期
	if expireAt > 0 && clock.Now().UnixNano() > expireAt {
		return nil, leveldb.ErrNotFound
	}

	return value, nil
}

func (l *LevelDB) Put(ctx context.Context, key []byte, value []byte) (err error) {
	// 编码值，_expire_at = 0 表示永不过期
	encoded := encodeValueWithExpire(value, 0)
	encrypted, err := l.encrypt(encoded)
	if err != nil {
		return err
	}
	return l.db.Put(key, encrypted, nil)
}

// Compact 手动触发压缩，减少文件数量
func (l *LevelDB) Compact() error {
	return l.db.CompactRange(util.Range{})
}

func (l *LevelDB) Scan(ctx context.Context, prefix []byte, filter store.FilterFunc) (kv map[string][]byte, err error) {
	iter := l.db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()

	kv = make(map[string][]byte)
	currentTime := clock.Now().UnixNano()

	for iter.Next() {
		// 复制迭代器返回的数据，避免被后续迭代修改
		valueData := make([]byte, len(iter.Value()))
		copy(valueData, iter.Value())

		// 解密（兼容旧格式）- 解密失败时跳过该记录，不影响其他记录
		decrypted, decryptErr := l.decrypt(valueData)
		if decryptErr != nil {
			// 解密失败可能是数据损坏或格式不兼容，跳过该记录
			continue
		}

		// 解码值和过期时间
		value, expireAt := decodeValueWithExpire(decrypted)

		// 检查是否过期
		if expireAt > 0 && currentTime > expireAt {
			continue
		}

		// 复制键和值数据
		keyStr := string(iter.Key())
		if filter != nil && !filter(keyStr, value) {
			continue
		}
		resultValue := make([]byte, len(value))
		copy(resultValue, value)
		kv[keyStr] = resultValue
	}
	if err = iter.Error(); err != nil {
		return nil, err
	}

	return kv, nil
}

func (l *LevelDB) Clean(ctx context.Context) (err error) {
	iter := l.db.NewIterator(nil, nil)
	defer iter.Release()

	currentTime := clock.Now().UnixNano()
	var expiredKeys [][]byte

	// 收集过期的键
	for iter.Next() {
		// 复制迭代器返回的数据，避免被后续迭代修改
		valueData := make([]byte, len(iter.Value()))
		copy(valueData, iter.Value())

		// 解密（兼容旧格式）- 解密失败时跳过该记录，不影响其他记录
		decrypted, decryptErr := l.decrypt(valueData)
		if decryptErr != nil {
			// 解密失败可能是数据损坏或格式不兼容，跳过该记录
			continue
		}

		// 解码值和过期时间
		_, expireAt := decodeValueWithExpire(decrypted)

		// 检查是否过期 (_expire_at > 0 表示有过期时间)
		if expireAt > 0 && currentTime > expireAt {
			key := make([]byte, len(iter.Key()))
			copy(key, iter.Key())
			expiredKeys = append(expiredKeys, key)
		}
	}
	if err = iter.Error(); err != nil {
		return err
	}

	// 删除过期的键
	for _, key := range expiredKeys {
		if err = l.db.Delete(key, nil); err != nil {
			return err
		}
	}

	return nil
}

func (l *LevelDB) Count(ctx context.Context, prefix []byte, filter store.FilterFunc) (count int64, err error) {
	iter := l.db.NewIterator(util.BytesPrefix(prefix), nil)
	defer iter.Release()

	count = 0
	currentTime := clock.Now().UnixNano()

	for iter.Next() {
		// 复制迭代器返回的数据，避免被后续迭代修改
		valueData := make([]byte, len(iter.Value()))
		copy(valueData, iter.Value())

		// 解密（兼容旧格式）- 解密失败时跳过该记录，不影响其他记录
		decrypted, decryptErr := l.decrypt(valueData)
		if decryptErr != nil {
			// 解密失败可能是数据损坏或格式不兼容，跳过该记录
			continue
		}

		// 解码值和过期时间
		value, expireAt := decodeValueWithExpire(decrypted)

		// 检查是否过期
		if expireAt > 0 && currentTime > expireAt {
			continue
		}

		if filter != nil && !filter(string(iter.Key()), value) {
			continue
		}

		count++
	}

	if err = iter.Error(); err != nil {
		return 0, err
	}

	return count, nil
}

func (l *LevelDB) PutWithTtl(ctx context.Context, key []byte, value []byte, ttl time.Duration) (err error) {
	expireAt := int64(0)
	if ttl > 0 {
		expireAt = clock.Now().Add(ttl).UnixNano()
	}
	// 编码值和过期时间
	encoded := encodeValueWithExpire(value, expireAt)
	encrypted, err := l.encrypt(encoded)
	if err != nil {
		return err
	}
	return l.db.Put(key, encrypted, nil)
}

func (l *LevelDB) StartTx(ctx context.Context) context.Context {
	// LevelDB 不支持事务，直接返回原始 context
	return ctx
}

func (l *LevelDB) Transaction(ctx context.Context, fn func(ctx context.Context) error) (err error) {
	// LevelDB 不支持事务，直接执行函数
	return fn(ctx)
}
