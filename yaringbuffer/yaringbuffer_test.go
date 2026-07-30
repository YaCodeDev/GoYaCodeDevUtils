package yaringbuffer_test

import (
	"sync"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaringbuffer"
)

func isEven(v int) bool {
	return v%2 == 0
}

func requireNext(t *testing.T, ring *yaringbuffer.RingBuffer[string, int], want int) {
	t.Helper()

	got, found := ring.Next()
	if !found {
		t.Fatalf("next should find an entry in a non-empty buffer: want %d", want)
	}

	if got != want {
		t.Fatalf("next should follow ring order: got %d, want %d", got, want)
	}
}

func requireNextMatch(
	t *testing.T,
	ring *yaringbuffer.RingBuffer[string, int],
	match func(int) bool,
	want int,
) {
	t.Helper()

	got, found := ring.NextMatch(match)
	if !found {
		t.Fatalf("next match should find a matching entry: want %d", want)
	}

	if got != want {
		t.Fatalf("next match should stay fair among matching entries: got %d, want %d", got, want)
	}
}

func TestRingBufferEmptyState(t *testing.T) {
	t.Parallel()

	ring := yaringbuffer.New[string, int](0)

	if got := ring.Len(); got != 0 {
		t.Fatalf("new buffer should be empty: got length %d", got)
	}

	if got, found := ring.Next(); found || got != 0 {
		t.Fatalf("next on empty buffer should return zero value and false: got %d, %v", got, found)
	}

	if got, found := ring.NextMatch(isEven); found || got != 0 {
		t.Fatalf(
			"next match on empty buffer should return zero value and false: got %d, %v",
			got,
			found,
		)
	}

	if got, found := ring.Get("missing"); found || got != 0 {
		t.Fatalf("get on empty buffer should return zero value and false: got %d, %v", got, found)
	}

	if got := len(ring.Keys()); got != 0 {
		t.Fatalf("keys on empty buffer should be empty: got %d entries", got)
	}

	if got := len(ring.Values()); got != 0 {
		t.Fatalf("values on empty buffer should be empty: got %d entries", got)
	}
}

func TestRingBufferZeroValueIsUsable(t *testing.T) {
	t.Parallel()

	const (
		key   = "a"
		value = 1
	)

	var ring yaringbuffer.RingBuffer[string, int]

	if got, found := ring.Next(); found || got != 0 {
		t.Fatalf(
			"next on zero-value buffer should return zero value and false: got %d, %v",
			got,
			found,
		)
	}

	if got, found := ring.NextMatch(isEven); found || got != 0 {
		t.Fatalf(
			"next match on zero-value buffer should return zero value and false: got %d, %v",
			got,
			found,
		)
	}

	if replaced := ring.Upsert(key, value); replaced {
		t.Fatal("upsert of a new key on zero-value buffer should not report replaced")
	}

	if got, found := ring.Get(key); !found || got != value {
		t.Fatalf(
			"zero-value buffer should store entries: got %d, %v, want %d, true",
			got,
			found,
			value,
		)
	}

	requireNext(t, &ring, value)
}

func TestRingBufferNextRoundRobinOverMultipleLaps(t *testing.T) {
	t.Parallel()

	const lapCount = 3

	ring := yaringbuffer.New[string, int](yaringbuffer.DefaultInitialCapacity)
	ring.Upsert("a", 1)
	ring.Upsert("b", 2)
	ring.Upsert("c", 3)

	for lap := range lapCount {
		for _, want := range []int{1, 2, 3} {
			got, found := ring.Next()
			if !found {
				t.Fatalf("next should find an entry on lap %d", lap)
			}

			if got != want {
				t.Fatalf(
					"next should repeat insertion order every lap: lap %d, got %d, want %d",
					lap,
					got,
					want,
				)
			}
		}
	}
}

func TestRingBufferUpsertReplaceKeepsPositionAndCursor(t *testing.T) {
	t.Parallel()

	const replacedValue = 10

	ring := yaringbuffer.New[string, int](yaringbuffer.DefaultInitialCapacity)
	ring.Upsert("a", 1)
	ring.Upsert("b", 2)
	ring.Upsert("c", 3)

	requireNext(t, ring, 1)

	if replaced := ring.Upsert("a", replacedValue); !replaced {
		t.Fatal("upsert of an existing key should report replaced")
	}

	if got := ring.Len(); got != 3 {
		t.Fatalf("replace should not change length: got %d, want 3", got)
	}

	requireNext(t, ring, 2)
	requireNext(t, ring, 3)
	requireNext(t, ring, replacedValue)
}

func TestRingBufferRemoveBeforeCursor(t *testing.T) {
	t.Parallel()

	ring := yaringbuffer.New[string, int](yaringbuffer.DefaultInitialCapacity)
	ring.Upsert("a", 1)
	ring.Upsert("b", 2)
	ring.Upsert("c", 3)

	requireNext(t, ring, 1)
	requireNext(t, ring, 2)

	if removed := ring.Remove("a"); !removed {
		t.Fatal("remove of an existing key should report removed")
	}

	requireNext(t, ring, 3)
	requireNext(t, ring, 2)
}

func TestRingBufferRemoveAtCursor(t *testing.T) {
	t.Parallel()

	ring := yaringbuffer.New[string, int](yaringbuffer.DefaultInitialCapacity)
	ring.Upsert("a", 1)
	ring.Upsert("b", 2)
	ring.Upsert("c", 3)

	requireNext(t, ring, 1)

	if removed := ring.Remove("b"); !removed {
		t.Fatal("remove of an existing key should report removed")
	}

	requireNext(t, ring, 3)
	requireNext(t, ring, 1)
}

func TestRingBufferRemoveAfterCursor(t *testing.T) {
	t.Parallel()

	ring := yaringbuffer.New[string, int](yaringbuffer.DefaultInitialCapacity)
	ring.Upsert("a", 1)
	ring.Upsert("b", 2)
	ring.Upsert("c", 3)

	requireNext(t, ring, 1)

	if removed := ring.Remove("c"); !removed {
		t.Fatal("remove of an existing key should report removed")
	}

	requireNext(t, ring, 2)
	requireNext(t, ring, 1)
}

func TestRingBufferRemoveFinalEntryWrapsCursor(t *testing.T) {
	t.Parallel()

	ring := yaringbuffer.New[string, int](yaringbuffer.DefaultInitialCapacity)
	ring.Upsert("a", 1)
	ring.Upsert("b", 2)
	ring.Upsert("c", 3)

	requireNext(t, ring, 1)
	requireNext(t, ring, 2)

	if removed := ring.Remove("c"); !removed {
		t.Fatal("remove of an existing key should report removed")
	}

	requireNext(t, ring, 1)
	requireNext(t, ring, 2)
}

func TestRingBufferRemoveLastRemainingEntry(t *testing.T) {
	t.Parallel()

	const (
		key      = "a"
		newKey   = "b"
		newValue = 2
	)

	ring := yaringbuffer.New[string, int](yaringbuffer.DefaultInitialCapacity)
	ring.Upsert(key, 1)

	requireNext(t, ring, 1)

	if removed := ring.Remove(key); !removed {
		t.Fatal("remove of the last remaining entry should report removed")
	}

	if got := ring.Len(); got != 0 {
		t.Fatalf("buffer should be empty after removing the last entry: got length %d", got)
	}

	if got, found := ring.Next(); found || got != 0 {
		t.Fatalf(
			"next after removing the last entry should return zero value and false: got %d, %v",
			got,
			found,
		)
	}

	ring.Upsert(newKey, newValue)
	requireNext(t, ring, newValue)
}

func TestRingBufferRemoveMissingKey(t *testing.T) {
	t.Parallel()

	ring := yaringbuffer.New[string, int](yaringbuffer.DefaultInitialCapacity)
	ring.Upsert("a", 1)

	if removed := ring.Remove("missing"); removed {
		t.Fatal("remove of a missing key should not report removed")
	}

	requireNext(t, ring, 1)
}

func TestRingBufferGrowthPreservesOrderAndCursor(t *testing.T) {
	t.Parallel()

	const initialCapacity = 2

	ring := yaringbuffer.New[string, int](initialCapacity)
	ring.Upsert("a", 1)
	ring.Upsert("b", 2)

	requireNext(t, ring, 1)

	ring.Upsert("c", 3)
	ring.Upsert("d", 4)
	ring.Upsert("e", 5)

	wantKeys := []string{"b", "c", "d", "e", "a"}

	gotKeys := ring.Keys()
	if len(gotKeys) != len(wantKeys) {
		t.Fatalf(
			"keys length should match live entries: got %d, want %d",
			len(gotKeys),
			len(wantKeys),
		)
	}

	for i, want := range wantKeys {
		if gotKeys[i] != want {
			t.Fatalf(
				"keys should keep ring order across growth: index %d, got %q, want %q",
				i,
				gotKeys[i],
				want,
			)
		}
	}

	for _, want := range []int{2, 3, 4, 5, 1, 2} {
		requireNext(t, ring, want)
	}
}

func TestRingBufferNextMatchSkipsAndStaysFair(t *testing.T) {
	t.Parallel()

	ring := yaringbuffer.New[string, int](yaringbuffer.DefaultInitialCapacity)
	ring.Upsert("a", 1)
	ring.Upsert("b", 2)
	ring.Upsert("c", 3)
	ring.Upsert("d", 4)
	ring.Upsert("e", 5)

	requireNextMatch(t, ring, isEven, 2)
	requireNextMatch(t, ring, isEven, 4)
	requireNextMatch(t, ring, isEven, 2)
	requireNextMatch(t, ring, isEven, 4)

	requireNext(t, ring, 5)
	requireNextMatch(t, ring, isEven, 2)
}

func TestRingBufferNextMatchNoMatchRestoresCursor(t *testing.T) {
	t.Parallel()

	ring := yaringbuffer.New[string, int](yaringbuffer.DefaultInitialCapacity)
	ring.Upsert("a", 1)
	ring.Upsert("b", 3)
	ring.Upsert("c", 5)

	requireNext(t, ring, 1)

	if got, found := ring.NextMatch(isEven); found || got != 0 {
		t.Fatalf(
			"next match without matches should return zero value and false: got %d, %v",
			got,
			found,
		)
	}

	requireNext(t, ring, 3)
}

func TestRingBufferChurnFairness(t *testing.T) {
	t.Parallel()

	const initialCapacity = 2

	ring := yaringbuffer.New[string, int](initialCapacity)
	ring.Upsert("a", 2)
	ring.Upsert("b", 3)
	ring.Upsert("c", 4)
	ring.Upsert("d", 5)
	ring.Upsert("e", 6)

	requireNext(t, ring, 2)
	ring.Remove("c")
	ring.Upsert("f", 8)
	ring.Remove("a")
	ring.Upsert("b", 30)

	matching := make([]int, 0, ring.Len())

	for _, value := range ring.Values() {
		if isEven(value) {
			matching = append(matching, value)
		}
	}

	if len(matching) < 2 {
		t.Fatalf("fixture should keep at least two matching entries alive: got %d", len(matching))
	}

	for lap := range 2 {
		counts := make(map[int]int, len(matching))

		for range matching {
			got, found := ring.NextMatch(isEven)
			if !found {
				t.Fatalf("next match should find a matching entry on lap %d", lap)
			}

			counts[got]++
		}

		for _, value := range matching {
			if counts[value] != 1 {
				t.Fatalf(
					"each live matching entry should be selected once per lap: lap %d, value %d, got %d selections",
					lap,
					value,
					counts[value],
				)
			}
		}
	}
}

func TestRingBufferClear(t *testing.T) {
	t.Parallel()

	ring := yaringbuffer.New[string, int](yaringbuffer.DefaultInitialCapacity)
	ring.Upsert("a", 1)
	ring.Upsert("b", 2)

	requireNext(t, ring, 1)

	ring.Clear()

	if got := ring.Len(); got != 0 {
		t.Fatalf("clear should empty the buffer: got length %d", got)
	}

	if got, found := ring.Next(); found || got != 0 {
		t.Fatalf("next after clear should return zero value and false: got %d, %v", got, found)
	}

	ring.Upsert("c", 3)
	requireNext(t, ring, 3)
}

func TestRingBufferConcurrentStress(t *testing.T) {
	t.Parallel()

	const (
		workerCount         = 8
		operationsPerWorker = 1000
		keySpace            = 32
		operationKinds      = 5
	)

	ring := yaringbuffer.New[int, int](yaringbuffer.DefaultInitialCapacity)

	var wg sync.WaitGroup

	for worker := range workerCount {
		wg.Add(1)

		go func(seed int) {
			defer wg.Done()

			for op := range operationsPerWorker {
				key := (seed*operationsPerWorker + op) % keySpace

				switch op % operationKinds {
				case 0:
					ring.Upsert(key, op)
				case 1:
					ring.Remove(key)
				case 2:
					ring.Next()
				case 3:
					ring.NextMatch(isEven)
				default:
					ring.Get(key)
					ring.Keys()
					ring.Values()
					ring.Len()
				}
			}
		}(worker)
	}

	wg.Wait()

	keys := ring.Keys()
	if got, want := ring.Len(), len(keys); got != want {
		t.Fatalf("length should match key count after concurrent churn: got %d, want %d", got, want)
	}

	for _, key := range keys {
		if _, found := ring.Get(key); !found {
			t.Fatalf("every listed key should stay retrievable after concurrent churn: key %d", key)
		}
	}
}
