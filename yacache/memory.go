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
	"weak"

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
	inner  MemoryContainer // nested map mainKey → childKey → *memoryCacheItem
	mutex  sync.RWMutex    // guards *all* access to data
	ticker *time.Ticker    // drives the cleanup loop
	done   chan struct{}   // signals the goroutine to exit on Close()
}

// NewMemory builds a new [Memory] cache instance and immediately starts the
// background sweeper.
//
//	data        – caller‑provided map; pass NewMemoryContainer() for an empty cache
//	tickToClean – sweep interval; choose a value >> typical TTL to amortise cost
//
// Example:
//
//	memory := cache.NewMemory(cache.NewMemoryContainer(), 30*time.Second)
func NewMemory(data MemoryContainer, tickToClean time.Duration) *Memory {
	cache := Memory{
		inner:  data,
		mutex:  sync.RWMutex{},
		ticker: time.NewTicker(tickToClean),
		done:   make(chan struct{}),
	}

	go cleanup(weak.Make(&cache), tickToClean, cache.done)

	return &cache
}

// cleanup runs in its own goroutine, periodically scanning the entire map for
// expired items.  Complexity is O(totalItems) but the operation is spread out in
// time thanks to the ticker.
func cleanup(
	pointer weak.Pointer[Memory],
	tickToClean time.Duration,
	done <-chan struct{},
) {
	ticker := time.NewTicker(tickToClean)

	for {
		select {
		case <-ticker.C:
			memory := pointer.Value()

			if memory == nil {
				return
			}

			memory.mutex.Lock()

			for mainKey, mainValue := range memory.inner.HMap {
				for childKey, childValue := range mainValue {
					if childValue.isExpired() {
						delete(memory.inner.HMap[mainKey], childKey)

						if memory.inner.decrementLen(mainKey) == 0 {
							// remove empty top‑level map to free memory and keep Len accurate
							delete(memory.inner.HMap, mainKey)

							break
						}
					}
				}
			}

			for key, value := range memory.inner.Map {
				if value.isExpired() {
					delete(memory.inner.Map, key)
				}
			}

			memory.mutex.Unlock()
		case <-done:
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
	return m.inner
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

	childMap, err := m.inner.getChildMap(mainKey, ErrFailedToHSetEx)
	if err != nil {
		childMap = make(map[string]*memoryCacheItem)

		m.inner.HMap[mainKey] = childMap
	}

	childMap[childKey] = newMemoryCacheItemEX(value, time.Now().Add(ttl))

	m.inner.incrementLen(mainKey)

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

	childMap, err := m.inner.getChildMap(mainKey, ErrFailedToGetValue)
	if err != nil {
		return "", err.Wrap("[MEMORY] failed to get map item")
	}

	value, err := childMap.get(childKey, ErrNotFoundValue)
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

	childMap, err := m.inner.getChildMap(mainKey, ErrFailedToGetValues)
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

	childMap, err := m.inner.getChildMap(mainKey, ErrFailedToGetDeleteSingle)
	if err != nil {
		return "", err.Wrap("[MEMORY] failed to get and delete item")
	}

	value, ok := childMap[childKey]
	if !ok {
		return "", yaerrors.FromError(
			http.StatusInternalServerError,
			ErrNotFoundValue,
			fmt.Sprintf("[MEMORY] failed `HGETDEL` by %s:%s", mainKey, childKey),
		)
	}

	delete(childMap, childKey)

	m.inner.decrementLen(mainKey)

	return value.Value, nil
}

// HLen implements [Cache.HLen] for the in‑memory back‑end.
func (m *Memory) HLen(
	_ context.Context,
	mainKey string,
) (int64, yaerrors.Error) {
	m.mutex.RLock()

	defer m.mutex.RUnlock()

	return int64(m.inner.getLen(mainKey)), nil
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

	childMap, err := m.inner.getChildMap(mainKey, ErrFailedToHExist)
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

	childMap, err := m.inner.getChildMap(mainKey, ErrFailedToDeleteSingle)
	if err != nil {
		return err.Wrap("[MEMORY] failed to delete item")
	}

	delete(childMap, childKey)

	m.inner.decrementLen(mainKey)

	return nil
}

func (m *Memory) Set(
	_ context.Context,
	key string,
	value string,
	ttl time.Duration,
) yaerrors.Error {
	m.mutex.Lock()

	defer m.mutex.Unlock()

	m.inner.Map[key] = newMemoryCacheItemEX(value, time.Now().Add(ttl))

	return nil
}

func (m *Memory) Get(
	_ context.Context,
	key string,
) (string, yaerrors.Error) {
	m.mutex.RLock()

	defer m.mutex.RUnlock()

	value, ok := m.inner.Map[key]
	if !ok {
		return "", yaerrors.FromError(
			http.StatusInternalServerError,
			ErrFailedToGetValue,
			fmt.Sprintf("[MEMORY] failed to get value in key: %s", key),
		)
	}

	return value.Value, nil
}

func (m *Memory) MGet(
	_ context.Context,
	keys ...string,
) (map[string]string, yaerrors.Error) {
	m.mutex.RLock()

	defer m.mutex.RUnlock()

	result := make(map[string]string)

	for _, key := range keys {
		value, ok := m.inner.Map[key]
		if !ok {
			return nil, yaerrors.FromError(
				http.StatusInternalServerError,
				ErrFailedToMGetValues,
				fmt.Sprintf("[MEMORY] failed to get value in key: %s", key),
			)
		}

		result[key] = value.Value
	}

	return result, nil
}

func (m *Memory) GetDel(
	_ context.Context,
	key string,
) (string, yaerrors.Error) {
	m.mutex.Lock()

	defer m.mutex.Unlock()

	value, ok := m.inner.Map[key]
	if !ok {
		return "", yaerrors.FromError(
			http.StatusInternalServerError,
			ErrFailedToGetDelValue,
			fmt.Sprintf("[MEMORY] failed to get and delete value in key: %s", key),
		)
	}

	delete(m.inner.Map, key)

	return value.Value, nil
}

func (m *Memory) Exists(
	_ context.Context,
	key string,
) (bool, yaerrors.Error) {
	m.mutex.RLock()

	defer m.mutex.RUnlock()

	_, ok := m.inner.Map[key]

	return ok, nil
}

func (m *Memory) Del(
	_ context.Context,
	key string,
) yaerrors.Error {
	m.mutex.Lock()

	defer m.mutex.Unlock()

	delete(m.inner.Map, key)

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

	for k := range m.inner.HMap {
		delete(m.inner.HMap, k)
	}

	for k := range m.inner.Map {
		delete(m.inner.Map, k)
	}

	m.done <- struct{}{}

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
//	          └─ yaMapLen (service key) → *memoryCacheItem(lenCounter)
//
// The service key **yaMapLen** keeps a running count of children to avoid
// walking the whole map on every HLen call.
//
// Example:
//
//	mc := NewMemoryContainer()
//	userMap := make(map[string]*memoryCacheItem)
//	userMap["name"] = newMemoryCacheItem("Alice")
//	mc["user:42"] = userMap
type childMemoryContainer map[string]*memoryCacheItem

type MemoryContainer struct {
	HMap map[string]childMemoryContainer
	Map  map[string]*memoryCacheItem
}

// NewMemoryContainer allocates an empty MemoryContainer.
//
// Example:
//
//	container := NewMemoryContainer()
//	fmt.Println(len(container)) // 0
func NewMemoryContainer() MemoryContainer {
	return MemoryContainer{
		HMap: make(map[string]childMemoryContainer),
		Map:  make(map[string]*memoryCacheItem),
	}
}

// get returns the payload stored under childKey or an error if absent.
//
// Example:
//
//	val, err := container["profile"].get("avatar")
//	if err != nil { … }
func (c childMemoryContainer) get(
	key string,
	wrapErr error,
) (string, yaerrors.Error) {
	value, ok := c[key]
	if !ok {
		return "", yaerrors.FromError(
			http.StatusInternalServerError,
			wrapErr,
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

// getLen returns how many “business” items (excluding yaMapLen) live under
// mainKey.  Zero is returned for non-existent maps.
//
// Example:
//
//	count := container.getLen("session")
//	fmt.Println(count) // 0
func (m MemoryContainer) getLen(mainKey string) int {
	childMap, yaerr := m.getChildMap(mainKey, ErrFailedToGetLen)
	if yaerr != nil {
		return 0
	}

	value, ok := childMap[yaMapLen]
	if !ok {
		m.HMap[mainKey][yaMapLen] = newMemoryCacheItem("0")

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

	m.HMap[mainKey][yaMapLen].Value = strconv.Itoa(value)

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

	m.HMap[mainKey][yaMapLen].Value = strconv.Itoa(value)

	return value
}

// getChildMap fetches the inner map for mainKey or returns an error if the
// key does not exist.
//
// Example:
//
//	child, err := container.getChildMap("user:42")
//	if err != nil { … }
func (m MemoryContainer) getChildMap(
	mainKey string,
	wrapErr error,
) (childMemoryContainer, yaerrors.Error) {
	childMap, ok := m.HMap[mainKey]
	if !ok {
		return nil, yaerrors.FromError(
			http.StatusInternalServerError,
			wrapErr,
			fmt.Sprintf("[MEMORY] failed to get child map by `%s`", mainKey),
		)
	}

	return childMap, nil
}
