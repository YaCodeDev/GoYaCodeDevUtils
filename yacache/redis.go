package yacache

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/redis/go-redis/v9"
)

// Redis wraps a *redis.Client and implements the Cache interface.
//
// It intentionally exposes only the subset of commands used by the
// in-memory implementation, so that your business-layer code can switch
// between Redis and Memory without `// +build` tags or extra plumbing.
//
// # Typical usage
//
// ```go
// client := cache.NewRedisClient("localhost", uint16(6379), "", 1, log)
// redis := cache.NewCache(client)
// ctx   := context.Background()
// _     = redis.HSetEX(ctx, "jobs", "id1", "yacodder", 0)
// job, _ := redis.HGetDelSingle(ctx, "jobs", "id1")
// fmt.Println(job) // "yacodder"
// ```
type Redis struct {
	client *redis.Client
}

// NewRedis turns an already-configured *redis.Client into a **Redis** cache.
//
// Use it when the application creates the low-level client itself
// (e.g. your DI container, connection pool manager, or tests).
//
// Example:
//
// client := cache.NewRedisClient("localhost", uint16(6379), "", 1, log)
// redis := cache.NewCache(client)
// _ = cache.Ping(context.Background())
func NewRedis(client *redis.Client) *Redis {
	return &Redis{
		client: client,
	}
}

// NewRedisClient dials a real Redis instance and performs an initial PING.
//
// It logs both the connection attempt and the final status via the
// supplied yalogger.Logger. On failure the logger’s Fatalf terminates
// the process, mirroring the standard library’s `log.Fatalf` semantics.
//
// Example:
//
//	client := cache.NewRedisClient("127.0.0.1", 6379, "", 0, log)
func NewRedisClient(
	host string,
	port uint16,
	password string,
	db int,
	log yalogger.Logger,
) *redis.Client {
	redisAddr := fmt.Sprintf("%s:%s", host, strconv.Itoa(int(port)))

	if log == nil {
		log = yalogger.NewBaseLogger(nil).NewLogger()
	}

	log.Infof("Redis connecting to addr %s", redisAddr)

	client := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: password,
		DB:       db,
		Network:  "tcp4",
	})

	if err := client.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Failed to connect redis: %v", err)
	}

	log.Infof("Redis connected to addr %s", redisAddr)

	return client
}

// Raw exposes the underlying *redis.Client so that advanced commands
// (e.g. Lua scripts, pipelines) can still be reached when absolutely
// necessary. Prefer the high-level helpers when possible.
//
// Example:
//
//	if err := r.Raw().FlushDB(ctx).Err(); err != nil { … }
func (r *Redis) Raw() *redis.Client {
	return r.client
}

// HSetEX stores field → value under mainKey with an absolute TTL.
//
// Internally it uses Redis 7.0 `HSETEX` command (via go-redis helper).
//
// Example:
//
//	ttl := 10 * time.Second
//	_ = redis.HSetEX(ctx, "session:token", "userID", "42", ttl)
func (r *Redis) HSetEX(
	ctx context.Context,
	mainKey string,
	childKey string,
	value string,
	ttl time.Duration,
) yaerrors.Error {
	if err := r.client.HSetEXWithArgs(
		ctx,
		mainKey,
		&redis.HSetEXOptions{
			ExpirationType: redis.HSetEXExpirationEX,
			ExpirationVal:  int64(ttl.Seconds()),
		},
		childKey,
		value,
	).Err(); err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToHSetEx),
			"[REDIS] failed `HSETEX`",
		)
	}

	return nil
}

// HGet returns the value previously stored by HSetEX.
//
// Returns an error if the key/field pair is missing.
//
// Example:
//
//	value, err := redis.HGet(ctx, "session:token", "userID")
//	if err != nil { … }
func (r *Redis) HGet(
	ctx context.Context,
	mainKey string,
	childKey string,
) (string, yaerrors.Error) {
	result, err := r.client.HGet(ctx, mainKey, childKey).Result()
	if err != nil {
		return "", yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToGetValue),
			fmt.Sprintf("[REDIS] failed `HGET` by `%s:%s`", mainKey, childKey),
		)
	}

	return result, nil
}

// HGetAll fetches the entire hash under mainKey.
//
// Example:
//
//	values, _ := redis.HGetAll(ctx, "user:42")
//	for key, value := range values {
//	    fmt.Printf("%s = %s\n", key, value)
//	}
func (r *Redis) HGetAll(
	ctx context.Context,
	mainKey string,
) (map[string]string, yaerrors.Error) {
	result, err := r.client.HGetAll(ctx, mainKey).Result()
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToGetValues),
			fmt.Sprintf("[REDIS] failed `HGETALL` by `%s`", mainKey),
		)
	}

	return result, nil
}

// HGetDelSingle atomically retrieves *and* deletes one field.
//
// Useful for queue-like semantics without Lua scripting.
//
// Example:
//
//	value, _ := redis.HGetDelSingle(ctx, "jobs:ready", "job123")
//	// job123 is now removed from the hash
func (r *Redis) HGetDelSingle(
	ctx context.Context,
	mainKey string,
	childKey string,
) (string, yaerrors.Error) {
	result, err := r.client.HGetDel(ctx, mainKey, childKey).Result()
	if err != nil {
		return "", yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToGetDeleteSingle),
			fmt.Sprintf("[REDIS] failed `HGETDEL` by `%s:%s`", mainKey, childKey),
		)
	}

	if len(result) == 0 {
		return "", yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(err, ErrNotFoundValue),
			fmt.Sprintf("[REDIS] not found value by `%s:%s`", mainKey, childKey),
		)
	}

	return result[0], nil
}

// HLen reports how many fields a hash contains.
//
// Example:
//
//	hlen, _ := redis.HLen(ctx, "cart:42")
//	fmt.Println("items in cart:", hlen)
func (r *Redis) HLen(
	ctx context.Context,
	mainKey string,
) (int64, yaerrors.Error) {
	result, err := r.client.HLen(ctx, mainKey).Result()
	if err != nil {
		return 0, yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToGetLen),
			fmt.Sprintf("[REDIS] failed `HLEN` by `%s`", mainKey),
		)
	}

	return result, nil
}

// HExist tells whether a particular field exists.
//
// Example:
//
//	ok, _ := redis.HExist(ctx, "user:42", "email")
//	if !ok { … }
func (r *Redis) HExist(
	ctx context.Context,
	mainKey string,
	childKey string,
) (bool, yaerrors.Error) {
	result, err := r.client.HExists(ctx, mainKey, childKey).Result()
	if err != nil {
		return result, yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToHExist),
			fmt.Sprintf("[REDIS] failed `HEXIST` by `%s:%s`", mainKey, childKey),
		)
	}

	return result, nil
}

// HDelSingle removes one field from the hash.
//
// Example:
//
//	_ = redis.HDelSingle(ctx, "cart:42", "item:99")
func (r *Redis) HDelSingle(
	ctx context.Context,
	mainKey string,
	childKey string,
) yaerrors.Error {
	if err := r.client.HDel(ctx, mainKey, childKey).Err(); err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToDeleteSingle),
			fmt.Sprintf("[REDIS] failed `HDEL` by `%s:%s`", mainKey, childKey),
		)
	}

	return nil
}

func (r *Redis) Set(
	ctx context.Context,
	key string,
	value string,
	ttl time.Duration,
) yaerrors.Error {
	if err := r.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToSet),
			fmt.Sprintf("[REDIS] failed `SET` by `%s`", key),
		)
	}

	return nil
}

func (r *Redis) Get(
	ctx context.Context,
	key string,
) (string, yaerrors.Error) {
	value, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return "", yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToGetValue),
			fmt.Sprintf("[REDIS] failed `GET` by `%s`", key),
		)
	}

	return value, nil
}

func (r *Redis) MGet(
	ctx context.Context,
	keys ...string,
) (map[string]string, yaerrors.Error) {
	values, err := r.client.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToMGetValues),
			fmt.Sprintf("[REDIS] failed `MGET` in `%s`", keys),
		)
	}

	if len(values) != len(keys) {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			ErrFailedToMGetValues,
			fmt.Sprintf("[REDIS] values count: %d in `MGET` doesn't equal to keys count: %d", len(values), len(keys)),
		)
	}

	result := make(map[string]string)

	for i, key := range keys {
		value, ok := values[i].(string)
		if !ok {
			return nil, yaerrors.FromError(
				http.StatusInternalServerError,
				ErrFailedToMGetValues,
				fmt.Sprintf("[REDIS] value in `MGET` doesn't compare to string type: %v", values[i]),
			)
		}

		result[key] = value
	}

	return result, nil
}

func (r *Redis) Exists(
	ctx context.Context,
	key string,
) (bool, yaerrors.Error) {
	if err := r.client.Exists(ctx, key).Err(); err != nil {
		return false, yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToExists),
			fmt.Sprintf("[REDIS] failed `Exists` by `%s`", key),
		)
	}

	return true, nil
}

func (r *Redis) Del(
	ctx context.Context,
	key string,
) yaerrors.Error {
	if err := r.client.Del(ctx, key).Err(); err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToDelValue),
			fmt.Sprintf("[REDIS] failed `DEL` by `%s`", key),
		)
	}

	return nil
}

func (r *Redis) GetDel(
	ctx context.Context,
	key string,
) (string, yaerrors.Error) {
	value, err := r.client.GetDel(ctx, key).Result()
	if err != nil {
		return "", yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToGetDelValue),
			fmt.Sprintf("[REDIS] failed `DEL` by `%s`", key),
		)
	}

	return value, nil
}

// Ping sends the Redis PING command.
//
// It is called by unit tests to guarantee that NewCache(client)
// returns a live service.
//
// Example:
//
//	if err := r.Ping(ctx); err != nil { … }
func (r *Redis) Ping(ctx context.Context) yaerrors.Error {
	if err := r.client.Ping(ctx).Err(); err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedPing),
			"[REDIS] failed `PING`",
		)
	}

	return nil
}

// Close closes the underlying connection(s). Always call it in `defer`
// when you created the *redis.Client* yourself.
//
// Example:
//
//	redis := cache.NewRedis(rdb)
//	defer redis.Close()
func (r *Redis) Close() yaerrors.Error {
	if err := r.client.Close(); err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			errors.Join(err, ErrFailedToCloseBackend),
			"[REDIS] failed `CLOSE`",
		)
	}

	return nil
}
