package logger

import (
	"testing"
)

func TestLoggerWrap(t *testing.T) {
	// 初始化控制台日志记录器
	InitConsoleLogger()

	// 获取logger实例
	log := GetLogger()

	// 创建一个带name字段的logger
	serviceLogger := log.Wrap("TestService").(ILogger).Wrap("TestService2")
	if l, ok := serviceLogger.(ILogger); ok {
		// 测试不同级别的日志
		l.Info("这是一条信息日志")
		l.Debug("这是一条调试日志")
		l.Warn("这是一条警告日志")
		l.Error("这是一条错误日志")

		// 使用格式化方法
		l.Infof("这是一条带参数的信息日志：%s", "test")
	}
}
