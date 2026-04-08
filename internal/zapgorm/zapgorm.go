// Package zapgorm provides a GORM logger that writes through a zap.Logger.
// This ensures all GORM SQL and errors appear in the same log sinks
// (file, stderr) as the rest of the application, rather than on stdout
// via the default GORM logger.
package zapgorm

import (
	"context"
	"errors"
	"time"

	gormlogger "gorm.io/gorm/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Logger struct {
	zap               *zap.Logger
	level             gormlogger.LogLevel
	slowThreshold     time.Duration
	skipNotFound      bool
}

// New returns a GORM logger backed by zap.
// slowThreshold controls when a query is logged as slow (0 = always log at debug).
func New(z *zap.Logger, level gormlogger.LogLevel, slowThreshold time.Duration) gormlogger.Interface {
	return &Logger{
		zap:           z.WithOptions(zap.AddCallerSkip(3)),
		level:         level,
		slowThreshold: slowThreshold,
		skipNotFound:  true,
	}
}

func (l *Logger) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	c := *l
	c.level = level
	return &c
}

func (l *Logger) Info(ctx context.Context, msg string, args ...interface{}) {
	if l.level >= gormlogger.Info {
		l.zap.Sugar().Infof(msg, args...)
	}
}

func (l *Logger) Warn(ctx context.Context, msg string, args ...interface{}) {
	if l.level >= gormlogger.Warn {
		l.zap.Sugar().Warnf(msg, args...)
	}
}

func (l *Logger) Error(ctx context.Context, msg string, args ...interface{}) {
	if l.level >= gormlogger.Error {
		l.zap.Sugar().Errorf(msg, args...)
	}
}

func (l *Logger) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if l.level <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	fields := []zapcore.Field{
		zap.Duration("elapsed", elapsed),
		zap.Int64("rows", rows),
		zap.String("sql", sql),
	}

	switch {
	case err != nil && !(l.skipNotFound && errors.Is(err, gormlogger.ErrRecordNotFound)):
		l.zap.Error("gorm: query error", append(fields, zap.Error(err))...)

	case l.slowThreshold > 0 && elapsed > l.slowThreshold:
		l.zap.Warn("gorm: slow query", fields...)

	case l.level >= gormlogger.Info:
		l.zap.Debug("gorm: query", fields...)
	}
}
