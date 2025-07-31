package yatgstorage_test

import (
	"context"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yacache"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgstorage"
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

func TestStorage_CreateWorks(t *testing.T) {
	client, cleanup := setupTestRedis(t)

	defer cleanup()

	if err := yatgstorage.
		NewStorage(yacache.NewCache(client), yalogger.NewBaseLogger(nil).NewLogger()).
		Ping(context.Background()); err != nil {
		t.Fatalf("Failed to create tg storage")
	}
}

func TestStorageChannel_WorkFlowWorks(t *testing.T) {
	const (
		entityID  = 1111
		channelID = 1111
	)

	ctx := context.Background()

	client, cleanup := setupTestRedis(t)
	log := yalogger.NewBaseLogger(nil).NewLogger()

	defer cleanup()

	storage := yatgstorage.
		NewStorage(yacache.NewCache(client), log)

	t.Run("Set and Get channel pts - works", func(t *testing.T) {
		const expected = 1000

		_ = storage.SetChannelPts(ctx, entityID, channelID, expected)

		result, _, _ := storage.GetChannelPts(ctx, entityID, channelID)

		assert.Equal(t, expected, result)
	})

	t.Run("For each channels iterate - works", func(t *testing.T) {
		const entityChildID = 9

		channelIDs := []int64{1, 2, 3, 4, 5, 6, 7}

		for _, v := range channelIDs {
			_ = storage.SetChannelPts(ctx, entityChildID, v, int(v)*2)
		}

		_ = storage.ForEachChannels(
			ctx,
			entityChildID,
			func(_ context.Context, channelID int64, pts int) error {
				assert.Equal(t, int(channelID)*2, pts)

				return nil
			},
		)
	})

	t.Run("Set and Get channel access hash - works", func(t *testing.T) {
		expected := int64(100)

		_ = storage.SetChannelAccessHash(ctx, entityID, channelID, expected)

		result, _, _ := storage.GetChannelAccessHash(ctx, entityID, channelID)

		assert.Equal(t, expected, result)
	})
}

func TestStorageUser_WorkFlowWorks(t *testing.T) {
	ctx := context.Background()

	client, cleanup := setupTestRedis(t)
	log := yalogger.NewBaseLogger(nil).NewLogger()

	defer cleanup()

	storage := yatgstorage.
		NewStorage(yacache.NewCache(client), log)

	t.Run("Set and Get user access hash - works", func(t *testing.T) {
		const (
			entityID = 1000
			userID   = 2222
		)

		expected := int64(200)

		_ = storage.SetUserAccessHash(ctx, entityID, userID, expected)

		result, _ := storage.GetUserAccessHash(ctx, entityID, userID)

		assert.Equal(t, expected, result)
	})
}
