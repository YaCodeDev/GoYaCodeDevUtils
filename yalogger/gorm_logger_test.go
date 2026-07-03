package yalogger

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func TestNewGormLoggerDefaults(t *testing.T) {
	logger, _ := NewGormLogger(NewBaseLogger(nil).NewLogger(), nil).(*compactGormLogger)

	if logger.level != gormlogger.Info {
		t.Fatalf("expected info level, got %v", logger.level)
	}

	if logger.slowThreshold != DefaultGormSlowQueryThreshold {
		t.Fatalf("expected default slow threshold, got %s", logger.slowThreshold)
	}

	if !logger.ignoreRecordNotFound {
		t.Fatal("expected record-not-found errors to be ignored by default")
	}
}

func TestGormLoggerLogMode(t *testing.T) {
	base := NewGormLogger(NewBaseLogger(nil).NewLogger(), nil).(*compactGormLogger)
	copyLogger, ok := base.LogMode(gormlogger.Warn).(*compactGormLogger)
	if !ok {
		t.Fatal("expected compact gorm logger")
	}

	if base == copyLogger {
		t.Fatal("expected LogMode to return a copy")
	}

	if copyLogger.level != gormlogger.Warn {
		t.Fatalf("expected warn level, got %v", copyLogger.level)
	}
}

func TestCompactGormLine(t *testing.T) {
	got := compactGormLine(" SELECT   *\n  FROM users \t WHERE id = 1 ")
	if got != "SELECT * FROM users WHERE id = 1" {
		t.Fatalf("unexpected compact output: %q", got)
	}
}

func TestGormLoggerInfoWarnAndError(t *testing.T) {
	log, buffer := newBufferedLogger(TraceLevel)
	logger := NewGormLogger(log, nil)

	logger.Info(context.Background(), "table %s", "users")
	logger.Warn(context.Background(), "slow %s", "query")
	logger.Error(context.Background(), "failed %s", "query")

	output := buffer.String()
	for _, expected := range []string{
		"[GORM] INFO: table users",
		"[GORM] WARN: slow query",
		"[GORM] ERROR: failed query",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got %q", expected, output)
		}
	}
}

func TestGormLoggerTraceBranches(t *testing.T) {
	log, buffer := newBufferedLogger(TraceLevel)
	ctxLog := log.WithField(KeyRequestID, "req-1")
	ctx := ContextWithLogger(context.Background(), ctxLog)
	logger := NewGormLogger(log, nil).(*compactGormLogger)

	logger.Trace(
		ctx,
		time.Now().Add(-50*time.Millisecond),
		func() (string, int64) {
			return "SELECT  *\nFROM users", 1
		},
		nil,
	)

	logger.Trace(
		ctx,
		time.Now().Add(-(DefaultGormSlowQueryThreshold + time.Millisecond)),
		func() (string, int64) {
			return "SELECT  *\nFROM users WHERE active = 1", 2
		},
		nil,
	)

	logger.Trace(
		ctx,
		time.Now().Add(-time.Millisecond),
		func() (string, int64) {
			return "SELECT * FROM users WHERE id = 1", 0
		},
		gorm.ErrRecordNotFound,
	)

	logger.Trace(
		ctx,
		time.Now().Add(-time.Millisecond),
		func() (string, int64) {
			return "SELECT * FROM users WHERE id = 2", 0
		},
		errors.New("boom"),
	)

	output := buffer.String()
	for _, expected := range []string{
		"[GORM] RUN elapsed=",
		"[GORM] SLOW SQL elapsed=",
		"[GORM] SQL ERROR elapsed=",
		"sql=SELECT * FROM users",
		"request_id=req-1",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("expected output to contain %q, got %q", expected, output)
		}
	}
}

func TestGormLoggerTraceSilentAndRecordNotFoundConfig(t *testing.T) {
	log, buffer := newBufferedLogger(TraceLevel)

	silent := NewGormLogger(log, &GormLoggerConfig{
		Level:                gormlogger.Silent,
		SlowThreshold:        DefaultGormSlowQueryThreshold,
		IgnoreRecordNotFound: true,
	}).(*compactGormLogger)
	silent.Trace(
		context.Background(),
		time.Now().Add(-time.Second),
		func() (string, int64) {
			return "SELECT * FROM users", 1
		},
		errors.New("boom"),
	)

	if buffer.Len() != 0 {
		t.Fatalf("expected silent logger to skip output, got %q", buffer.String())
	}

	recordNotFoundLogger := NewGormLogger(log, &GormLoggerConfig{
		Level:                gormlogger.Error,
		SlowThreshold:        DefaultGormSlowQueryThreshold,
		IgnoreRecordNotFound: false,
	}).(*compactGormLogger)
	recordNotFoundLogger.Trace(
		context.Background(),
		time.Now().Add(-time.Millisecond),
		func() (string, int64) {
			return "SELECT * FROM users WHERE id = 1", 0
		},
		gorm.ErrRecordNotFound,
	)

	if !strings.Contains(buffer.String(), "[GORM] SQL ERROR") {
		t.Fatalf("expected record-not-found error to be logged, got %q", buffer.String())
	}
}
