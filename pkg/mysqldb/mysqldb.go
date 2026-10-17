package mysqldb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"vane-common/pkg/clock"
	"vane-common/pkg/logger"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

// Config MySQL数据库配置
type Config struct {
	DSN string // 数据库连接字符串，例如: "user:password@tcp(localhost:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
	// 示例 DSN 格式:
	// - 用户名:密码@tcp(主机:端口)/数据库名?参数
	// - user:password@tcp(127.0.0.1:3306)/mydb?charset=utf8mb4&parseTime=True&loc=Local
	Namespace string // 数据库命名空间，会在数据库名后添加后缀
	ShowSql   bool   // 是否显示SQL日志
}

// NewMysqlDB 创建MySQL数据库连接
// 使用示例:
//   - 普通数据库: NewMysqlDB(Config{DSN: "user:password@tcp(localhost:3306)/dbname"})
//   - 带命名空间: NewMysqlDB(Config{DSN: "user:password@tcp(localhost:3306)/dbname", Namespace: "dev"})
func NewMysqlDB(config Config) (*gorm.DB, error) {
	// 重新构建 DSN，添加命名空间
	rebuiltDSN := config.DSN
	if config.Namespace != "" {
		rebuiltDSN = rebuildDSNWithNamespace(config.DSN, config.Namespace)
	}

	// 先测试连接
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*3)
	defer cancel()

	driver, err := sql.Open("mysql", rebuiltDSN)
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql driver: %w", err)
	}
	defer driver.Close()

	if err = driver.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("connection mysql timeout: %w", err)
	}

	// 创建GORM日志配置
	loggerConfig := &GormLogger{
		Config: gormLogger.Config{
			SlowThreshold: 200 * time.Millisecond,
		},
		ShowSql: config.ShowSql,
	}

	// 连接数据库
	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       rebuiltDSN,
		DefaultStringSize:         256,
		DisableDatetimePrecision:  true,
		DontSupportRenameIndex:    true,
		DontSupportRenameColumn:   true,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{
		Logger: loggerConfig,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open mysql database: %w", err)
	}

	// 获取底层 sql.DB 对象
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// 配置连接池
	sqlDB.SetMaxIdleConns(10) // 空闲连接的最大数量
	// 单数据源上限压到 50：MySQL 服务端 max_connections=151，而一个进程常注册多个数据源
	// （vane-api-server = main + imlog 两个），100/源会让总量直接超过服务端上限，
	// 表现为 Error 1040: Too many connections —— 上报、图片补传整批失败。
	// 20/源时单进程最多 40，给同机多个 vane-api-server 实例和其他业务服务留足余量；
	// 连接不够时 database/sql 会排队等待而不是把 MySQL 的 1040 直接暴露给业务层。
	sqlDB.SetMaxOpenConns(20) // 打开数据库连接的最大数量
	// 空闲连接超过 30s 即关闭：云上 NAT/防火墙会静默断开空闲 TCP，
	// 复用半开连接会导致查询卡在 Sending to client（慢查询假象）。
	sqlDB.SetConnMaxIdleTime(30 * time.Second)
	sqlDB.SetConnMaxLifetime(time.Hour)

	logger.Debug(context.Background(), "MySQL database connected",
		zap.String("namespace", config.Namespace))

	return db, nil
}

// rebuildDSNWithNamespace 重新构建 DSN，在数据库名后添加命名空间
func rebuildDSNWithNamespace(dsn string, namespace string) string {
	parts := splitDSN(dsn)
	if parts == nil {
		return dsn
	}

	dbName := parts.db
	if dbName != "" && namespace != "" {
		dbName = dbName + "_" + namespace
	}

	// 重新构建 DSN
	var rebuilt string
	if parts.userInfo != "" {
		rebuilt = parts.userInfo + "@"
	}

	if parts.protocol != "" {
		rebuilt += parts.protocol + "(" + parts.host + ")"
	} else {
		rebuilt += parts.host
	}

	if dbName != "" {
		rebuilt += "/" + dbName
	}

	if parts.params != "" {
		rebuilt += "?" + parts.params
	}

	logger.Debug(context.Background(), "Rebuild MySQL DSN",
		zap.String("original_dsn", dsn),
		zap.String("rebuilt_dsn", rebuilt),
		zap.String("namespace", namespace),
		zap.String("dbname", dbName))

	return rebuilt
}

// dsnParts DSN 各部分
type dsnParts struct {
	userInfo string
	protocol string
	host     string
	db       string
	params   string
}

// splitDSN 解析 MySQL DSN
// DSN 格式: [username[:password]@][protocol[(address)]]/dbname[?param1=value1&...&paramN=valueN]
func splitDSN(dsn string) *dsnParts {
	var result dsnParts

	// 查找 @ 符号分隔用户信息
	atIdx := findAtSign(dsn)
	if atIdx >= 0 {
		result.userInfo = dsn[:atIdx]
		dsn = dsn[atIdx+1:]
	}

	// 查找协议部分 (tcp 或 unix)
	if len(dsn) > 4 && (dsn[:4] == "tcp(" || dsn[:4] == "unix") {
		protocolEnd := findClosingParen(dsn)
		if protocolEnd > 4 {
			result.protocol = dsn[:4]
			result.host = dsn[5:protocolEnd]
			dsn = dsn[protocolEnd+1:]
		}
	} else {
		// 简单格式，没有协议
		slashIdx := findSlash(dsn)
		if slashIdx > 0 {
			result.host = dsn[:slashIdx]
			dsn = dsn[slashIdx:]
		}
	}

	// 查找数据库名和参数
	if len(dsn) > 0 && dsn[0] == '/' {
		dsn = dsn[1:]
		questionIdx := findQuestionMark(dsn)
		if questionIdx >= 0 {
			result.db = dsn[:questionIdx]
			result.params = dsn[questionIdx+1:]
		} else {
			result.db = dsn
		}
	}

	return &result
}

func findAtSign(s string) int {
	for i, c := range s {
		if c == '@' {
			return i
		}
	}
	return -1
}

func findClosingParen(s string) int {
	for i, c := range s {
		if c == ')' {
			return i
		}
	}
	return -1
}

func findSlash(s string) int {
	for i, c := range s {
		if c == '/' {
			return i
		}
	}
	return -1
}

func findQuestionMark(s string) int {
	for i, c := range s {
		if c == '?' {
			return i
		}
	}
	return -1
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
	since := clock.Since(begin)
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
