package yacache_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yacache"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestCacheModules(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "when MemoryModule is wired / then it resolves a usable Cache[MemoryContainer]",
			run:  testMemoryModuleResolvesUsableCache,
		},
		{
			name: "when RedisModule is wired against a miniredis instance / then it resolves a usable Cache[*redis.Client]",
			run:  testRedisModuleResolvesUsableCache,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, testCase.run)
	}
}

func testMemoryModuleResolvesUsableCache(t *testing.T) {
	t.Parallel()

	const (
		key   = "fx-memory-key"
		value = "fx-memory-value"
	)

	var cache yacache.Cache[yacache.MemoryContainer]

	fxtest.New(
		t,
		yacache.MemoryModule,
		fx.Populate(&cache),
	)

	if cache == nil {
		t.Fatalf("expected MemoryModule to populate a non-nil Cache[MemoryContainer]")
	}

	ctx := context.Background()

	if err := cache.Set(ctx, key, value, 0); err != nil {
		t.Fatalf("expected Set to succeed, got error: %v", err)
	}

	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("expected Get to succeed, got error: %v", err)
	}

	if got != value {
		t.Errorf("expected stored value %q, got %q", value, got)
	}
}

func testRedisModuleResolvesUsableCache(t *testing.T) {
	t.Parallel()

	const (
		key   = "fx-redis-key"
		value = "fx-redis-value"
	)

	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("expected miniredis to start, got error: %v", err)
	}

	defer server.Close()

	port, err := strconv.ParseUint(server.Port(), 10, 16)
	if err != nil {
		t.Fatalf("expected miniredis port to parse as uint16, got error: %v", err)
	}

	var cache yacache.Cache[*redis.Client]

	fxtest.New(
		t,
		yacache.RedisModule,
		yalogger.LoggerModule,
		fx.Supply((*yalogger.Config)(nil)),
		fx.Supply(yacache.RedisParams{
			Host: server.Host(),
			Port: uint16(port),
		}),
		fx.Populate(&cache),
	)

	if cache == nil {
		t.Fatalf("expected RedisModule to populate a non-nil Cache[*redis.Client]")
	}

	ctx := context.Background()

	if err := cache.Set(ctx, key, value, 0); err != nil {
		t.Fatalf("expected Set to succeed, got error: %v", err)
	}

	got, err := cache.Get(ctx, key)
	if err != nil {
		t.Fatalf("expected Get to succeed, got error: %v", err)
	}

	if got != value {
		t.Errorf("expected stored value %q, got %q", value, got)
	}
}
