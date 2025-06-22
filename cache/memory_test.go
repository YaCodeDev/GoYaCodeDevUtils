package cache_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/cache"
	"github.com/stretchr/testify/assert"
)

const (
	yamainKey  = "yamain"
	yachildKey = "yachild"
	yavalue    = "yavalue"
	yattl      = time.Hour
)

func TestMemory_New_Works(t *testing.T) {
	memory := cache.NewMemory(cache.NewMemoryContainer(), time.Hour)

	assert.Equal(t, memory.Ping(), nil)
}

func TestMemory_TTLCleanup_Works(t *testing.T) {
	ctx := context.Background()

	tick := time.Second / 10

	memory := cache.NewMemory(cache.NewMemoryContainer(), tick)

	memory.HSetEX(ctx, yamainKey, yachildKey, yavalue, time.Microsecond) //nolint:errcheck

	time.Sleep(tick + (time.Millisecond * 5))

	exist, _ := memory.HExist(ctx, yamainKey, yachildKey)

	expected := false

	assert.Equal(t, expected, exist)
}

func TestMemory_InsertWorkflow_Works(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	memory := cache.NewMemory(cache.NewMemoryContainer(), time.Hour)

	err := memory.HSetEX(ctx, yamainKey, yachildKey, yavalue, yattl)
	if err != nil {
		panic(err)
	}

	t.Run("[HSetEX] insert value works", func(t *testing.T) {
		value := memory.Raw()[yamainKey][yachildKey].Value

		assert.Equal(t, yavalue, value)
	})

	t.Run("[HSetEX] increment len works", func(t *testing.T) {
		hlen, _ := memory.HLen(context.Background(), yamainKey)

		expected := int64(1)

		assert.Equal(t, expected, hlen)
	})
}

func TestMemory_FetchWorkflow_Works(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	memory := cache.NewMemory(cache.NewMemoryContainer(), time.Hour)

	err := memory.HSetEX(ctx, yamainKey, yachildKey, yavalue, yattl)
	if err != nil {
		panic(err)
	}

	t.Run("[HExist] - works", func(t *testing.T) {
		exist, _ := memory.HExist(ctx, yamainKey, yachildKey)

		expected := true

		assert.Equal(t, expected, exist)
	})

	t.Run("[HGet] - get item works", func(t *testing.T) {
		value, _ := memory.HGet(ctx, yamainKey, yachildKey)

		assert.Equal(t, yavalue, value)
	})

	t.Run("[HGetAll] - get items works", func(t *testing.T) {
		expected := make(map[string]string)

		expected[yachildKey] = yavalue

		for i := range 10 {
			err := memory.HSetEX(
				ctx,
				yamainKey,
				fmt.Sprintf("%s:%d", yachildKey, i),
				fmt.Sprintf("%s:%d", yavalue, i),
				yattl,
			)
			if err != nil {
				panic(err)
			}

			expected[fmt.Sprintf("%s:%d", yachildKey, i)] = fmt.Sprintf("%s:%d", yavalue, i)
		}

		result, _ := memory.HGetAll(ctx, yamainKey)

		assert.Equal(t, expected, result)
	})

	t.Run("[HGetDelSingle] - get and delete item works", func(t *testing.T) {
		deleteMainKey := yamainKey + ":delete_test"
		deleteChildKey := yachildKey + ":delete_test"
		deleteValue := yavalue + ":delete_test"

		err := memory.HSetEX(ctx, deleteMainKey, deleteChildKey, deleteValue, yattl)
		if err != nil {
			panic(err)
		}

		oldLen, _ := memory.HLen(ctx, deleteMainKey)

		value, _ := memory.HGetDelSingle(ctx, deleteMainKey, deleteChildKey)

		t.Run("[HGetDelSingle] - get works", func(t *testing.T) {
			assert.Equal(t, deleteValue, value)
		})

		t.Run("[HGetDelSingle] - delete works", func(t *testing.T) {
			_, err := memory.HGet(ctx, deleteMainKey, deleteChildKey)

			assert.NotNil(t, err)
		})

		t.Run("[HGetDelSingle] - decrement len works", func(t *testing.T) {
			hlen, _ := memory.HLen(ctx, deleteMainKey)

			expected := oldLen - 1

			assert.Equal(t, expected, hlen)
		})
	})
}

func TestMemory_DeleteWorkflow_Works(t *testing.T) {
	ctx := context.Background()

	memory := cache.NewMemory(cache.NewMemoryContainer(), time.Hour)

	err := memory.HSetEX(ctx, yamainKey, yachildKey, yavalue, yattl)
	if err != nil {
		panic(err)
	}

	oldLen, _ := memory.HLen(ctx, yamainKey)

	t.Run("[HDelSingle] - delete item works", func(t *testing.T) {
		_ = memory.HDelSingle(ctx, yamainKey, yachildKey)

		t.Run("[HDelSingle] - not exists works", func(t *testing.T) {
			exist, _ := memory.HExist(ctx, yamainKey, yachildKey)

			expected := false

			assert.Equal(t, exist, expected)
		})

		t.Run("[HDelSingle] - decrement len works", func(t *testing.T) {
			hlen, _ := memory.HLen(ctx, yamainKey)

			expected := oldLen - 1

			assert.Equal(t, expected, hlen)
		})
	})
}
