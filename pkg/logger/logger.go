package logger

import (
	"os"
	"path/filepath"
	"time"

	"github.com/v2v-blockchain/v2v-blockchain/internal/app/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Logger is a wrapper around zap.Logger
type Logger struct {
	*zap.Logger
	sugar *zap.SugaredLogger
}

// New creates a new logger based on configuration
func New(cfg config.LogConfig) (*Logger, error) {
	// Parse log level
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	// Create encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Adjust encoder based on format
	if cfg.Format == "console" {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}

	// Create encoder
	var encoder zapcore.Encoder
	if cfg.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Create write syncer
	var writeSyncer zapcore.WriteSyncer
	if cfg.Output == "file" && cfg.File != "" {
		// Ensure log directory exists
		dir := filepath.Dir(cfg.File)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}

		// Create lumberjack logger for rotation
		lumberjackLogger := &lumberjack.Logger{
			Filename:   cfg.File,
			MaxSize:    cfg.MaxSize,    // megabytes
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAge,     // days
			Compress:   true,
		}
		writeSyncer = zapcore.AddSync(lumberjackLogger)
	} else {
		writeSyncer = zapcore.AddSync(os.Stdout)
	}

	// Create core
	core := zapcore.NewCore(encoder, writeSyncer, level)

	// Create logger
	zapLogger := zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	return &Logger{
		Logger: zapLogger,
		sugar:  zapLogger.Sugar(),
	}, nil
}

// NewDevelopment creates a development logger
func NewDevelopment() (*Logger, error) {
	zapLogger, err := zap.NewDevelopment()
	if err != nil {
		return nil, err
	}
	return &Logger{
		Logger: zapLogger,
		sugar:  zapLogger.Sugar(),
	}, nil
}

// NewProduction creates a production logger
func NewProduction() (*Logger, error) {
	zapLogger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	return &Logger{
		Logger: zapLogger,
		sugar:  zapLogger.Sugar(),
	}, nil
}

// Sugar returns the sugared logger
func (l *Logger) Sugar() *zap.SugaredLogger {
	return l.sugar
}

// With returns a logger with the given fields
func (l *Logger) With(fields ...zap.Field) *Logger {
	return &Logger{
		Logger: l.Logger.With(fields...),
		sugar:  l.sugar.With(l.fieldsToArgs(fields)...),
	}
}

// fieldsToArgs converts zap.Field slice to interface{} slice
func (l *Logger) fieldsToArgs(fields []zap.Field) []interface{} {
	args := make([]interface{}, len(fields))
	for i, f := range fields {
		args[i] = f
	}
	return args
}

// Named returns a named logger
func (l *Logger) Named(name string) *Logger {
	return &Logger{
		Logger: l.Logger.Named(name),
		sugar:  l.sugar.Named(name),
	}
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}

// Debug logs a debug message
func (l *Logger) Debug(msg string, fields ...zap.Field) {
	l.Logger.Debug(msg, fields...)
}

// Info logs an info message
func (l *Logger) Info(msg string, fields ...zap.Field) {
	l.Logger.Info(msg, fields...)
}

// Warn logs a warning message
func (l *Logger) Warn(msg string, fields ...zap.Field) {
	l.Logger.Warn(msg, fields...)
}

// Error logs an error message
func (l *Logger) Error(msg string, fields ...zap.Field) {
	l.Logger.Error(msg, fields...)
}

// Fatal logs a fatal message and exits
func (l *Logger) Fatal(msg string, fields ...zap.Field) {
	l.Logger.Fatal(msg, fields...)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(template string, args ...interface{}) {
	l.sugar.Debugf(template, args...)
}

// Infof logs a formatted info message
func (l *Logger) Infof(template string, args ...interface{}) {
	l.sugar.Infof(template, args...)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(template string, args ...interface{}) {
	l.sugar.Warnf(template, args...)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(template string, args ...interface{}) {
	l.sugar.Errorf(template, args...)
}

// Fatalf logs a formatted fatal message and exits
func (l *Logger) Fatalf(template string, args ...interface{}) {
	l.sugar.Fatalf(template, args...)
}

// WithError adds an error field to the logger
func (l *Logger) WithError(err error) *Logger {
	return l.With(ErrField(err))
}

// WithField adds a field to the logger
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return l.With(zap.Any(key, value))
}

// WithFields adds multiple fields to the logger
func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	zapFields := make([]zap.Field, 0, len(fields))
	for k, v := range fields {
		zapFields = append(zapFields, zap.Any(k, v))
	}
	return l.With(zapFields...)
}

// Field helpers
func String(key, val string) zap.Field {
	return zap.String(key, val)
}

func Int(key string, val int) zap.Field {
	return zap.Int(key, val)
}

func Int64(key string, val int64) zap.Field {
	return zap.Int64(key, val)
}

func Uint64(key string, val uint64) zap.Field {
	return zap.Uint64(key, val)
}

func Bool(key string, val bool) zap.Field {
	return zap.Bool(key, val)
}

func Float64(key string, val float64) zap.Field {
	return zap.Float64(key, val)
}

func ErrField(err error) zap.Field {
	return zap.Error(err)
}

func Any(key string, val interface{}) zap.Field {
	return zap.Any(key, val)
}

func ByteString(key string, val []byte) zap.Field {
	return zap.ByteString(key, val)
}

func Duration(key string, val time.Duration) zap.Field {
	return zap.Duration(key, val)
}

// Default logger instance
var defaultLogger *Logger

// InitDefault initializes the default logger
func InitDefault(cfg config.LogConfig) error {
	logger, err := New(cfg)
	if err != nil {
		return err
	}
	defaultLogger = logger
	return nil
}

// GetDefault returns the default logger
func GetDefault() *Logger {
	if defaultLogger == nil {
		logger, _ := NewProduction()
		defaultLogger = logger
	}
	return defaultLogger
}

// SetDefault sets the default logger
func SetDefault(logger *Logger) {
	defaultLogger = logger
}

// Package-level functions for convenience
func Debug(msg string, fields ...zap.Field) {
	GetDefault().Debug(msg, fields...)
}

func Info(msg string, fields ...zap.Field) {
	GetDefault().Info(msg, fields...)
}

func Warn(msg string, fields ...zap.Field) {
	GetDefault().Warn(msg, fields...)
}

func Error(msg string, fields ...zap.Field) {
	GetDefault().Error(msg, fields...)
}

func Fatal(msg string, fields ...zap.Field) {
	GetDefault().Fatal(msg, fields...)
}

func Debugf(template string, args ...interface{}) {
	GetDefault().Debugf(template, args...)
}

func Infof(template string, args ...interface{}) {
	GetDefault().Infof(template, args...)
}

func Warnf(template string, args ...interface{}) {
	GetDefault().Warnf(template, args...)
}

func Errorf(template string, args ...interface{}) {
	GetDefault().Errorf(template, args...)
}

func Fatalf(template string, args ...interface{}) {
	GetDefault().Fatalf(template, args...)
}
