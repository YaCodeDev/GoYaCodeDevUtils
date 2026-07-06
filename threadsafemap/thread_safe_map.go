package threadsafemap

import (
	"encoding/json"
	"fmt"
	"maps"
	"sync"
)

// ThreadSafeMap is a generic map implementation that supports concurrent read and write operations safely.
type ThreadSafeMap[K comparable, V any] struct {
	data map[K]V
	mu   sync.RWMutex
}

// NewThreadSafeMap returns a new instance of a thread-safe map with initialized internal storage.
func NewThreadSafeMap[K comparable, V any]() *ThreadSafeMap[K, V] {
	return &ThreadSafeMap[K, V]{
		data: make(map[K]V),
	}
}

// Clear removes all key-value pairs from the map, resetting its internal state.
func (m *ThreadSafeMap[K, V]) Clear() {
	m.safetyCheck()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[K]V)
}

// Copy returns a new copy of the current map's content to avoid concurrency issues.
func (m *ThreadSafeMap[K, V]) Copy() map[K]V {
	m.safetyCheck()

	m.mu.RLock()
	defer m.mu.RUnlock()

	copyMap := make(map[K]V, len(m.data))
	maps.Copy(copyMap, m.data)

	return copyMap
}

// Delete removes the specified key from the map if it exists.
func (m *ThreadSafeMap[K, V]) Delete(key K) {
	m.safetyCheck()

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, key)
}

// Get retrieves the value for a key and a boolean indicating whether it was found.
func (m *ThreadSafeMap[K, V]) Get(key K) (V, bool) {
	m.safetyCheck()

	m.mu.RLock()
	defer m.mu.RUnlock()

	val, exists := m.data[key]

	return val, exists
}

// GetOrDefault returns the value for a key, or a provided default if the key doesn't exist.
func (m *ThreadSafeMap[K, V]) GetOrDefault(key K, def V) V {
	m.safetyCheck()

	m.mu.RLock()
	defer m.mu.RUnlock()

	if val, ok := m.data[key]; ok {
		return val
	}

	return def
}

// GetOrSet retrieves the value for a key or sets it to a default value if not found.
// It returns the existing or newly set value, along with a boolean indicating whether the key was found.
func (m *ThreadSafeMap[K, V]) GetOrSet(key K, value V) (V, bool) {
	m.safetyCheck()

	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.data[key]
	if exists {
		return existing, true
	}

	m.data[key] = value

	return value, false
}

// Has checks whether a given key exists in the map.
func (m *ThreadSafeMap[K, V]) Has(key K) bool {
	m.safetyCheck()

	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.data[key]

	return exists
}

// Iterate iterates over the map and calls the given function for each key-value pair.
func (m *ThreadSafeMap[K, V]) Iterate(fn func(K, V)) {
	m.safetyCheck()

	m.mu.RLock()
	defer m.mu.RUnlock()

	for k, v := range m.data {
		fn(k, v)
	}
}

// IterateOnCopy iterates over a copy of the map to avoid holding locks during iteration.
func (m *ThreadSafeMap[K, V]) IterateOnCopy(fn func(K, V)) {
	for k, v := range m.Copy() {
		fn(k, v)
	}
}

// IterateWithBreak iterates through the map until the callback returns false, then breaks.
func (m *ThreadSafeMap[K, V]) IterateWithBreak(fn func(K, V) bool) {
	m.safetyCheck()

	m.mu.RLock()
	defer m.mu.RUnlock()

	for k, v := range m.data {
		if !fn(k, v) {
			break
		}
	}
}

// Keys returns a slice containing all keys currently in the map.
func (m *ThreadSafeMap[K, V]) Keys() []K {
	m.safetyCheck()

	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := make([]K, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}

	return keys
}

// Length returns the total number of key-value pairs in the map.
func (m *ThreadSafeMap[K, V]) Length() int {
	m.safetyCheck()

	m.mu.RLock()
	defer m.mu.RUnlock()

	length := len(m.data)

	return length
}

// MarshalJSON provides a custom JSON marshaling implementation for the thread-safe map.
func (m *ThreadSafeMap[K, V]) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(m.Copy())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal map: %w", err)
	}

	return data, nil
}

// Pop removes and returns the value associated with the key. It returns a boolean indicating if the key was found.
func (m *ThreadSafeMap[K, V]) Pop(key K) (V, bool) {
	m.safetyCheck()

	m.mu.Lock()
	defer m.mu.Unlock()

	val, ok := m.data[key]
	if ok {
		delete(m.data, key)
	}

	return val, ok
}

// Set sets or updates the value for a given key.
func (m *ThreadSafeMap[K, V]) Set(key K, value V) {
	m.safetyCheck()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[key] = value
}

// ImportFromMap imports values from a map into the thread-safe map.
//
// Example usage:
//
//	m := threadsafemap.NewThreadSafeMap[string, string]()
//	src := map[string]string{"key1": "value1", "key2": "value2"}
//	m.ImportFromMap(src)
//	println(m.String()) // Outputs: {"key1":"value1","key2":"value2"}
func (m *ThreadSafeMap[K, V]) ImportFromMap(src map[K]V) {
	m.safetyCheck()

	m.mu.Lock()
	defer m.mu.Unlock()

	maps.Copy(m.data, src)
}

// String returns a pretty-printed JSON string representation of the map.
func (m *ThreadSafeMap[K, V]) String() string {
	b, err := json.MarshalIndent(m.Copy(), "", "  ")
	if err != nil {
		return "<error>"
	}

	return string(b)
}

// Update allows updating a key’s value using a user-supplied transformation function that receives the old value and a
// boolean flag indicating whether it existed.
func (m *ThreadSafeMap[K, V]) Update(key K, fn func(old V, exists bool) V) {
	m.safetyCheck()

	m.mu.Lock()
	defer m.mu.Unlock()

	old, exists := m.data[key]
	m.data[key] = fn(old, exists)
}

// Values returns a slice of all values stored in the map.
func (m *ThreadSafeMap[K, V]) Values() []V {
	m.safetyCheck()

	m.mu.RLock()
	defer m.mu.RUnlock()

	values := make([]V, 0, len(m.data))
	for _, v := range m.data {
		values = append(values, v)
	}

	return values
}
