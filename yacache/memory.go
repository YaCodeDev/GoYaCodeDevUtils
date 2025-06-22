// ========================= In‑memory implementation ========================= //

// Memory is a threadsafe, TTL‑aware map‑backed cache suitable for single‑process
// applications or unit‑tests.  A background goroutine cleans up expired entries
// at a fixed interval specified by timeToClean.

package yacache

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
)

const yaMapLen = `[_____YaMapLen_____YA_/\_CODE_/\_DEV]`

// Memory is a threadsafe, TTL‑aware map‑backed cache.
//
// Example (create + basic operations):
//
//	memory := cache.NewMemory(cache.NewMemoryContainer(), time.Minute)
//	_   = memory.HSetEX(ctx, "main", "field", "v", time.Hour)
//	hlen, _ := memory.HLen(ctx, "main")
//	fmt.Println(hlen) // 1
type Memory struct {
	data   MemoryContainer // nested map mainKey → childKey → *memoryCacheItem
	mutex  sync.RWMutex    // guards *all* access to data
	ticker *time.Ticker    // drives the cleanup loop
	done   chan bool       // signals the goroutine to exit on Close()
}

// NewMemory builds a new [Memory] cache instance and immediately starts the
// background sweeper.
//
//	data        – caller‑provided map; pass NewMemoryContainer() for an empty cache
//	timeToClean – sweep interval; choose a value >> typical TTL to amortise cost
//
// Example:
//
//	memory := cache.NewMemory(cache.NewMemoryContainer(), 30*time.Second)
func NewMemory(data MemoryContainer, timeToClean time.Duration) *Memory {
	cache := Memory{
		data:   data,
		mutex:  sync.RWMutex{},
		ticker: time.NewTicker(timeToClean),
		done:   make(chan bool),
	}

	go cache.cleanup()

	return &cache
}

// cleanup runs in its own goroutine, periodically scanning the entire map for
// expired items.  Complexity is O(totalItems) but the operation is spread out in
// time thanks to the ticker.
func (m *Memory) cleanup() {
	for {
		select {
		case <-m.ticker.C:
			m.mutex.Lock()

			for mainKey, mainValue := range m.data {
				for childKey, childValue := range mainValue {
					if childValue.isExpired() {
						delete(m.data[mainKey], childKey)

						if m.data.decrementLen(mainKey) == 0 {
							// remove empty top‑level map to free memory and keep Len accurate
							delete(m.data, mainKey)

							break
						}
					}
				}
			}

			m.mutex.Unlock()
		case <-m.done:
			return
		}
	}
}

// Raw returns the underlying MemoryContainer.
//
// Example:
//
//	raw := mem.Raw()
func (m *Memory) Raw() MemoryContainer {
	return m.data
}

// HSetEX implementation for Memory.
//
// Example:
//
//	_ = mem.HSetEX(ctx, "main", "field", "val", time.Minute)
func (m *Memory) HSetEX(
	_ context.Context,
	mainKey string,
	childKey string,
	value string,
	ttl time.Duration,
) yaerrors.Error {
	m.mutex.Lock()

	defer m.mutex.Unlock()

	childMap, err := m.data.getChildMap(mainKey)
	if err != nil {
		childMap = make(map[string]*memoryCacheItem)

		m.data[mainKey] = childMap
	}

	childMap[childKey] = newMemoryCacheItemEX(value, time.Now().Add(ttl))

	m.data.incrementLen(mainKey)

	return nil
}

// HGet implementation for Memory.
//
// Example:
//
//	value, _ := memory.HGet(ctx, "main", "field")
func (m *Memory) HGet(
	_ context.Context,
	mainKey string,
	childKey string,
) (string, yaerrors.Error) {
	m.mutex.RLock()

	defer m.mutex.RUnlock()

	childMap, err := m.data.getChildMap(mainKey)
	if err != nil {
		return "", err.Wrap("[MEMORY] failed to get map item")
	}

	value, err := childMap.get(childKey)
	if err != nil {
		return "", err.Wrap("[MEMORY] failed to get map item")
	}

	return value, nil
}

// HGetAll implementation for Memory.
//
// Example:
//
//	main, _ := memory.HGetAll(ctx, "main")
func (m *Memory) HGetAll(
	_ context.Context,
	mainKey string,
) (map[string]string, yaerrors.Error) {
	m.mutex.RLock()

	defer m.mutex.RUnlock()

	childMap, err := m.data.getChildMap(mainKey)
	if err != nil {
		return nil, err.Wrap("[MEMORY] failed to get all map items")
	}

	result := make(map[string]string)

	for key, value := range childMap {
		if key != yaMapLen {
			result[key] = value.Value
		}
	}

	return result, nil
}

// HGetDelSingle implementation for Memory.
//
// Example:
//
//	value, _ := mem.HGetDelSingle(ctx, "jobs", "id‑1")
func (m *Memory) HGetDelSingle(
	_ context.Context,
	mainKey string,
	childKey string,
) (string, yaerrors.Error) {
	m.mutex.Lock()

	defer m.mutex.Unlock()

	childMap, err := m.data.getChildMap(mainKey)
	if err != nil {
		return "", err.Wrap("[MEMORY] failed to get and delete item")
	}

	value, ok := childMap[childKey]
	if !ok {
		return "", yaerrors.FromString(http.StatusInternalServerError, "[MEMORY] childKey not found in childMap")
	}

	delete(childMap, childKey)

	m.data.decrementLen(mainKey)

	return value.Value, nil
}

// HLen implements [Cache.HLen] for the in‑memory back‑end.
func (m *Memory) HLen(
	_ context.Context,
	mainKey string,
) (int64, yaerrors.Error) {
	m.mutex.RLock()

	defer m.mutex.RUnlock()

	return int64(m.data.getLen(mainKey)), nil
}

// HExist reports whether the childKey exists.
//
// Example:
//
//	ok, _ := memory.HExist(ctx, "k", "f")
func (m *Memory) HExist(
	_ context.Context,
	mainKey string,
	childKey string,
) (bool, yaerrors.Error) {
	m.mutex.RLock()

	defer m.mutex.RUnlock()

	childMap, err := m.data.getChildMap(mainKey)
	if err != nil {
		return false, err.Wrap("[MEMORY] failed to check exist")
	}

	return childMap.exist(childKey), nil
}

// HGetDelSingle atomically fetches and deletes.
//
// Example:
//
//	v, _ := memory.HGetDelSingle(ctx, "jobs", "id-1")
func (m *Memory) HDelSingle(
	_ context.Context,
	mainKey string,
	childKey string,
) yaerrors.Error {
	m.mutex.Lock()

	defer m.mutex.Unlock()

	childMap, err := m.data.getChildMap(mainKey)
	if err != nil {
		return err.Wrap("[MEMORY] failed to delete item")
	}

	delete(childMap, childKey)

	m.data.decrementLen(mainKey)

	return nil
}

// Ping always succeeds for the in‑memory backend.
//
// Example:
//
//	_ = memory.Ping(ctx)
func (m *Memory) Ping(_ context.Context) yaerrors.Error {
	return nil
}

// Close stops the sweeper and clears the map.
//
// Example:
//
//	_ = memory.Close()
func (m *Memory) Close() yaerrors.Error {
	m.mutex.Lock()

	defer m.mutex.Unlock()

	for k := range m.data {
		delete(m.data, k)
	}

	m.done <- true

	return nil
}

// memoryCacheItem is the atomic unit stored inside the in-memory cache.
// It keeps the actual value together with TTL metadata.
//
//   - Value      – payload the user saved.
//   - ExpiresAt  – absolute point in time when the item becomes stale
//     (ignored if Endless is true).
//   - Endless    – true means “no TTL at all”, so the item never expires.
//
// Example:
//
//	// A value without TTL.
//	item := newMemoryCacheItem("forever")
//	fmt.Println(item.Value)      // "forever"
//	fmt.Println(item.isExpired())// false
//
//	// A value that lives only one second.
//	item = newMemoryCacheItemEX("short-lived", time.Now().Add(time.Second))
//	time.Sleep(1100 * time.Millisecond)
//	fmt.Println(item.isExpired())// true
type memoryCacheItem struct {
	Value     string    // user payload
	ExpiresAt time.Time // TTL deadline (ignored when Endless)
	Endless   bool      // true → infinite lifetime
}

// newMemoryCacheItem returns a non-expiring cache item.
//
// Example:
//
//	item := newMemoryCacheItem("immutable")
//	_ = item // use item in a MemoryContainer
func newMemoryCacheItem(value string) *memoryCacheItem {
	return &memoryCacheItem{
		Value:   value,
		Endless: true,
	}
}

// newMemoryCacheItemEX returns a cache item that expires at the
// supplied timestamp.
//
// Example:
//
//	exp := time.Now().Add(5 * time.Minute)
//	item := newMemoryCacheItemEX("with-ttl", exp)
//	fmt.Println(item.Endless) // false
func newMemoryCacheItemEX(
	value string,
	expiresAt time.Time,
) *memoryCacheItem {
	return &memoryCacheItem{
		Value:     value,
		ExpiresAt: expiresAt,
		Endless:   false,
	}
}

// isExpired reports whether the item’s TTL has elapsed.
// Endless items are never reported as expired.
//
// Example:
//
//	item := newMemoryCacheItem("forever")
//	fmt.Println(item.isExpired()) // false
func (m *memoryCacheItem) isExpired() bool {
	return time.Now().After(m.ExpiresAt) && !m.Endless
}

// MemoryContainer is the backing store for the in-memory Cache
// implementation.  It is a two-level map:
//
//	mainKey ─┬─ childKey → *memoryCacheItem
//	          └─ YaMapLen (service key) → *memoryCacheItem(lenCounter)
//
// The service key **YaMapLen** keeps a running count of children to avoid
// walking the whole map on every HLen call.
//
// Example:
//
//	mc := NewMemoryContainer()
//	userMap := make(map[string]*memoryCacheItem)
//	userMap["name"] = newMemoryCacheItem("Alice")
//	mc["user:42"] = userMap
type (
	MemoryContainer      map[string]childMemoryContainer
	childMemoryContainer map[string]*memoryCacheItem
)

// NewMemoryContainer allocates an empty MemoryContainer.
//
// Example:
//
//	container := NewMemoryContainer()
//	fmt.Println(len(container)) // 0
func NewMemoryContainer() MemoryContainer {
	return make(MemoryContainer)
}

// get returns the payload stored under childKey or an error if absent.
//
// Example:
//
//	val, err := container["profile"].get("avatar")
//	if err != nil { … }
func (c childMemoryContainer) get(key string) (string, yaerrors.Error) {
	value, ok := c[key]
	if !ok {
		return "", yaerrors.FromString(
			http.StatusInternalServerError,
			fmt.Sprintf("[MEMORY] failed to get value in child map by `%s`", key),
		)
	}

	return value.Value, nil
}

// exist reports whether childKey is present.
//
// Example:
//
//	ok := container["profile"].exist("avatar")
func (c childMemoryContainer) exist(key string) bool {
	_, ok := c[key]

	return ok
}

// getLen returns how many “business” items (excluding YaMapLen) live under
// mainKey.  Zero is returned for non-existent maps.
//
// Example:
//
//	count := container.getLen("session")
//	fmt.Println(count) // 0
func (m MemoryContainer) getLen(mainKey string) int {
	childMap, yaerr := m.getChildMap(mainKey)
	if yaerr != nil {
		return 0
	}

	value, ok := childMap[yaMapLen]
	if !ok {
		m[mainKey][yaMapLen] = newMemoryCacheItem("0")

		return 0
	}

	count, err := strconv.Atoi(value.Value)
	if err != nil {
		return 0
	}

	return count
}

// incrementLen atomically increases the stored length counter for mainKey
// and returns the new value.
//
// Example:
//
//	newLen := container.incrementLen("session")
func (m MemoryContainer) incrementLen(mainKey string) int {
	value := m.getLen(mainKey)

	value++

	m[mainKey][yaMapLen].Value = strconv.Itoa(value)

	return value
}

// decrementLen decreases the length counter for mainKey (never below zero)
// and returns the new value.
//
// Example:
//
//	newLen := container.decrementLen("session")
func (m MemoryContainer) decrementLen(mainKey string) int {
	value := m.getLen(mainKey)

	value--

	m[mainKey][yaMapLen].Value = strconv.Itoa(value)

	return value
}

// getChildMap fetches the inner map for mainKey or returns an error if the
// key does not exist.
//
// Example:
//
//	child, err := container.getChildMap("user:42")
//	if err != nil { … }
func (m MemoryContainer) getChildMap(mainKey string) (childMemoryContainer, yaerrors.Error) {
	childMap, ok := m[mainKey]
	if !ok {
		return nil, yaerrors.FromString(
			http.StatusInternalServerError,
			fmt.Sprintf("[MEMORY] failed to get main map by `%s`", mainKey),
		)
	}

	return childMap, nil
}
