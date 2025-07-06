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

	_ = redis.Set(ctx, yamainKey2, yavalue2, yattl)

	t.Run("[Set] - set value works", func(t *testing.T) {
		value, _ := redis.Raw().Get(ctx, yamainKey2).Result()

		assert.Equal(t, yavalue2, value)
	})

	t.Run("[Get] - get value works", func(t *testing.T) {
		value, _ := redis.Get(ctx, yamainKey2)

		assert.Equal(t, yavalue2, value)
	})

	t.Run("[MGet] - multi get values works", func(t *testing.T) {
		expected := make(map[string]*string)

		yavaluePtr := yavalue2
		expected[yachildKey] = &yavaluePtr

		var keys []string

		for i := range 10 {
			keys = append(keys, fmt.Sprintf("%s:%d", yamainKey2, i))

			err := redis.Set(
				ctx,
				keys[len(keys)-1],
				fmt.Sprintf("%s:%d", yavalue2, i),
				yattl,
			)
			if err != nil {
				panic(err)
			}

			yavaluePtr := fmt.Sprintf("%s:%d", yavalue2, i)
			expected[keys[len(keys)-1]] = &yavaluePtr
		}

		keys = append(keys, "key_which_doesnt_contains___))")
		result, _ := redis.MGet(ctx, keys...)

		for _, key := range keys {
			assert.Equal(t, expected[key], result[key])
		}
	})

	t.Run("[GetDel] - get and delete value works", func(t *testing.T) {
		key := yamainKey2 + "GETDELTEST"

		value := yavalue + "GETDELTEST"

		redis.Raw().Set(ctx, key, value, yattl)

		gotValue, _ := redis.GetDel(ctx, key)

		t.Run("[GetDel] - get value works", func(t *testing.T) {
			assert.Equal(t, value, gotValue)
		})

		t.Run("[GetDel] - delete value works", func(t *testing.T) {
			result, _ := redis.Raw().Exists(ctx, key).Result()

			expected := 0

			assert.Equal(t, int64(expected), result)
		})
	})

	t.Run("[Exists] - check values exsists works", func(t *testing.T) {
		keys := make([]string, 0, 10)
		for i := range 10 {
			keys = append(keys, fmt.Sprintf("check_exists_key:%d", i))
			redis.Raw().Set(ctx, keys[i], yavalue, yattl)
		}

		result, _ := redis.Exists(ctx, keys...)

		expected := true

		assert.Equal(t, expected, result)
	})

	t.Run("[Del] - delete value works", func(t *testing.T) {
		deleteKey := yamainKey2 + "DELTEST"
		redis.Raw().Set(ctx, deleteKey, yavalue, yattl)

		result, _ := redis.Raw().Exists(ctx, deleteKey).Result()

		expected := 1

		assert.Equal(t, int64(expected), result)
	})

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
