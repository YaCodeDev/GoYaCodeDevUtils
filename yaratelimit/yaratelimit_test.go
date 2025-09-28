package yaratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yacache"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaratelimit"
	"github.com/stretchr/testify/assert"
)

const (
	TestUserID = 100
	TestGroup  = "sigma-life"
)

func TestIncrementWorkFlow_Works(t *testing.T) {
	ctx := context.Background()

	cache := yacache.NewCache(yacache.NewMemoryContainer())

	t.Run("Increment works", func(t *testing.T) {
		rate := yaratelimit.NewRateLimit(cache, 5, time.Second*400)

		_, _ = rate.Increment(ctx, TestUserID, TestGroup)

		expected := yaratelimit.FormatValue(1, time.Now().Unix())

		result, _ := cache.Get(ctx, yaratelimit.FormatKey(TestUserID, TestGroup))

		assert.Equal(t, expected, result)
	})

	t.Run("Refresh works", func(t *testing.T) {
		rate := yaratelimit.NewRateLimit(cache, 3, time.Millisecond*5)

		_, _ = rate.Increment(ctx, TestUserID, TestGroup)
		_, _ = rate.Increment(ctx, TestUserID, TestGroup)
		_, _ = rate.Increment(ctx, TestUserID, TestGroup)

		time.Sleep(time.Millisecond * 5)

		expected := yaratelimit.FormatValue(1, time.Now().Unix())

		_, _ = rate.Increment(ctx, TestUserID, TestGroup)

		result, _ := cache.Get(ctx, yaratelimit.FormatKey(TestUserID, TestGroup))

		assert.Equal(t, expected, result)
	})

	t.Run("Overflow works", func(t *testing.T) {
		rate := yaratelimit.NewRateLimit(cache, 3, time.Second)

		_, _ = rate.Increment(ctx, TestUserID, TestGroup)
		_, _ = rate.Increment(ctx, TestUserID, TestGroup)
		_, _ = rate.Increment(ctx, TestUserID, TestGroup)

		expected := yaratelimit.FormatValue(3, time.Now().Unix())

		_, _ = rate.Increment(ctx, TestUserID, TestGroup)
		_, _ = rate.Increment(ctx, TestUserID, TestGroup)
		_, _ = rate.Increment(ctx, TestUserID, TestGroup)

		result, _ := cache.Get(ctx, yaratelimit.FormatKey(TestUserID, TestGroup))

		assert.Equal(t, expected, result)
	})

	t.Run("Ban works", func(t *testing.T) {
		rate := yaratelimit.NewRateLimit(cache, 3, time.Second)

		_, _ = rate.Increment(ctx, TestUserID, TestGroup)
		_, _ = rate.Increment(ctx, TestUserID, TestGroup)
		_, _ = rate.Increment(ctx, TestUserID, TestGroup)

		result, _ := rate.Increment(ctx, TestUserID, TestGroup)

		expected := true

		assert.Equal(t, expected, result)
	})
}

func TestGet_Works(t *testing.T) {
	ctx := context.Background()

	cache := yacache.NewCache(yacache.NewMemoryContainer())

	rate := yaratelimit.NewRateLimit(cache, 5, time.Second*400)

	expected := &yaratelimit.Storage{
		Limit:        1,
		FirstRequest: time.Now().Unix(),
	}

	_, _ = rate.Increment(ctx, TestUserID, TestGroup)

	result, _ := rate.Get(ctx, TestUserID, TestGroup)

	assert.Equal(t, expected, result)
}

func TestCheckBanned_Works(t *testing.T) {
	ctx := context.Background()

	cache := yacache.NewCache(yacache.NewMemoryContainer())

	rate := yaratelimit.NewRateLimit(cache, 5, time.Second*400)

	expected := true

	_, _ = rate.Increment(ctx, TestUserID, TestGroup)
	_, _ = rate.Increment(ctx, TestUserID, TestGroup)
	_, _ = rate.Increment(ctx, TestUserID, TestGroup)
	_, _ = rate.Increment(ctx, TestUserID, TestGroup)
	_, _ = rate.Increment(ctx, TestUserID, TestGroup)

	result, _ := rate.CheckBanned(ctx, TestUserID, TestGroup)

	assert.Equal(t, expected, result)
}

func TestRefresh_Works(t *testing.T) {
	ctx := context.Background()

	cache := yacache.NewCache(yacache.NewMemoryContainer())

	rate := yaratelimit.NewRateLimit(cache, 5, time.Second*400)

	expected := false

	_, _ = rate.Increment(ctx, TestUserID, TestGroup)
	_, _ = rate.Increment(ctx, TestUserID, TestGroup)
	_, _ = rate.Increment(ctx, TestUserID, TestGroup)
	_, _ = rate.Increment(ctx, TestUserID, TestGroup)
	_, _ = rate.Increment(ctx, TestUserID, TestGroup)

	_ = rate.Refresh(ctx, TestUserID, TestGroup)

	result, _ := rate.CheckBanned(ctx, TestUserID, TestGroup)

	assert.Equal(t, expected, result)
}
