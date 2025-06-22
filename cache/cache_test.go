package cache_test

import (
	"context"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/cache"
	"github.com/stretchr/testify/assert"
)

func TestCache_Initialize_Works(t *testing.T) {
	ctx := context.Background()

	t.Parallel()

	t.Run("[Redis] initialize works", func(t *testing.T) {
		client, cleanup := setupTestRedis(t)
		defer cleanup()

		cache := cache.NewCache(client)

		result := cache.Ping(ctx)

		assert.Nil(t, result)
	})

	t.Run("[Memory] initialize works", func(t *testing.T) {
		cache := cache.NewCache(cache.NewMemoryContainer())

		result := cache.Ping(ctx)

		assert.Nil(t, result)
	})
}
