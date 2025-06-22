package cache

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
}

func NewRedis(client *redis.Client) *Redis {
	return &Redis{
		client: client,
	}
}

func (r *Redis) Raw() *redis.Client {
	return r.client
}

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
			err,
			"[REDIS] failed to set new value in `HSETEX`",
		)
	}

	return nil
}

func (r *Redis) HGet(
	ctx context.Context,
	mainKey string,
	childKey string,
) (string, yaerrors.Error) {
	result, err := r.client.HGet(ctx, mainKey, childKey).Result()
	if err != nil {
		return "", yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("[REDIS] failed to get value by `%s:%s`", mainKey, childKey),
		)
	}

	return result, nil
}

func (r *Redis) HGetAll(
	ctx context.Context,
	mainKey string,
) (map[string]string, yaerrors.Error) {
	result, err := r.client.HGetAll(ctx, mainKey).Result()
	if err != nil {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("[REDIS] failed to get map values by `%s`", mainKey),
		)
	}

	return result, nil
}

func (r *Redis) HGetDelSingle(
	ctx context.Context,
	mainKey string,
	childKey string,
) (string, yaerrors.Error) {
	result, err := r.client.HGetDel(ctx, mainKey, childKey).Result()
	if err != nil {
		return "", yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("[REDIS] failed to get and delete value by `%s:%s`", mainKey, childKey),
		)
	}

	if len(result) == 0 {
		return "", yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("[REDIS] got empty value by `%s:%s`", mainKey, childKey),
		)
	}

	return result[0], nil
}

func (r *Redis) HLen(
	ctx context.Context,
	mainKey string,
) (int64, yaerrors.Error) {
	result, err := r.client.HLen(ctx, mainKey).Result()
	if err != nil {
		return 0, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("[REDIS] failed to get len values by `%s`", mainKey),
		)
	}

	return result, nil
}

func (r *Redis) HExist(
	ctx context.Context,
	mainKey string,
	childKey string,
) (bool, yaerrors.Error) {
	result, err := r.client.HExists(ctx, mainKey, childKey).Result()
	if err != nil {
		return result, yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("[REDIS] failed to get exists value by `%s:%s`", mainKey, childKey),
		)
	}

	return result, nil
}

func (r *Redis) HDelSingle(
	ctx context.Context,
	mainKey string,
	childKey string,
) yaerrors.Error {
	_, err := r.client.HDel(ctx, mainKey, childKey).Result()
	if err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("[REDIS] failed to delete value by `%s:%s`", mainKey, childKey),
		)
	}

	return nil
}

func (r *Redis) Ping(ctx context.Context) yaerrors.Error {
	if err := r.client.Ping(ctx).Err(); err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"[REDIS] failed to get `PONG`",
		)
	}

	return nil
}

func (r *Redis) Close() yaerrors.Error {
	if err := r.client.Close(); err != nil {
		return yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			"[REDIS] failed to close connection",
		)
	}

	return nil
}
