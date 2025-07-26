package yatgstorage_test

import (
	"context"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yacache"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgstorage"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
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

func TestYaTgStorage_CreateWorks(t *testing.T) {
	client, cleanup := setupTestRedis(t)

	defer cleanup()

	if err := yatgstorage.
		NewStorage(yacache.NewCache(client), nil, yalogger.NewBaseLogger(nil).NewLogger()).
		Ping(context.Background()); err != nil {
		t.Fatalf("Failed to create tg storage")
	}
}
