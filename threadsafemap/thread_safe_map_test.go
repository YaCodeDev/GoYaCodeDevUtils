package threadsafemap_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/threadsafemap"
)

func TestThreadSafeMapImportFromMap(t *testing.T) {
	t.Parallel()

	const (
		firstKey    = "one"
		firstValue  = 1
		secondKey   = "two"
		secondValue = 2
		wantLength  = 2
	)

	m := threadsafemap.NewThreadSafeMap[string, int]()
	m.ImportFromMap(map[string]int{
		firstKey:  firstValue,
		secondKey: secondValue,
	})

	if got := m.Length(); got != wantLength {
		t.Fatalf("map length should match imported value count: got %d, want %d", got, wantLength)
	}

	if got, ok := m.Get(firstKey); !ok || got != firstValue {
		t.Fatalf("imported key should be readable: got %d, %v, want %d, true", got, ok, firstValue)
	}
}

func TestThreadSafeMapGetOrDefaultReleasesReadLock(t *testing.T) {
	t.Parallel()

	const (
		key          = "key"
		value        = 1
		defaultValue = 0
		nextKey      = "next"
		nextValue    = 2
	)

	m := threadsafemap.NewThreadSafeMap[string, int]()
	m.Set(key, value)

	if got := m.GetOrDefault(key, defaultValue); got != value {
		t.Fatalf("existing key should return stored value: got %d, want %d", got, value)
	}

	requireWriteCompletes(t, func() {
		m.Set(nextKey, nextValue)
	})
}

func TestThreadSafeMapMarshalJSONErrorReleasesReadLock(t *testing.T) {
	t.Parallel()

	const (
		badKey  = "bad"
		nextKey = "next"
	)

	m := threadsafemap.NewThreadSafeMap[string, func()]()
	m.Set(badKey, func() {})

	if _, err := json.Marshal(m); err == nil {
		t.Fatal("json marshal should fail for unsupported map values")
	}

	requireWriteCompletes(t, func() {
		m.Set(nextKey, func() {})
	})
}

func requireWriteCompletes(t *testing.T, write func()) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		defer close(done)
		write()
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("write should complete after read path returns")
	}
}
