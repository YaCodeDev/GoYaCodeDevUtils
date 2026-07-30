// Package yaringbuffer provides a generic, concurrency-safe, keyed ring
// buffer designed for fair round-robin selection — for example, picking the
// next available executor in a job scheduler where executors register,
// reconnect under the same key, and disconnect at any time.
//
// # Quick start
//
//	ring := yaringbuffer.New[string, Executor](yaringbuffer.DefaultInitialCapacity)
//	ring.Upsert("executor-1", exec1)
//	ring.Upsert("executor-2", exec2)
//
//	executor, found := ring.NextMatch(func(e Executor) bool { return e.Idle() })
//	if found {
//	    executor.Dispatch(job)
//	}
//
// The package is dependency-free and can be safely vendored.
package yaringbuffer

import (
	"sync"
)

// entry pairs a key with its stored value inside the ring.
type entry[K comparable, V any] struct {
	key   K
	value V
}

// RingBuffer is a generic, concurrency-safe, keyed ring buffer with fair
// cursor-based round-robin selection.
//
// Fairness model:
//   - Ring order is insertion order; a cursor marks the next logical entry.
//   - Next returns the entry at the cursor and advances the cursor by one,
//     wrapping at the end, so entries take strict turns.
//   - Upsert on an existing key replaces the value in place: the entry keeps
//     its ring position and the cursor is untouched, so a reconnecting
//     executor neither gains nor loses its turn.
//   - Remove compacts the ring preserving relative order and keeps the cursor
//     pointing at the same next logical entry, so a disconnect never skips or
//     repeats anyone else's turn.
//   - NextMatch scans at most one full lap from the cursor, returns the first
//     entry accepted by the predicate, and sets the cursor to the position
//     right after it, so each matching entry gets exactly one turn per lap.
//
// The zero value is ready to use; internal storage initializes lazily on
// first use. All methods are safe for concurrent use.
type RingBuffer[K comparable, V any] struct {
	entries []entry[K, V]
	index   map[K]int
	cursor  int
	mu      sync.RWMutex
}

// New returns a ring buffer whose backing storage is pre-allocated for
// initialCapacity entries. A non-positive initialCapacity falls back to
// DefaultInitialCapacity.
//
// Example:
//
//	ring := yaringbuffer.New[string, int](0) // uses DefaultInitialCapacity
func New[K comparable, V any](initialCapacity int) *RingBuffer[K, V] {
	if initialCapacity <= 0 {
		initialCapacity = DefaultInitialCapacity
	}

	return &RingBuffer[K, V]{
		entries: make([]entry[K, V], 0, initialCapacity),
		index:   make(map[K]int, initialCapacity),
	}
}

// Upsert inserts the value under the key or replaces an existing value.
//
// An existing key keeps its ring position and leaves the cursor untouched; a
// new key is appended at the tail of the ring, growing the backing storage by
// GrowthFactor when full. It reports whether an existing entry was replaced.
//
// Example:
//
//	replaced := ring.Upsert("executor-1", executor) // false on first insert
func (r *RingBuffer[K, V]) Upsert(key K, value V) (replaced bool) {
	r.safetyCheck()

	r.mu.Lock()
	defer r.mu.Unlock()

	if pos, exists := r.index[key]; exists {
		r.entries[pos].value = value

		return true
	}

	if len(r.entries) == cap(r.entries) {
		r.grow()
	}

	r.index[key] = len(r.entries)
	r.entries = append(r.entries, entry[K, V]{key: key, value: value})

	return false
}

// Remove deletes the entry stored under the key, compacting the ring while
// preserving relative order. The cursor keeps pointing at the same next
// logical entry: it shifts down when an earlier position is removed and wraps
// to the start when it falls off the end. Removing the last entry resets the
// cursor to zero. It reports whether an entry was removed.
//
// Example:
//
//	removed := ring.Remove("executor-1")
func (r *RingBuffer[K, V]) Remove(key K) (removed bool) {
	r.safetyCheck()

	r.mu.Lock()
	defer r.mu.Unlock()

	pos, exists := r.index[key]
	if !exists {
		return false
	}

	delete(r.index, key)

	r.entries = append(r.entries[:pos], r.entries[pos+1:]...)

	for i := pos; i < len(r.entries); i++ {
		r.index[r.entries[i].key] = i
	}

	if pos < r.cursor {
		r.cursor--
	}

	if r.cursor >= len(r.entries) {
		r.cursor = 0
	}

	return true
}

// Get retrieves the value stored under the key without affecting the cursor,
// along with a boolean indicating whether the key was found.
//
// Example:
//
//	executor, found := ring.Get("executor-1")
func (r *RingBuffer[K, V]) Get(key K) (value V, found bool) {
	r.safetyCheck()

	r.mu.RLock()
	defer r.mu.RUnlock()

	pos, exists := r.index[key]
	if !exists {
		return value, false
	}

	return r.entries[pos].value, true
}

// Len returns the number of entries currently in the ring buffer.
func (r *RingBuffer[K, V]) Len() int {
	r.safetyCheck()

	r.mu.RLock()
	defer r.mu.RUnlock()

	return len(r.entries)
}

// Next returns the entry at the cursor and advances the cursor by one,
// wrapping at the end of the ring. On an empty buffer it returns the zero
// value and false.
//
// Example:
//
//	executor, found := ring.Next()
func (r *RingBuffer[K, V]) Next() (value V, found bool) {
	r.safetyCheck()

	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.entries) == 0 {
		return value, false
	}

	value = r.entries[r.cursor].value
	r.cursor = (r.cursor + 1) % len(r.entries)

	return value, true
}

// NextMatch scans at most one full lap starting at the cursor and returns the
// first entry the match predicate accepts, setting the cursor to the position
// right after it so consecutive calls stay fair round-robin among matching
// entries. Non-matching entries are skipped without being returned. If no
// entry matches, it returns the zero value and false and the cursor stays at
// its starting position.
//
// The match predicate runs while the buffer's lock is held: it must be fast
// and must not call back into the buffer, or it will deadlock.
//
// Example:
//
//	executor, found := ring.NextMatch(func(e Executor) bool { return e.Idle() })
func (r *RingBuffer[K, V]) NextMatch(match func(V) bool) (value V, found bool) {
	r.safetyCheck()

	r.mu.Lock()
	defer r.mu.Unlock()

	total := len(r.entries)

	for offset := range total {
		pos := (r.cursor + offset) % total
		if match(r.entries[pos].value) {
			r.cursor = (pos + 1) % total

			return r.entries[pos].value, true
		}
	}

	return value, false
}

// Keys returns the keys in ring order starting at the cursor.
//
// Example:
//
//	for _, key := range ring.Keys() {
//	    fmt.Println(key)
//	}
func (r *RingBuffer[K, V]) Keys() []K {
	r.safetyCheck()

	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := make([]K, 0, len(r.entries))
	for offset := range len(r.entries) {
		keys = append(keys, r.entries[(r.cursor+offset)%len(r.entries)].key)
	}

	return keys
}

// Values returns the values in ring order starting at the cursor.
//
// Example:
//
//	for _, value := range ring.Values() {
//	    fmt.Println(value)
//	}
func (r *RingBuffer[K, V]) Values() []V {
	r.safetyCheck()

	r.mu.RLock()
	defer r.mu.RUnlock()

	values := make([]V, 0, len(r.entries))
	for offset := range len(r.entries) {
		values = append(values, r.entries[(r.cursor+offset)%len(r.entries)].value)
	}

	return values
}

// Clear removes every entry and resets the cursor to zero.
func (r *RingBuffer[K, V]) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries = nil
	r.index = make(map[K]int, DefaultInitialCapacity)
	r.cursor = 0
}

// grow reallocates the backing storage multiplied by GrowthFactor, falling
// back to DefaultInitialCapacity when the current capacity is zero.
func (r *RingBuffer[K, V]) grow() {
	capacity := cap(r.entries) * GrowthFactor
	if capacity == 0 {
		capacity = DefaultInitialCapacity
	}

	grown := make([]entry[K, V], len(r.entries), capacity)
	copy(grown, r.entries)
	r.entries = grown
}

// safetyCheck ensures the internal index is initialized before any operation,
// so a zero-value RingBuffer is fully usable.
func (r *RingBuffer[K, V]) safetyCheck() {
	r.mu.RLock()
	initialized := r.index != nil
	r.mu.RUnlock()

	if initialized {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.index == nil {
		r.index = make(map[K]int, DefaultInitialCapacity)
	}
}
