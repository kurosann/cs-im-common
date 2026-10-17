package sqlitedb

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	gormSqlcipher "vane-common/pkg/sqlcipherdb/devicer"

	"github.com/beevik/ntp"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

func TestNewSqliteDB(t *testing.T) {
	// 创建临时测试目录
	testDir := "test_db"
	os.MkdirAll(testDir, 0755)
	defer os.RemoveAll(testDir)

	t.Run("普通数据库", func(t *testing.T) {
		dbPath := filepath.Join(testDir, "test.db")
		db, err := NewSqliteDB(Config{
			DSN:     dbPath,
			ShowSql: false,
		})
		assert.NoError(t, err)
		assert.NotNil(t, db)

		// 测试基本操作
		type TestTable struct {
			ID   uint `gorm:"primaryKey"`
			Name string
		}

		err = db.AutoMigrate(&TestTable{})
		assert.NoError(t, err)

		test := TestTable{Name: "test"}
		err = db.Create(&test).Error
		assert.NoError(t, err)
		time, err := ntp.Query("pool.ntp.org")
		assert.NoError(t, err)
		fmt.Println(time)

		var result TestTable
		err = db.First(&result, test.ID).Error
		assert.NoError(t, err)
		assert.Equal(t, "test", result.Name)

		err = db.Close()
		assert.NoError(t, err)
	})

	t.Run("加密数据库", func(t *testing.T) {
		dbPath := filepath.Join("", "encrypted.db")

		// 1. 创建加密数据库
		db, err := NewSqliteDB(Config{
			DSN:     dbPath,
			ShowSql: false,
		})
		assert.NoError(t, err, "应该成功创建加密数据库")
		assert.NotNil(t, db)

		// 测试基本操作
		type TestTable struct {
			ID   uint `gorm:"primaryKey"`
			Name string
		}

		err = db.AutoMigrate(&TestTable{})
		assert.NoError(t, err, "应该成功迁移表结构")

		test := TestTable{Name: "encrypted_test"}
		err = db.Create(&test).Error
		assert.NoError(t, err, "应该成功插入数据")

		var result TestTable
		err = db.First(&result, test.ID).Error
		assert.NoError(t, err, "应该成功读取数据")
		assert.Equal(t, "encrypted_test", result.Name)

		err = db.Close()
		assert.NoError(t, err)

		// 2. 测试使用错误密钥无法打开/读取加密数据库
		// 注意：某些 SQLCipher 驱动实现可能在连接时不验证密钥，而是在首次查询时验证
		db2, err := NewSqliteDB(Config{
			DSN:     dbPath,
			ShowSql: false,
		})

		if err == nil && db2 != nil {
			// 如果连接成功（某些驱动延迟验证密钥），尝试读取数据应该失败
			var result2 TestTable
			err2 := db2.First(&result2, test.ID).Error
			assert.Error(t, err2, "使用错误密钥应该无法读取加密数据")
			err = db2.Close()
			assert.NoError(t, err)
		} else {
			// 如果连接直接失败，这也是正确的
			assert.Error(t, err, "使用错误密钥应该无法打开数据库")
		}

		// 3. 测试使用正确密钥应该能够正常打开和读取
		db3, err := NewSqliteDB(Config{
			DSN:     dbPath,
			ShowSql: false,
		})
		assert.NoError(t, err, "使用正确密钥应该能够打开数据库")
		assert.NotNil(t, db3)

		var result3 TestTable
		err = db3.First(&result3, test.ID).Error
		assert.NoError(t, err, "使用正确密钥应该能够读取数据")
		assert.Equal(t, "encrypted_test", result3.Name)

		err = db3.Close()
		assert.NoError(t, err)
	})

	t.Run("加密数据库配置", func(t *testing.T) {
		dbPath := filepath.Join(testDir, "encrypted_config.db")

		db, err := NewSqliteDB(Config{
			DSN:     dbPath,
			ShowSql: false,
		})
		assert.NoError(t, err)
		assert.NotNil(t, db)

		// 测试基本操作
		type TestTable struct {
			ID   uint `gorm:"primaryKey"`
			Name string
		}

		err = db.AutoMigrate(&TestTable{})
		assert.NoError(t, err)

		test := TestTable{Name: "encrypted_config_test"}
		err = db.Create(&test).Error
		assert.NoError(t, err)

		var result TestTable
		err = db.First(&result, test.ID).Error
		assert.NoError(t, err)
		assert.Equal(t, "encrypted_config_test", result.Name)

		err = db.Close()
		assert.NoError(t, err)
	})

	t.Run("检查数据库加密状态", func(t *testing.T) {
		// 测试未加密数据库
		plainPath := filepath.Join(testDir, "plain_check.db")
		db1, err := NewSqliteDB(Config{
			DSN:     plainPath,
			ShowSql: false,
		})
		assert.NoError(t, err)
		err = db1.Close()
		assert.NoError(t, err)

		isEncrypted, err := CheckDatabaseEncryption(plainPath)
		assert.NoError(t, err)
		assert.False(t, isEncrypted, "未加密的数据库应该返回 false")

		// 测试加密数据库
		encryptedPath := filepath.Join(testDir, "encrypted_check.db")
		db2, err := NewSqliteDB(Config{
			DSN:     encryptedPath,
			ShowSql: false,
		})
		assert.NoError(t, err)
		err = db2.Close()
		assert.NoError(t, err)

		isEncrypted, err = CheckDatabaseEncryption(encryptedPath)
		assert.NoError(t, err)
		assert.True(t, isEncrypted, "加密的数据库应该返回 true")
	})
}

func TestGormLogger(t *testing.T) {
	logger := &GormLogger{
		Config: gormLogger.Config{
			SlowThreshold: 100,
		},
		ShowSql: true,
	}

	ctx := context.Background()
	logger.Info(ctx, "test info")
	logger.Warn(ctx, "test warn")
	logger.Error(ctx, "test error")

	// 测试Trace方法
	logger.Trace(ctx, time.Now(), func() (string, int64) {
		return "SELECT * FROM test", 1
	}, nil)
}

// CheckDatabaseEncryption 检查数据库文件是否加密
// 返回值：isEncrypted, error
func CheckDatabaseEncryption(dbPath string) (bool, error) {
	// 尝试不使用密钥打开数据库
	dialector := gormSqlcipher.Open(dbPath)
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: &GormLogger{Config: gormLogger.Config{}},
	})
	if err != nil {
		return false, fmt.Errorf("failed to open database: %w", err)
	}
	defer func() {
		sqlDB, _ := db.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}()

	// 尝试执行一个简单的查询来验证数据库是否可访问
	var result int
	err = db.Raw("SELECT 1").Scan(&result).Error

	// 如果查询成功，说明数据库未加密
	if err == nil {
		return false, nil
	}

	// 如果查询失败且错误信息包含 "file is not a database"，说明是加密的数据库
	errStr := err.Error()
	if contains(errStr, "file is not a database") || contains(errStr, "database is malformed") || contains(errStr, "not a database") {
		return true, nil
	}

	// 其他错误
	return false, err
}

// contains 检查字符串是否包含子串
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
