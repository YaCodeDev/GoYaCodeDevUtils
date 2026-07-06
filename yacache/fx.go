package yacache

import (
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

// newMemoryCache builds a *Memory cache from the MemoryContainer this module
// provides, sweeping expired entries once a minute — the same default
// NewCache's fallback path already uses when nothing else is specified.
func newMemoryCache(data MemoryContainer) *Memory {
	return NewMemory(data, time.Minute)
}

// MemoryModuleName is the fx module name for the in-memory yacache backend.
const MemoryModuleName = "yacache-memory"

// MemoryModule provides a Cache[MemoryContainer] backed by Memory.
//
// Example usage:
//
//	fx.New(yacache.MemoryModule)
var MemoryModule = fx.Module(
	MemoryModuleName,
	fx.Provide(NewMemoryContainer),
	fx.Provide(fx.Annotate(newMemoryCache, fx.As(new(Cache[MemoryContainer])))),
)

// RedisParams configures the Redis client provided by RedisModule.
type RedisParams struct {
	Host     string
	Port     uint16
	Password string
	DB       int
}

// newRedisClientFromParams dials Redis using RedisParams and the
// yalogger.Logger supplied by the graph. It shares NewRedisClient's existing
// behavior of terminating the process (log.Fatalf) if the initial ping fails.
func newRedisClientFromParams(params RedisParams, log yalogger.Logger) *redis.Client {
	return NewRedisClient(params.Host, params.Port, params.Password, params.DB, log)
}

// RedisModuleName is the fx module name for the Redis yacache backend.
const RedisModuleName = "yacache-redis"

// RedisModule provides a Cache[*redis.Client] backed by Redis.
//
// Example usage:
//
//	fx.New(
//		fx.Supply(yacache.RedisParams{Host: "localhost", Port: 6379}),
//		yalogger.LoggerModule,
//		yacache.RedisModule,
//	)
var RedisModule = fx.Module(
	RedisModuleName,
	fx.Provide(newRedisClientFromParams),
	fx.Provide(fx.Annotate(NewRedis, fx.As(new(Cache[*redis.Client])))),
)
