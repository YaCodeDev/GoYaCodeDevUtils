package yaratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yacache"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaratelimit"
	"github.com/stretchr/testify/assert"
)

func TestIncrementWorkFlow_Works(t *testing.T) {
	ctx := context.Background()

	cache := yacache.NewCache(yacache.NewMemoryContainer())

	t.Run("Increment works", func(t *testing.T) {
		rate := yaratelimit.NewRateLimit(cache, 5, time.Second*400)

		userID, group := uint64(100), "party"

		_, _ = rate.Increment(ctx, userID, group)

		expected := yaratelimit.FormatValue(1, time.Now().Unix())

		result, _ := cache.Get(ctx, yaratelimit.FormatKey(userID, group))

		assert.Equal(t, expected, result)
	})

	t.Run("Refresh works", func(t *testing.T) {
		rate := yaratelimit.NewRateLimit(cache, 3, time.Millisecond*5)

		userID, group := uint64(100), "party"

		rate.Increment(ctx, userID, group)
		rate.Increment(ctx, userID, group)
		rate.Increment(ctx, userID, group)

		time.Sleep(time.Millisecond * 5)

		expected := yaratelimit.FormatValue(1, time.Now().Unix())

		_, _ = rate.Increment(ctx, userID, group)

		result, _ := cache.Get(ctx, yaratelimit.FormatKey(userID, group))

		assert.Equal(t, expected, result)
	})

	t.Run("Overflow works", func(t *testing.T) {
		rate := yaratelimit.NewRateLimit(cache, 3, time.Second)

		userID, group := uint64(100), "party"

		_, _ = rate.Increment(ctx, userID, group)
		_, _ = rate.Increment(ctx, userID, group)
		_, _ = rate.Increment(ctx, userID, group)

		expected := yaratelimit.FormatValue(3, time.Now().Unix())

		_, _ = rate.Increment(ctx, userID, group)
		_, _ = rate.Increment(ctx, userID, group)
		_, _ = rate.Increment(ctx, userID, group)

		result, _ := cache.Get(ctx, yaratelimit.FormatKey(userID, group))

		assert.Equal(t, expected, result)
	})

	t.Run("Ban works", func(t *testing.T) {
		rate := yaratelimit.NewRateLimit(cache, 3, time.Second)

		userID, group := uint64(100), "party"

		_, _ = rate.Increment(ctx, userID, group)
		_, _ = rate.Increment(ctx, userID, group)
		_, _ = rate.Increment(ctx, userID, group)

		_, _ = rate.Increment(ctx, userID, group)
		_, _ = rate.Increment(ctx, userID, group)

		result, _ := rate.Increment(ctx, userID, group)

		expected := true

		assert.Equal(t, expected, result)
	})
}

func TestGet_Works(t *testing.T) {
	ctx := context.Background()

	cache := yacache.NewCache(yacache.NewMemoryContainer())

	rate := yaratelimit.NewRateLimit(cache, 5, time.Second*400)

	userID, group := uint64(100), "party"

	expected := &yaratelimit.Storage{
		Limit:        1,
		FirstRequest: time.Now().Unix(),
	}

	_, _ = rate.Increment(ctx, userID, group)

	result, _ := rate.Get(ctx, userID, group)

	assert.Equal(t, expected, result)
}

func TestCheckBanned_Works(t *testing.T) {
	ctx := context.Background()

	cache := yacache.NewCache(yacache.NewMemoryContainer())

	rate := yaratelimit.NewRateLimit(cache, 5, time.Second*400)

	userID, group := uint64(100), "party"

	expected := true

	_, _ = rate.Increment(ctx, userID, group)
	_, _ = rate.Increment(ctx, userID, group)
	_, _ = rate.Increment(ctx, userID, group)
	_, _ = rate.Increment(ctx, userID, group)
	_, _ = rate.Increment(ctx, userID, group)

	result, _ := rate.CheckBanned(ctx, userID, group)

	assert.Equal(t, expected, result)
}

func TestRefresh_Works(t *testing.T) {
	ctx := context.Background()

	cache := yacache.NewCache(yacache.NewMemoryContainer())

	rate := yaratelimit.NewRateLimit(cache, 5, time.Second*400)

	userID, group := uint64(100), "party"

	expected := false

	_, _ = rate.Increment(ctx, userID, group)
	_, _ = rate.Increment(ctx, userID, group)
	_, _ = rate.Increment(ctx, userID, group)
	_, _ = rate.Increment(ctx, userID, group)
	_, _ = rate.Increment(ctx, userID, group)

	_ = rate.Refresh(ctx, userID, group)

	result, _ := rate.CheckBanned(ctx, userID, group)

	assert.Equal(t, expected, result)
}
