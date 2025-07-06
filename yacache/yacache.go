// Package cache provides a generic, pluggable key–value cache abstraction with two
// concrete back‑ends: an in-memory map protected by a RW‑mutex and a Redis hash‑map
// wrapper.  Both back-ends expose the same high-level API so that callers can switch
// implementations without changing their business logic.
//
// The public API is intentionally kept small and focused on hash‑like semantics in
// order to cover 90 % of typical caching use‑cases (session stores, idempotency
// keys, short‑lived tokens, etc.) while still being easy to reason about and test.
//
// # Generic design
//
// The package is written using Go 1.22 generics.  The [Cache] interface is
// parameterised by a single type parameter T constrained to either *redis.Client or
// MemoryContainer.  This allows the concrete implementation to expose its raw
// driver value via [Cache.Raw] without resorting to unsafe type assertions.
//
// # Thread‑safety
//
//   - [Redis] is as thread‑safe as the underlying go‑redis/v9 client.
//   - [Memory] uses a sync.RWMutex to protect all reads/writes.  Long‑running calls
//     such as the background TTL sweeper acquire the mutex only for short, bounded
//     periods.
//
// # Error handling
//
// All methods return the custom yaerrors.Error type so that callers get
// stack‑traces and HTTP status codes for free.  The helper wrappers translate
// driver‑specific errors into this common representation.
//
// # Time‑to‑live (TTL)
//
// The Redis back‑end relies on the HSETEX command and therefore delegates TTL
// handling to Redis.  The memory back‑end stores the absolute expiry timestamp in
// each [memoryCacheItem] and relies on a background [Memory.cleanup] goroutine to
// evict expired entries.
//
// ─────────────────────────────────────────────────────────────────────────────
// # Quick start (in-memory)
//
// ```go
// memory := cache.NewCache(cache.NewMemoryContainer())
// ctx := context.Background()
// _   = memory.HSetEX(ctx, "u:42", "token", "abc", time.Minute)
// value, _ := memory.HGet(ctx, "u:42", "token")
// fmt.Println(value) // "abc"
// ```
//
// # Quick start (Redis)
//
// ```go
// client := cache.NewRedisClient("localhost", uint16(6379), "", 1, log)
// redis := cache.NewCache(client)
// ctx   := context.Background()
// _     = redis.HSetEX(ctx, "jobs", "id1", "yacodder", 0)
// job, _ := redis.HGetDelSingle(ctx, "jobs", "id1")
// fmt.Println(job) // "yacodder"
// ```
// ─────────────────────────────────────────────────────────────────────────────
package yacache

import (
	"context"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/redis/go-redis/v9"
)

// Cache is a generic, hash‑oriented cache abstraction.
//
// The type parameter T must satisfy [Container] and is used by [Cache.Raw] to
// return the underlying low‑level client (*redis.Client or MemoryContainer).
//
// The API surface mirrors a subset of Redis hash commands (HSETEX, HGET, etc.)
// because this data‑model maps well to most caching scenarios while still keeping
// the implementation portable across back‑ends.
//
// All write‑operations use copy‑semantics – the value is cloned into an internal
// buffer.  Callers are therefore safe to mutate the slice/struct after the method
// returns.
//
// Each method returns a yaerrors.Error instead of the built‑in error so that the
// caller can propagate HTTP status codes and stack‑traces up the call‑stack.
type Cache[T Container] interface {
	// Raw exposes the concrete client.  Use this for advanced operations that are
	// outside the scope of the high‑level API – e.g., Lua scripts on Redis or a
	// full clone of the in‑memory map for debugging.
	//
	// Example:
	//
	// 	client := c.Raw() // *redis.Client when Redis backend is active
	Raw() T

	// HSetEX sets (childKey,value) under mainKey and assigns a TTL.  If the key
	// already exists its value is overwritten and the TTL is refreshed.
	//
	// Example:
	//
	// 	_ = c.HSetEX(ctx, "sessions", "token", "abc", time.Minute)
	HSetEX(
		ctx context.Context,
		mainKey string,
		childKey string,
		value string,
		ttl time.Duration,
	) yaerrors.Error

	// HGet fetches a single field from the hash.  If the pair does not exist
	// (either the mainKey or childKey is missing) a yaerrors.Error with HTTP 500 is
	// returned.
	//
	// Example:
	//
	// 	value, _ := c.HGet(ctx, "sessions", "token")
	HGet(
		ctx context.Context,
		mainKey string,
		childKey string,
	) (string, yaerrors.Error)

	// HGetAll returns a shallow copy of the hash (childKey→value).  The internal
	// bookkeeping key YaMapLen is filtered out automatically.
	//
	// Example:
	//
	// 	values, _ := c.HGetAll(ctx, "sessions")
	HGetAll(
		ctx context.Context,
		mainKey string,
	) (map[string]string, yaerrors.Error)

	// HGetDelSingle is an atomic *read‑and‑delete* helper.  It returns the value
	// that was stored under childKey and then deletes exactly that field.  If the
	// resulting hash becomes empty the Redis backend will leave an empty hash
	// while the memory backend deletes the entire map to free memory.
	//
	// Example:
	//
	// 	value, _ := c.HGetDelSingle(ctx, "jobs", "yacodder")
	HGetDelSingle(
		ctx context.Context,
		mainKey string,
		childKey string,
	) (string, yaerrors.Error)

	// HLen returns the number of *user* fields in the hash (YaMapLen is excluded).
	//
	// Example:
	//
	// 	hlen, _ := c.HLen(ctx, "sessions")
	HLen(
		ctx context.Context,
		mainKey string,
	) (int64, yaerrors.Error)

	// HExist answers whether the specific childKey exists in the hash.
	//
	// Example:
	//
	// 	ok, _ := c.HExist(ctx, "sessions", "token")
	HExist(
		ctx context.Context,
		mainKey string,
		childKey string,
	) (bool, yaerrors.Error)

	// HDelSingle deletes exactly one field from the hash.
	//
	// Example:
	//
	// 	_ = c.HDelSingle(ctx, "sessions", "token")
	HDelSingle(
		ctx context.Context,
		mainKey string,
		childKey string,
	) yaerrors.Error

	// Set stores key → value and applies a TTL.
	// A zero `ttl` means “store indefinitely”.
	//
	// Example:
	//
	//	ctx := context.Background()
	//	ttl := 10 * time.Minute
	//	_   = c.Set(ctx, "access-token", "abc123", ttl)
	Set(
		ctx context.Context,
		key string,
		value string,
		ttl time.Duration,
	) yaerrors.Error

	// Get retrieves the value previously saved under key.
	// If the key is missing, an implementation-specific yaerrors.Error is returned.
	//
	// Example:
	//
	//	ctx   := context.Background()
	//	token, _ := c.Get(ctx, "access-token")
	Get(
		ctx context.Context,
		key string,
	) (string, yaerrors.Error)

	// MGet fetches the values for the specified keys from the cache.
	//
	// It returns a map where each key is mapped to its corresponding value.
	// If any of the keys are missing or the operation fails,
	// it returns an error of type ErrFailedToMGetValues, allowing callers to
	// rely on the atomicity of the operation — it's all or nothing.
	//
	// Example:
	//
	//	ctx := context.Background()
	//	values, err := cache.MGet(ctx, "k1", "k2", "k3")
	//	if err != nil {
	//	    log.Fatalf("failed to fetch keys: %v", err)
	//	}
	//	for k, v := range values {
	//	    if v != nil {
	//	        fmt.Printf("%s = %s\n", k, *v)
	//	    } else {
	//	        fmt.Printf("%s = <nil>\n", k)
	//	    }
	//	}
	//
	// Returns:
	//   - map[string]*string: a map of keys to their string values (or nil if not found)
	//   - yaerrors.Error: a wrapped error indicating failure
	MGet(
		ctx context.Context,
		keys ...string,
	) (map[string]*string, yaerrors.Error)

	// GetDel atomically reads **and then deletes** key.
	// Useful for one-shot tokens or queue semantics.
	//
	// Example:
	//
	//	ctx   := context.Background()
	//	token, _ := c.GetDel(ctx, "single-use-token")
	//	fmt.Println(token) // "abc"
	GetDel(
		ctx context.Context,
		key string,
	) (string, yaerrors.Error)

	// Exists reports whether the specified key is currently present in the cache.
	//
	// For in-memory caches (like Memory), an item is considered present even if expired,
	// until it is purged by the background sweeper. Therefore, the presence check may
	// return true for expired but not yet swept items.
	//
	// Example:
	//
	//	ctx := context.Background()
	//	ok, err := cache.Exists(ctx, "access-token")
	//	if err != nil {
	//	    log.Fatalf("exists check failed: %v", err)
	//	}
	//	if !ok {
	//	    // key not found or expired and already purged
	//	    handleMissing()
	//	}
	//
	// Returns:
	//   - bool: true if the key exists (possibly expired but not swept), false otherwise
	//   - yaerrors.Error: non-nil if an error occurred during the check
	Exists(
		ctx context.Context,
		key ...string,
	) (bool, yaerrors.Error)

	// Del unconditionally removes key from the cache.
	// The operation is idempotent: deleting a non-existent key is not an error.
	//
	// Example:
	//
	//	ctx := context.Background()
	//	_   = c.Del(ctx, "obsolete-token")
	Del(
		ctx context.Context,
		key string,
	) yaerrors.Error

	// Ping verifies that the cache service is reachable and healthy.
	//
	// Example:
	//
	// 	_ = c.Ping(ctx)
	Ping(ctx context.Context) yaerrors.Error

	// Close flushes buffers and releases resources.
	//
	// Example:
	//
	// 	_ = c.Close()
	Close() yaerrors.Error
}

// Container is the union (via type-set) of all back‑end client types the generic
// cache can wrap.  Add new back‑ends by extending this constraint and updating
// NewCache accordingly.
type Container interface {
	*redis.Client | MemoryContainer
}

// NewCache performs a *runtime* type‑switch on the supplied container to create
// the appropriate concrete implementation.  When an unsupported type is
// provided a fallback in‑memory cache with a default 1‑minute sweep interval is
// returned so that callers never get a nil value.
//
// Example:
//
// MEMORY
//
//	memory := cache.NewCache(cache.NewMemoryContainer())
//
// REDIS
//
//	client := cache.NewRedisClient("localhost", uint16(6379), "", 1, log)
//	redis := cache.NewCache(client)
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
