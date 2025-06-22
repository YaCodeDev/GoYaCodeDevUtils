package cache

import (
	"context"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/redis/go-redis/v9"
)

type Cache[T Container] interface {
	//
	Raw() T

	HSetEX(
		ctx context.Context,
		mainKey string,
		childKey string,
		value string,
		ttl time.Duration,
	) yaerrors.Error

	HGet(
		ctx context.Context,
		mainKey string,
		childKey string,
	) (string, yaerrors.Error)

	HGetAll(
		ctx context.Context,
		mainKey string,
	) (map[string]string, yaerrors.Error)

	HGetDelSingle(
		ctx context.Context,
		mainKey string,
		childKey string,
	) (string, yaerrors.Error)

	HLen(
		ctx context.Context,
		mainKey string,
	) (int64, yaerrors.Error)

	HExist(
		ctx context.Context,
		mainKey string,
		childKey string,
	) (bool, yaerrors.Error)

	HDelSingle(
		ctx context.Context,
		mainKey string,
		childKey string,
	) yaerrors.Error

	// Ping checks if the cache service is available
	Ping(ctx context.Context) yaerrors.Error

	// Close closes the cache connection
	Close() yaerrors.Error
}

type Container interface {
	*redis.Client | MemoryContainer
}

func NewCache[T Container](container T) Cache[T] {
	switch _container := any(container).(type) {
	case *redis.Client:
		value, _ := any(NewRedis(_container)).(Cache[T])

		return value
	case MemoryContainer:
		value, _ := any(NewMemory(_container, time.Minute)).(Cache[T])

		return value
	default:
		value, _ := any(NewMemory(NewMemoryContainer(), time.Minute)).(Cache[T])

		return value
	}
}
