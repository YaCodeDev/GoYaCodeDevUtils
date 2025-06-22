package yacache_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yacache"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRedis(t *testing.T) (*redis.Client, func()) {
	mr, err := miniredis.Run()

	require.NoError(t, err)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	cleanup := func() {
		client.Close()
		mr.Close()
	}

	return client, cleanup
}

func TestRedisCacheService(t *testing.T) {
	client, cleanup := setupTestRedis(t)
	defer cleanup()

	redis := yacache.NewRedis(client)

	ctx := context.Background()

	t.Parallel()

	redis.Raw().HSet(ctx, yamainKey, yachildKey, yavalue)

	t.Run("[HGet] - get value works", func(t *testing.T) {
		value, _ := redis.HGet(ctx, yamainKey, yachildKey)

		assert.Equal(t, yavalue, value)
	})

	t.Run("[HLen] - get len works", func(t *testing.T) {
		hlen, _ := redis.HLen(context.Background(), yamainKey)

		expected := int64(1)

		assert.Equal(t, expected, hlen)
	})

	t.Run("[HGetAll] - get len works", func(t *testing.T) {
		expected := make(map[string]string)

		expected[yachildKey] = yavalue

		for i := range 10 {
			redis.Raw().HSet(
				ctx,
				yamainKey,
				fmt.Sprintf("%s:%d", yachildKey, i),
				fmt.Sprintf("%s:%d", yavalue, i),
			)

			expected[fmt.Sprintf("%s:%d", yachildKey, i)] = fmt.Sprintf("%s:%d", yavalue, i)
		}

		hlen, _ := redis.HGetAll(ctx, yamainKey)

		assert.Equal(t, expected, hlen)
	})

	t.Run("[HDelSingle] - delete item works", func(t *testing.T) {
		deleteMainKey := yamainKey + ":delete_test"
		deleteChildKey := yachildKey + ":delete_test"
		deleteValue := yavalue + ":delete_test"

		redis.Raw().HSet(ctx, deleteMainKey, deleteChildKey, deleteValue)

		oldLen, _ := redis.HLen(ctx, deleteMainKey)

		_ = redis.HDelSingle(ctx, deleteMainKey, deleteChildKey)

		t.Run("[HDelSingle] - not exists works", func(t *testing.T) {
			exist, _ := redis.HExist(ctx, deleteMainKey, deleteChildKey)

			expected := false

			assert.Equal(t, exist, expected)
		})

		t.Run("[HDelSingle] - decrement len works", func(t *testing.T) {
			hlen, _ := redis.HLen(ctx, deleteMainKey)

			expected := oldLen - 1

			assert.Equal(t, expected, hlen)
		})
	})
}
