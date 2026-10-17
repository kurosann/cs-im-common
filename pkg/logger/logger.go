package logger

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	logger            *Logger
	internalLogger    *zap.Logger
	loggerDebugWriter *LoggerWriter
	loggerInfoWriter  *LoggerWriter
	loggerWarnWriter  *LoggerWriter
	loggerErrorWriter *LoggerWriter
	callerInfoFn      func(ctx context.Context) map[string]any
)

func GetInternalLogger() *zap.Logger {
	return internalLogger
}
func RegisterGetCallerInfo(getInfo func(ctx context.Context) map[string]any) {
	callerInfoFn = getInfo
}

func RegisterLoggerWriter(name string, level zapcore.Level, writer io.Writer) {
	switch level {
	case zapcore.DebugLevel:
		loggerDebugWriter.Add(name, writer)
	case zapcore.InfoLevel:
		loggerInfoWriter.Add(name, writer)
	case zapcore.WarnLevel:
		loggerWarnWriter.Add(name, writer)
	default:
		loggerErrorWriter.Add(name, writer)
	}
}

func UnregisterLoggerWriter(name string, level zapcore.Level) {
	switch level {
	case zapcore.DebugLevel:
		loggerDebugWriter.Remove(name)
	case zapcore.InfoLevel:
		loggerInfoWriter.Remove(name)
	case zapcore.WarnLevel:
		loggerWarnWriter.Remove(name)
	default:
		loggerWarnWriter.Remove(name)
	}
}

type LoggerWriter struct {
	zapcore.WriteSyncer
	registerWriter sync.Map
}

func NewLoggerWriter(writer io.Writer) *LoggerWriter {
	return &LoggerWriter{WriteSyncer: zapcore.AddSync(writer)}
}
func (w *LoggerWriter) Add(name string, writer io.Writer) {
	if w == nil {
		return
	}
	w.registerWriter.Store(name, writer)
}
func (w *LoggerWriter) Remove(name string) (io.Writer, bool) {
	if w == nil {
		return nil, false
	}
	value, loaded := w.registerWriter.LoadAndDelete(name)
	if !loaded {
		return nil, false
	}
	return value.(io.Writer), true
}

func (w *LoggerWriter) Write(p []byte) (n int, err error) {
	w.registerWriter.Range(func(key any, value any) bool {
		if writer, ok := value.(io.Writer); ok {
			_, _ = writer.Write(p)
		}
		return true
	})
	return w.WriteSyncer.Write(p)
}

func init() {
	checkLogger()
}

func checkLogger() {
	if logger == nil {
		logger = newLogger(LogConfig{
			Level:         zap.DebugLevel,
			HideConsole:   false,
			ConsoleEncode: false,
		})
	}
}

type FormaterLogger interface {
	Infof(template string, args ...interface{})
	Debugf(template string, args ...interface{})
	Warnf(template string, args ...interface{})
	Errorf(template string, args ...interface{})
	Panicf(template string, args ...interface{})
}

type StdLogger interface {
	Info(msg string, args ...zap.Field)
	Debug(msg string, args ...zap.Field)
	Warn(msg string, args ...zap.Field)
	Error(msg string, args ...zap.Field)
	Panic(msg string, args ...zap.Field)
	Fatal(msg string, args ...zap.Field)
}

type ILogger interface {
	FormaterLogger
	StdLogger
	Wrap(name string) any
}

func GetLogger() *Logger {
	return logger
}

func getContextFields(ctx context.Context) []zap.Field {
	var fields []zap.Field
	if callerInfoFn != nil {
		fields = append(fields, zap.Any("caller", callerInfoFn(ctx)))
	}
	return fields
}

func Infof(ctx context.Context, template string, args ...interface{}) {
	logger._logger.Info(fmt.Sprintf(template, args...), getContextFields(ctx)...)
}
func Debugf(ctx context.Context, template string, args ...interface{}) {
	logger._logger.Debug(fmt.Sprintf(template, args...), getContextFields(ctx)...)
}
func Warnf(ctx context.Context, template string, args ...interface{}) {
	logger._logger.Warn(fmt.Sprintf(template, args...), getContextFields(ctx)...)
}
func Errorf(ctx context.Context, template string, args ...interface{}) {
	logger._logger.Error(fmt.Sprintf(template, args...), getContextFields(ctx)...)
}
func Panicf(ctx context.Context, template string, args ...interface{}) {
	logger._logger.Panic(fmt.Sprintf(template, args...), getContextFields(ctx)...)
}
func Info(ctx context.Context, msg string, args ...zap.Field) {
	logger._logger.Info(msg, append(args, getContextFields(ctx)...)...)
}
func Debug(ctx context.Context, msg string, args ...zap.Field) {
	logger._logger.Debug(msg, append(args, getContextFields(ctx)...)...)
}
func Warn(ctx context.Context, msg string, args ...zap.Field) {
	logger._logger.Warn(msg, append(args, getContextFields(ctx)...)...)
}
func Error(ctx context.Context, msg string, args ...zap.Field) {
	logger._logger.Error(msg, append(args, getContextFields(ctx)...)...)
}
func Panic(ctx context.Context, msg string, args ...zap.Field) {
	logger._logger.Panic(msg, append(args, getContextFields(ctx)...)...)
}
func Fatal(ctx context.Context, msg string, args ...zap.Field) {
	logger._logger.Fatal(msg, append(args, getContextFields(ctx)...)...)
}

// Sync 同步流数据
//
//	写入文件时不是立刻就写入的，若要停止程序，需要在停止程序前调用，以保证日志可以完全写入到文件中
func Sync() {
	_ = logger._logger.Sync()
}

type LogConfig struct {
	Name          string        `yaml:"name"`
	LogPath       string        `yaml:"log_path"`
	Level         zapcore.Level `yaml:"level"`
	MaxSize       int           `yaml:"max_size"`
	MaxBackup     int           `yaml:"max_backup"`
	MaxAge        int           `yaml:"max_age"`
	HideConsole   bool          `yaml:"hide_console"`
	ConsoleEncode bool          `yaml:"console_encode"`
}

func InitLogger(logConfig LogConfig) {
	logger = newLogger(logConfig)
}

func InitConsoleLogger() {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = zapcore.TimeEncoderOfLayout("2006/01/02-15:04:05.000")
	config.EncodeLevel = zapcore.CapitalColorLevelEncoder
	config.EncodeCaller = nil
	consoleCore := zapcore.NewCore(zapcore.NewConsoleEncoder(config), os.Stdout, zap.DebugLevel)
	internalLogger = zap.New(consoleCore, zap.AddCaller())
	logger = &Logger{
		_logger: internalLogger.WithOptions(zap.AddCallerSkip(1)),
	}
}
func InitTestLogger() {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = zapcore.TimeEncoderOfLayout("2006/01/02-15:04:05.000")
	config.EncodeLevel = zapcore.CapitalColorLevelEncoder
	consoleCore := zapcore.NewCore(zapcore.NewConsoleEncoder(config), os.Stdout, zap.DebugLevel)
	internalLogger = zap.New(consoleCore, zap.AddCaller())
	logger = &Logger{
		_logger: internalLogger.WithOptions(zap.AddCallerSkip(1)),
	}
}

type Logger struct {
	_logger *zap.Logger
}

func newLogger(logConfig LogConfig) *Logger {
	config := zap.NewProductionEncoderConfig()
	config.EncodeTime = zapcore.TimeEncoderOfLayout("2006/01/02-15:04:05.000")

	var encoder zapcore.Encoder
	if logConfig.ConsoleEncode {
		config.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(config)
	} else {
		encoder = zapcore.NewJSONEncoder(config)
	}
	var cores []zapcore.Core
	if !logConfig.HideConsole {
		cores = append(cores, zapcore.NewCore(encoder, os.Stdout, logConfig.Level))
	} else {
		if logConfig.LogPath != "" {
			errFile, err := os.OpenFile(filepath.Join(logConfig.LogPath, logConfig.Name+"_stderr.log"), os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
			if err != nil {
				panic(err)
			}
			_, _ = errFile.WriteString(fmt.Sprintf("==========================> pid: %d <========================== %s\n", os.Getpid(), time.Now().Format(time.DateTime)))
			outFile, err := os.OpenFile(filepath.Join(logConfig.LogPath, logConfig.Name+"_stdout.log"), os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
			if err != nil {
				panic(err)
			}
			_, _ = outFile.WriteString(fmt.Sprintf("==========================> pid: %d <========================== %s\n", os.Getpid(), time.Now().Format(time.DateTime)))
			redirectStderr(outFile, errFile)
		}
	}
	if logConfig.LogPath != "" {
		_ = os.Mkdir(logConfig.LogPath, 0755)
		cores = append(cores,
			debugFileCore(encoder, logConfig.LogPath, logConfig.Name),
			infoFileCore(encoder, logConfig.LogPath, logConfig.Name),
			errorFileCore(encoder, logConfig.LogPath, logConfig.Name),
			warnFileCore(encoder, logConfig.LogPath, logConfig.Name),
		)
	}
	core := zapcore.NewTee(cores...)
	internalLogger = zap.New(core, zap.AddCaller())
	return &Logger{
		_logger: internalLogger.WithOptions(zap.AddCallerSkip(1)),
	}
}
func infoFileCore(consoleEncoder zapcore.Encoder, logPath, filename string) zapcore.Core {
	loggerInfoWriter = NewLoggerWriter(&lumberjack.Logger{
		Filename:   filepath.Join(logPath, filename+"_info.log"),
		MaxSize:    10,
		MaxBackups: 7,
		MaxAge:     7,
		Compress:   true,
	})
	return zapcore.NewCore(consoleEncoder,
		loggerInfoWriter,
		zap.LevelEnablerFunc(func(level zapcore.Level) bool {
			return level == zapcore.InfoLevel
		}))
}
func debugFileCore(consoleEncoder zapcore.Encoder, logPath, filename string) zapcore.Core {
	loggerDebugWriter = NewLoggerWriter(&lumberjack.Logger{
		Filename:   filepath.Join(logPath, filename+"_debug.log"),
		MaxSize:    10,
		MaxBackups: 7,
		MaxAge:     7,
		Compress:   true,
	})
	return zapcore.NewCore(consoleEncoder,
		loggerDebugWriter,
		zap.LevelEnablerFunc(func(level zapcore.Level) bool {
			return level == zapcore.DebugLevel
		}))
}
func errorFileCore(consoleEncoder zapcore.Encoder, logPath, filename string) zapcore.Core {
	loggerErrorWriter = NewLoggerWriter(&lumberjack.Logger{
		Filename:   filepath.Join(logPath, filename+"_error.log"),
		MaxSize:    10,
		MaxBackups: 7,
		MaxAge:     7,
		Compress:   true,
	})
	return zapcore.NewCore(consoleEncoder,
		loggerErrorWriter,
		zap.LevelEnablerFunc(func(level zapcore.Level) bool {
			return level >= zapcore.ErrorLevel
		}))
}

func warnFileCore(consoleEncoder zapcore.Encoder, logPath, filename string) zapcore.Core {
	loggerWarnWriter = NewLoggerWriter(&lumberjack.Logger{
		Filename:   filepath.Join(logPath, filename+"_warn.log"),
		MaxSize:    10,
		MaxBackups: 7,
		MaxAge:     7,
		Compress:   true,
	})
	return zapcore.NewCore(consoleEncoder,
		loggerWarnWriter,
		zap.LevelEnablerFunc(func(level zapcore.Level) bool {
			return level == zapcore.WarnLevel
		}))
}
func (l *Logger) Wrap(name string) any {
	n := *l
	n._logger = l._logger.With(zap.Namespace(name))
	return &n
}
func (l *Logger) Info(msg string, args ...zap.Field) {
	l._logger.Info(msg, args...)
}
func (l *Logger) Debug(msg string, args ...zap.Field) {
	l._logger.Debug(msg, args...)
}
func (l *Logger) Warn(msg string, args ...zap.Field) {
	l._logger.Warn(msg, args...)
}
func (l *Logger) Error(msg string, args ...zap.Field) {
	l._logger.Error(msg, args...)
}
func (l *Logger) Panic(msg string, args ...zap.Field) {
	l._logger.Panic(msg, args...)
}
func (l *Logger) Fatal(msg string, args ...zap.Field) {
	l._logger.Fatal(msg, args...)
}
func (l *Logger) Infof(template string, args ...interface{}) {
	l._logger.Info(fmt.Sprintf(template, args...))
}
func (l *Logger) Debugf(template string, args ...interface{}) {
	l._logger.Debug(fmt.Sprintf(template, args...))
}
func (l *Logger) Warnf(template string, args ...interface{}) {
	l._logger.Warn(fmt.Sprintf(template, args...))
}
func (l *Logger) Errorf(template string, args ...interface{}) {
	l._logger.Error(fmt.Sprintf(template, args...))
}
func (l *Logger) Panicf(template string, args ...interface{}) {
	l._logger.Panic(fmt.Sprintf(template, args...))
}
