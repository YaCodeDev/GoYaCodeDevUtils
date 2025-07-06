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
	m.data = make(map[K]V)
	m.mu.Unlock()
}

// Copy returns a new copy of the current map's content to avoid concurrency issues.
func (m *ThreadSafeMap[K, V]) Copy() map[K]V {
	m.safetyCheck()
	m.mu.RLock()

	copyMap := make(map[K]V, len(m.data))
	maps.Copy(copyMap, m.data)
	m.mu.RUnlock()

	return copyMap
}

// Delete removes the specified key from the map if it exists.
func (m *ThreadSafeMap[K, V]) Delete(key K) {
	m.safetyCheck()
	m.mu.Lock()
	delete(m.data, key)
	m.mu.Unlock()
}

// Get retrieves the value for a key and a boolean indicating whether it was found.
func (m *ThreadSafeMap[K, V]) Get(key K) (V, bool) {
	m.safetyCheck()
	m.mu.RLock()
	val, exists := m.data[key]
	m.mu.RUnlock()

	return val, exists
}

// GetOrDefault returns the value for a key, or a provided default if the key doesn't exist.
func (m *ThreadSafeMap[K, V]) GetOrDefault(key K, def V) V {
	m.safetyCheck()
	m.mu.RLock()

	if val, ok := m.data[key]; ok {
		return val
	}

	m.mu.RUnlock()

	return def
}

// GetOrSet retrieves the value for a key or sets it to a default value if not found.
// It returns the existing or newly set value, along with a boolean indicating whether the key was found.
func (m *ThreadSafeMap[K, V]) GetOrSet(key K, value V) (V, bool) {
	m.safetyCheck()
	m.mu.Lock()

	existing, exists := m.data[key]
	if exists {
		m.mu.Unlock()

		return existing, true
	}

	m.data[key] = value
	m.mu.Unlock()

	return value, false
}

// Has checks whether a given key exists in the map.
func (m *ThreadSafeMap[K, V]) Has(key K) bool {
	m.safetyCheck()
	m.mu.RLock()
	_, exists := m.data[key]
	m.mu.RUnlock()

	return exists
}

// Iterate iterates over the map and calls the given function for each key-value pair.
func (m *ThreadSafeMap[K, V]) Iterate(fn func(K, V)) {
	m.safetyCheck()
	m.mu.RLock()

	defer func() {
		if r := recover(); r != nil {
			m.mu.RUnlock()
			panic(r)
		}
	}()

	for k, v := range m.data {
		fn(k, v)
	}

	m.mu.RUnlock()
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

	defer func() {
		if r := recover(); r != nil {
			m.mu.RUnlock()
			panic(r)
		}
	}()

	for k, v := range m.data {
		if !fn(k, v) {
			break
		}
	}

	m.mu.RUnlock()
}

// Keys returns a slice containing all keys currently in the map.
func (m *ThreadSafeMap[K, V]) Keys() []K {
	m.safetyCheck()
	m.mu.RLock()

	keys := make([]K, 0, len(m.data))
	for k := range m.data {
		keys = append(keys, k)
	}

	m.mu.RUnlock()

	return keys
}

// Length returns the total number of key-value pairs in the map.
func (m *ThreadSafeMap[K, V]) Length() int {
	m.safetyCheck()
	m.mu.RLock()
	length := len(m.data)
	m.mu.RUnlock()

	return length
}

// MarshalJSON provides a custom JSON marshaling implementation for the thread-safe map.
func (m *ThreadSafeMap[K, V]) MarshalJSON() ([]byte, error) {
	m.safetyCheck()
	m.mu.RLock()

	data, err := json.Marshal(m.data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal map: %w", err)
	}

	m.mu.RUnlock()

	return data, nil
}

// Pop removes and returns the value associated with the key. It returns a boolean indicating if the key was found.
func (m *ThreadSafeMap[K, V]) Pop(key K) (V, bool) {
	m.safetyCheck()
	m.mu.Lock()

	val, ok := m.data[key]
	if ok {
		delete(m.data, key)
	}

	m.mu.Unlock()

	return val, ok
}

// Set sets or updates the value for a given key.
func (m *ThreadSafeMap[K, V]) Set(key K, value V) {
	m.safetyCheck()
	m.mu.Lock()
	m.data[key] = value
	m.mu.Unlock()
}

// String returns a pretty-printed JSON string representation of the map.
func (m *ThreadSafeMap[K, V]) String() string {
	m.safetyCheck()
	m.mu.RLock()

	b, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return "<error>"
	}

	m.mu.RUnlock()

	return string(b)
}

// Update allows updating a key’s value using a user-supplied transformation function that receives the old value and a
// boolean flag indicating whether it existed.
func (m *ThreadSafeMap[K, V]) Update(key K, fn func(old V, exists bool) V) {
	m.safetyCheck()
	m.mu.Lock()
	old, exists := m.data[key]
	m.data[key] = fn(old, exists)
	m.mu.Unlock()
}

// Values returns a slice of all values stored in the map.
func (m *ThreadSafeMap[K, V]) Values() []V {
	m.safetyCheck()
	m.mu.RLock()

	values := make([]V, 0, len(m.data))
	for _, v := range m.data {
		values = append(values, v)
	}

	m.mu.RUnlock()

	return values
}
