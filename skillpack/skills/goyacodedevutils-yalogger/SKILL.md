---
name: goyacodedevutils-yalogger
description: Structured, logrus-backed Logger interface threaded through nearly every GoYaCodeDevUtils package, plus context helpers and GORM/Gin adapters that route those frameworks' output through the same Logger. Use as the standard logger type instead of the stdlib log package or a raw logrus.Logger.
---

# yalogger Skill

Import path: `github.com/YaCodeDev/GoYaCodeDevUtils/yalogger`.

Structured logging interface (logrus-backed) used as the standard `Logger` type threaded through nearly
every other package in this repo.

## Key API

- `Logger` interface — `Info`/`Infof`, `Trace`/`Tracef`, `Error`/`Errorf`, `Warn`/`Warnf`, `Debug`/`Debugf`, `Fatal`/`Fatalf`, `Panic`/`Panicf`; `WithField`/`WithFields`/`WithRequestStringID`/`WithRequestUUID`/`WithRequestID`/`WithRandomRequestID`/`WithSystemRequestID`/`WithUserID`; `GetFields`/`GetField`/`MergeFields`/`DeleteField`.
- `BaseLogger` interface — `NewLogger() Logger`.
- `NewBaseLogger(config *Config) BaseLogger` — `config == nil` uses sane defaults (Logrus backend, `TraceLevel`, no timestamp).
- `Config` struct — `BaseLoggerType`, `Level`, `FullTimestamp`, `DisableTimestamp`, `TimestampFormat`.
- `Level` (uint8) — `PanicLevel` .. `TraceLevel`, implements `Unmarshal`/`UnmarshalText` for config-tag parsing.
- `BaseLoggerType` (uint8) — `const Logrus`.
- `const KeyRequestID`, `KeySystemRequestID`, `KeyUserID`.

## Context Helpers

- `ContextWithLogger(ctx context.Context, log Logger) context.Context` — stores `log` in `ctx` (tolerates a nil `ctx`; nil `log` returns `ctx` unchanged).
- `LoggerFromContext(ctx context.Context, fallback Logger) Logger` — returns the stored `Logger` merged over `fallback` (context fields win); returns `fallback` when the context holds none.

## GORM Adapter

- `NewGormLogger(log Logger, config *GormLoggerConfig) gormlogger.Interface` — implements `gorm.io/gorm/logger`.Interface; pass to `gorm.Config{Logger: ...}`. `config == nil` uses defaults (Info level, `DefaultGormSlowQueryThreshold`, record-not-found errors ignored). Resolves a request-scoped logger per call via `LoggerFromContext`.
- `GormLoggerConfig` struct — `Level` (`gormlogger.LogLevel`), `SlowThreshold` (`time.Duration`), `IgnoreRecordNotFound` (bool).
- `const DefaultGormSlowQueryThreshold = 200 * time.Millisecond`.
- Level mapping: GORM Info → `Debug`, Warn → `Warn`, Error → `Error`; per-statement `Trace` logs failures at `Error`, slow queries at `Warn`, everything else at `Debug`. SQL is compacted to one line.

## Gin Helpers

- `ConfigureGinDebugLogging(log Logger)` — routes Gin's global debug/route/error writers through `log` (no-op on nil `log`).
- `SetGinContextLogger(ctx *gin.Context, log Logger, config *GinLoggerConfig)` — stores the request logger in the Gin context and (unless `DisableRequestContext`) the request's `context.Context`.
- `GinLoggerFromContext(ctx *gin.Context, fallback Logger, config *GinLoggerConfig) Logger` — retrieves that logger merged over `fallback`.
- `GinAccessLogger(log Logger, config *GinLoggerConfig) gin.HandlerFunc` — one compact access line per request; 5xx → `Error`, 4xx → `Warn`, else `Debug`.
- `GinRecovery(log Logger, config *GinLoggerConfig) gin.HandlerFunc` — recovers panics, logs at `Error`, aborts 500 (adds a compacted stack trace in Gin debug mode).
- `GinLoggerConfig` struct — `ContextKey` (string, defaults to `GinContextLoggerKey`), `DisableRequestContext` (bool).
- `const GinContextLoggerKey = "ContextLogger"`.

## Usage Notes

- Only the Logrus backend is implemented; `NewBaseLogger` panics on an unsupported `BaseLoggerType`.
- `WithField`/`WithX` methods return a **new** `Logger` (immutable-style chaining) — they do not mutate the receiver.
- Leaf package: no dependency on other repo packages. Nearly everything else in this repo depends on it, both for logging and as a parameter to `yaerrors` `*WithLog` constructors.
- The GORM and Gin adapters bring in `gorm.io/gorm` and `github.com/gin-gonic/gin` as external deps; the core `Logger` itself stays framework-free.
- All adapters accept a nil `*...Config` and fall back to sane defaults, mirroring `NewBaseLogger`.
