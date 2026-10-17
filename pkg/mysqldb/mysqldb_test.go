package mysqldb

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	gormLogger "gorm.io/gorm/logger"
)

func TestSplitDSN(t *testing.T) {
	tests := []struct {
		name     string
		dsn      string
		expected *dsnParts
	}{
		{
			name: "完整DSN",
			dsn:  "user:password@tcp(127.0.0.1:3306)/mydb?charset=utf8mb4&parseTime=True",
			expected: &dsnParts{
				userInfo: "user:password",
				protocol: "tcp(",
				host:     "127.0.0.1:3306",
				db:       "mydb",
				params:   "charset=utf8mb4&parseTime=True",
			},
		},
		{
			name: "DSN without protocol",
			dsn:  "user:password@127.0.0.1:3306/mydb",
			expected: &dsnParts{
				userInfo: "user:password",
				protocol: "",
				host:     "127.0.0.1:3306",
				db:       "mydb",
				params:   "",
			},
		},
		{
			name: "DSN without user",
			dsn:  "tcp(127.0.0.1:3306)/mydb",
			expected: &dsnParts{
				userInfo: "",
				protocol: "tcp(",
				host:     "127.0.0.1:3306",
				db:       "mydb",
				params:   "",
			},
		},
		{
			name: "DSN without params",
			dsn:  "user:password@tcp(127.0.0.1:3306)/mydb",
			expected: &dsnParts{
				userInfo: "user:password",
				protocol: "tcp(",
				host:     "127.0.0.1:3306",
				db:       "mydb",
				params:   "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitDSN(tt.dsn)
			assert.Equal(t, tt.expected.userInfo, result.userInfo)
			assert.Equal(t, tt.expected.protocol, result.protocol)
			assert.Equal(t, tt.expected.host, result.host)
			assert.Equal(t, tt.expected.db, result.db)
			assert.Equal(t, tt.expected.params, result.params)
		})
	}
}

func TestRebuildDSNWithNamespace(t *testing.T) {
	tests := []struct {
		name      string
		dsn       string
		namespace string
		expected  string
	}{
		{
			name:      "添加命名空间",
			dsn:       "user:password@tcp(127.0.0.1:3306)/mydb?charset=utf8mb4",
			namespace: "dev",
			expected:  "user:password@tcp(127.0.0.1:3306)/mydb_dev?charset=utf8mb4",
		},
		{
			name:      "空命名空间",
			dsn:       "user:password@tcp(127.0.0.1:3306)/mydb",
			namespace: "",
			expected:  "user:password@tcp(127.0.0.1:3306)/mydb",
		},
		{
			name:      "无数据库名",
			dsn:       "user:password@tcp(127.0.0.1:3306)",
			namespace: "dev",
			expected:  "user:password@tcp(127.0.0.1:3306)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rebuildDSNWithNamespace(tt.dsn, tt.namespace)
			assert.Equal(t, tt.expected, result)
		})
	}
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
