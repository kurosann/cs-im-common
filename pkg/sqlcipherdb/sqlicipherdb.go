package sqlcipherdb

import (
	"context"
	"fmt"
	"time"

	"vane-common/pkg/logger"

	gormSqlcipher "vane-common/pkg/sqlcipherdb/devicer"

	"go.uber.org/zap"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// DB 数据库封装，包含 Close 方法
type DB struct {
	*gorm.DB
}

// Config SQLite数据库配置
type Config struct {
	DSN           string // 数据库连接字符串，例如: "data.db"
	EncryptionKey string // 加密密钥
	ShowSql       bool   // 是否显示SQL日志
	OpenWAL       bool   // 是否开启WAL模式
}

// isWAL returns true if the database has WAL mode enabled
func isWAL(db *gorm.DB) bool {
	var mode string
	db.Raw("PRAGMA journal_mode").Scan(&mode)
	return mode == "wal"
}

// NewSqlcipherDB 创建SQLite数据库连接，支持加密
// 使用示例:
//   - 普通数据库: NewSqlcipherDB(Config{DSN: "data.db"})
//   - 加密数据库: NewSqlcipherDB(Config{DSN: "data.db", EncryptionKey: "your-key"})
//
// 注意：
//   - 如果数据库已经加密，必须提供正确的 EncryptionKey
//   - 如果使用错误的密钥打开加密数据库，会返回 "file is not a database" 错误
//   - EncryptionKey 会自动转换为十六进制格式，建议使用足够强度的密码
func NewSqlcipherDB(config Config) (*DB, error) {
	// 创建GORM日志配置
	loggerConfig := &GormLogger{
		Config: gormLogger.Config{
			SlowThreshold: 200 * time.Millisecond,
		},
		ShowSql: config.ShowSql,
	}

	// 打开数据库连接
	// 如果有加密密钥，需要在 DSN 中添加 _pragma_key 参数
	dsn := config.DSN
	if config.EncryptionKey != "" {
		// 将字符串密钥转换为十六进制格式，因为 SQLCipher 要求使用十六进制格式的密钥
		// 使用 _pragma_key 参数设置加密密钥?cipher=sqlcipher&key=x'36366f6e626e5755786d3133303032643635623039343130373264326261343335636137383538313963'&legacy=4
		hexKey := fmt.Sprintf("%x", config.EncryptionKey)
		dsn += fmt.Sprintf("?_pragma_key=x'%s'&_pragma_cipher_page_size=4096", hexKey)
	}
	var nowFunc func() time.Time

	db, err := gorm.Open(gormSqlcipher.Open(dsn), &gorm.Config{
		Logger:                 loggerConfig,
		SkipDefaultTransaction: true,
		NowFunc:                nowFunc,
	})
	if err != nil {
		if config.EncryptionKey != "" {
			return nil, fmt.Errorf("failed to open encrypted sqlite database (DSN: %s): %w", dsn, err)
		}
		return nil, fmt.Errorf("failed to open sqlite database (DSN: %s): %w", dsn, err)
	}

	// 获取底层 sql.DB 对象
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// 配置 SQLite 连接池参数以避免并发锁问题
	// SetMaxOpenConns: SQLite 设置为 1，因为 SQLite 不支持真正的并发写入
	// 设置为 1 可以确保写入操作的顺序性，避免 "database is locked" 错误
	sqlDB.SetMaxOpenConns(1)

	// SetMaxIdleConns: 闲置连接数也设置为 1
	sqlDB.SetMaxIdleConns(1)

	// SetConnMaxLifetime: 连接的最大生命周期，设置较长的时间以减少连接重建
	sqlDB.SetConnMaxLifetime(time.Hour)

	if config.OpenWAL {
		// 启用 WAL 模式 (Write-Ahead Logging)，大幅提升并发读取性能
		// WAL 模式允许同时进行读写操作，减少锁竞争
		err = db.Exec("PRAGMA journal_mode=WAL").Error
		if err != nil {
			logger.Warn(context.Background(), "Failed to enable WAL mode", zap.Error(err))
		}
		err = db.Exec("pragma synchronous=NORMAL").Error
		if err != nil {
			logger.Warn(context.Background(), "Failed to enable WAL mode", zap.Error(err))
		}
	} else {
		err = db.Exec("PRAGMA journal_mode=DELETE").Error
		if err != nil {
			logger.Warn(context.Background(), "Failed to disable WAL mode", zap.Error(err))
		}
		err = db.Exec("pragma synchronous=FULL").Error
		if err != nil {
			logger.Warn(context.Background(), "Failed to enable WAL mode", zap.Error(err))
		}
	}

	// 设置繁忙超时时间为 30 秒，当数据库被锁定时等待 30 秒
	err = db.Exec("PRAGMA busy_timeout=30000").Error
	if err != nil {
		logger.Warn(context.Background(), "Failed to set busy_timeout", zap.Error(err))
	}

	// 设置同步模式为 NORMAL，在性能和数据安全之间取得平衡
	// NORMAL 模式在每次事务提交时将数据写入磁盘，但不需要等待 fsync
	err = db.Exec("PRAGMA synchronous=NORMAL").Error
	if err != nil {
		logger.Warn(context.Background(), "Failed to set synchronous mode", zap.Error(err))
	}

	// 启用外键约束
	err = db.Exec("PRAGMA foreign_keys=ON").Error
	if err != nil {
		logger.Warn(context.Background(), "Failed to enable foreign keys", zap.Error(err))
	}

	return &DB{DB: db}, nil
}

// Close 关闭数据库连接，释放资源
// 如果数据库开启了 WAL 模式，会先执行 checkpoint 将数据合并回主数据库
func (db *DB) Close() error {
	if db.DB == nil {
		return nil
	}

	// 如果开启了 WAL 模式，先执行 checkpoint 将 WAL 文件的数据合并回主数据库
	if isWAL(db.DB) {
		if err := db.Exec("PRAGMA wal_checkpoint(TRUNCATE)").Error; err != nil {
			logger.Warn(context.Background(), "Failed to checkpoint WAL before close", zap.Error(err))
		}
	}

	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}
	return sqlDB.Close()
}

// GormLogger GORM日志适配器
type GormLogger struct {
	gormLogger.Config
	ShowSql bool
}

func (l *GormLogger) LogMode(level gormLogger.LogLevel) gormLogger.Interface {
	l.LogLevel = level
	return l
}

func (l *GormLogger) Info(ctx context.Context, s string, data ...any) {
	logger.Infof(ctx, s, data...)
}

func (l *GormLogger) Warn(ctx context.Context, s string, data ...any) {
	logger.Warnf(ctx, s, data...)
}

func (l *GormLogger) Error(ctx context.Context, s string, data ...any) {
	logger.Errorf(ctx, s, data...)
}

func (l *GormLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, rows := fc()
	since := time.Since(begin)
	if since > l.SlowThreshold && l.SlowThreshold != 0 {
		if rows == -1 {
			logger.Warn(
				ctx,
				"slow sql",
				zap.String("sql", sql),
				zap.String("since", since.String()),
			)
		} else {
			logger.Warn(
				ctx,
				"slow sql",
				zap.String("sql", sql),
				zap.String("since", since.String()),
				zap.Int64("rows", rows),
			)
		}
	} else if l.ShowSql {
		logger.Debug(ctx, "sql", zap.String("sql", sql), zap.String("since", since.String()), zap.Int64("rows", rows))
	}
}
