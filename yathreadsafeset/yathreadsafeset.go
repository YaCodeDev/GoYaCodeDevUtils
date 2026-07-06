package yathreadsafeset

import (
	"encoding/json"
	"fmt"
	"maps"
	"sync"
)

// ThreadSafeSet is a generic set implementation that supports concurrent read and write operations safely.
type ThreadSafeSet[K comparable] struct {
	data map[K]struct{}
	mu   sync.RWMutex
}

// NewThreadSafeSet returns a new instance of a thread-safe set with initialized internal storage.
//
// Example usage:
//
//	set := threadsafeset.NewThreadSafeSet[string]()
func NewThreadSafeSet[K comparable]() *ThreadSafeSet[K] {
	return &ThreadSafeSet[K]{
		data: make(map[K]struct{}),
	}
}

// Clear removes all values from the set, resetting its internal state.
//
// Example usage:
//
//	set := threadsafeset.NewThreadSafeSet[string]()
//	set.Set("value1")
//	fmt.Println(set.String()) // Outputs: ["value1"]
//	set.Clear()
//	fmt.Println(set.String()) // Outputs: []
func (m *ThreadSafeSet[K]) Clear() {
	m.safetyCheck()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.data = make(map[K]struct{})
}

// Copy returns a new copy of the current set's content to avoid concurrency issues.
//
// Example usage:
//
//	set := threadsafeset.NewThreadSafeSet[string]()
//	set.Set("value1")
//	copySet := set.Copy()
//	set.Delete("value1")
//	fmt.Println(copySet.String()) // Outputs: ["value1"]
func (m *ThreadSafeSet[K]) Copy() *ThreadSafeSet[K] {
	m.safetyCheck()

	m.mu.RLock()
	defer m.mu.RUnlock()

	copySet := NewThreadSafeSet[K]()
	maps.Copy(copySet.data, m.data)

	return copySet
}

// Delete removes the specified value from the set if it exists.
//
// Example usage:
//
//	set := threadsafeset.NewThreadSafeSet[string]()
//	set.Set("value1")
//	set.Delete("value1") // Removes "value1" from the set
func (m *ThreadSafeSet[K]) Delete(value K) {
	m.safetyCheck()

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.data, value)
}

// Has checks whether a given value exists in the set.
//
// Example usage:
//
//	set := threadsafeset.NewThreadSafeSet[string]()
//	set.Set("value1")
//	fmt.Println(set.Has("value1")) // Outputs: true
//	fmt.Println(set.Has("value2")) // Outputs: false
func (m *ThreadSafeSet[K]) Has(value K) bool {
	m.safetyCheck()

	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.data[value]

	return exists
}

// Iterate iterates over the set and calls the given function for each value.
//
// DEADLOCK: During iteration, it is forbidden to modify the set (add or remove values),
// failing to do so will result in a deadlock.
//
// Example usage:
//
//	set := threadsafeset.NewThreadSafeSet[string]()
//	set.Set("value1")
//	set.Set("value2")
//	set.Iterate(func(value string) {
//	    fmt.Println(value) // Outputs: value1, value2
//	})
func (m *ThreadSafeSet[K]) Iterate(fn func(K)) {
	m.safetyCheck()

	m.mu.RLock()
	defer m.mu.RUnlock()

	for k := range m.data {
		fn(k)
	}
}

// IterateOnCopy iterates over a copy of the set to avoid holding locks during iteration.
//
// DEADLOCK: During iteration, it is forbidden to modify the set (add or remove values),
// failing to do so will result in a deadlock.
//
// Example usage:
//
//	set := threadsafeset.NewThreadSafeSet[string]()
//	set.Set("value1")
//
//	set.IterateOnCopy(func(value string) {
//	    fmt.Println(value) // Outputs: value1
//	    time.Sleep(1 * time.Second) // Assume time-consuming processing
//	})
func (m *ThreadSafeSet[K]) IterateOnCopy(fn func(K)) {
	for k := range m.CopyRaw() {
		fn(k)
	}
}

// IterateWithBreak iterates through the set until the callback returns false, then breaks.
//
// DEADLOCK: During iteration, it is forbidden to modify the set (add or remove values),
// failing to do so will result in a deadlock.
//
// Example usage:
//
//	set := threadsafeset.NewThreadSafeSet[string]()
//	set.Set("value1")
//
//	set.IterateWithBreak(func(value string) bool {
//	    fmt.Println(value) // Outputs: value1
//	    return true // Continue iteration
//	})
func (m *ThreadSafeSet[K]) IterateWithBreak(fn func(K) bool) {
	m.safetyCheck()

	m.mu.RLock()
	defer m.mu.RUnlock()

	for k := range m.data {
		if !fn(k) {
			break
		}
	}
}

// Length returns the total number of values in the set.
//
// Example usage:
//
//	set := threadsafeset.NewThreadSafeSet[string]()
//	fmt.Println(set.Length()) // Outputs: 0
//	set.Set("value1")
//	fmt.Println(set.Length()) // Outputs: 1
func (m *ThreadSafeSet[K]) Length() int {
	m.safetyCheck()

	m.mu.RLock()
	defer m.mu.RUnlock()

	length := len(m.data)

	return length
}

// MarshalJSON provides a custom JSON marshaling implementation for the thread-safe set.
//
// Example usage:
//
//	set := threadsafeset.NewThreadSafeSet[string]()
//	set.Set("value1")
//	jsonData, err := json.Marshal(set)
//
//	if err != nil {
//	    // handle error
//	}
//
//	fmt.Println(string(jsonData)) // Outputs: ["value1"]
func (m *ThreadSafeSet[K]) MarshalJSON() ([]byte, error) {
	data, err := json.Marshal(m.Values())
	if err != nil {
		return nil, fmt.Errorf("failed to marshal set: %w", err)
	}

	return data, nil
}

// Pop removes and returns a boolean indicating if the value was found.
//
// Example usage:
//
//	set := threadsafeset.NewThreadSafeSet[string]()
//	set.Set("value1")
//	fmt.Println(set.String()) // Outputs: ["value1"]
//	popped := set.Pop("value1") // Removes "value1" from the set
//	fmt.Println(popped) // Outputs: true
//	fmt.Println(set.String()) // Outputs: []
func (m *ThreadSafeSet[K]) Pop(value K) bool {
	m.safetyCheck()

	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.data[value]
	if ok {
		delete(m.data, value)
	}

	return ok
}

// Set adds a value to the set.
//
// Example usage:
//
//	set := threadsafeset.NewThreadSafeSet[string]()
//	set.Set("value1") // Adds "value1" to the set
//	fmt.Println(set.String()) // Outputs: ["value1"]
func (m *ThreadSafeSet[K]) Set(value K) {
	m.safetyCheck()

	m.mu.Lock()
	defer m.mu.Unlock()

	m.data[value] = struct{}{}
}

// ImportFromMap imports values from a map into the set.
//
// Example usage:
//
//	set := threadsafeset.NewThreadSafeSet[string]()
//	src := map[string]struct{}{"value1": {}, "value2": {}}
//	set.ImportFromMap(src)
//	fmt.Println(set.String()) // Outputs: ["value1", "value2"]
func (m *ThreadSafeSet[K]) ImportFromMap(src map[K]struct{}) {
	m.safetyCheck()

	m.mu.Lock()
	defer m.mu.Unlock()

	maps.Copy(m.data, src)
}

func (m *ThreadSafeSet[K]) CopyRaw() map[K]struct{} {
	m.safetyCheck()

	m.mu.RLock()
	defer m.mu.RUnlock()

	copySet := make(map[K]struct{}, len(m.data))
	maps.Copy(copySet, m.data)

	return copySet
}

// String returns a pretty-printed JSON string representation of the set.
//
// Example usage:
//
//	set := threadsafeset.NewThreadSafeSet[string]()
//	set.Set("value1")
//	fmt.Println(set.String()) // Outputs: ["value1"]
func (m *ThreadSafeSet[K]) String() string {
	b, err := json.MarshalIndent(m.Values(), "", "  ")
	if err != nil {
		return "<error>"
	}

	return string(b)
}

// Values returns a slice of all values stored in the set.
//
// Example usage:
//
//	set := threadsafeset.NewThreadSafeSet[string]()
//	set.Set("value1")
//	values := set.Values()
//	fmt.Println(values) // Outputs: ["value1"]
func (m *ThreadSafeSet[K]) Values() []K {
	m.safetyCheck()

	m.mu.RLock()
	defer m.mu.RUnlock()

	values := make([]K, 0, len(m.data))
	for k := range m.data {
		values = append(values, k)
	}

	return values
}

// Intersect returns a slice of values that are present in both the set and the provided slice.
//
// Example usage:
//
//	set := threadsafeset.NewThreadSafeSet[string]()
//	set.Set("value1")
//	set.Set("value2")
//	other := threadsafeset.NewThreadSafeSet[string]()
//	other.Set("value2")
//	intersection := set.Intersect(other)
//	fmt.Println(intersection.String()) // Outputs: ["value2"]
func (m *ThreadSafeSet[K]) Intersect(other *ThreadSafeSet[K]) *ThreadSafeSet[K] {
	left := m.CopyRaw()
	right := other.CopyRaw()

	intersection := NewThreadSafeSet[K]()

	for k := range left {
		if _, ok := right[k]; ok {
			intersection.data[k] = struct{}{}
		}
	}

	return intersection
}

// DeleteMultiple removes multiple values from the set.
//
// Example usage:
//
//	set := threadsafeset.NewThreadSafeSet[string]()
//	set.Set("value1")
//	set.Set("value2")
//	set.DeleteMultiple([]string{"value1", "value2"})
func (m *ThreadSafeSet[K]) DeleteMultiple(values []K) {
	m.safetyCheck()

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, v := range values {
		delete(m.data, v)
	}
}

// IsEmpty checks if the set is empty.
//
// Example usage:
//
//	set := threadsafeset.NewThreadSafeSet[string]()
//	fmt.Println(set.IsEmpty()) // Outputs: true
//	set.Set("value1")
//	fmt.Println(set.IsEmpty()) // Outputs: false
func (m *ThreadSafeSet[K]) IsEmpty() bool {
	return m.Length() == 0
}

// IsEqual checks if the current set is equal to another set.
// Two sets are considered equal if they contain the same elements.
//
// Example usage:
//
//	set1 := threadsafeset.NewThreadSafeSet[string]()
//	set2 := threadsafeset.NewThreadSafeSet[string]()
//	set1.Set("value1")
//	set2.Set("value1")
//	fmt.Println(set1.IsEqual(set2)) // Outputs: true
//	set2.Set("value2")
//	fmt.Println(set1.IsEqual(set2)) // Outputs: false
func (m *ThreadSafeSet[K]) IsEqual(other *ThreadSafeSet[K]) bool {
	left := m.CopyRaw()
	right := other.CopyRaw()

	if len(left) != len(right) {
		return false
	}

	for k := range left {
		if _, ok := right[k]; !ok {
			return false
		}
	}

	return true
}

// Union returns a new set containing elements that are in either the current set or the other set.
//
// Example usage:
//
//	set1 := threadsafeset.NewThreadSafeSet[string]()
//	set2 := threadsafeset.NewThreadSafeSet[string]()
//	set1.Set("value1")
//	set2.Set("value2")
//	result := set1.Union(set2)
//	fmt.Println(result.String()) // Outputs: ["value1", "value2"]
func (m *ThreadSafeSet[K]) Union(other *ThreadSafeSet[K]) *ThreadSafeSet[K] {
	left := m.CopyRaw()
	right := other.CopyRaw()

	result := NewThreadSafeSet[K]()
	maps.Copy(result.data, left)
	maps.Copy(result.data, right)

	return result
}

// Difference returns a new set containing elements that are in the current set but not in the other set.
//
// Example usage:
//
//	set1 := threadsafeset.NewThreadSafeSet[string]()
//	set2 := threadsafeset.NewThreadSafeSet[string]()
//	set1.Set("value1")
//	set2.Set("value2")
//	result := set1.Difference(set2)
//	fmt.Println(result.String()) // Outputs: ["value1"]
func (m *ThreadSafeSet[K]) Difference(other *ThreadSafeSet[K]) *ThreadSafeSet[K] {
	left := m.CopyRaw()
	right := other.CopyRaw()

	result := NewThreadSafeSet[K]()

	for k := range left {
		if _, ok := right[k]; !ok {
			result.data[k] = struct{}{}
		}
	}

	return result
}

// SymmetricDifference returns a new set containing elements that are in either set but not in both.
//
// Example usage:
//
//	set1 := threadsafeset.NewThreadSafeSet[string]()
//	set2 := threadsafeset.NewThreadSafeSet[string]()
//	set1.Set("value1")
//	set2.Set("value2")
//	result := set1.SymmetricDifference(set2)
//	fmt.Println(result.String()) // Outputs: ["value1", "value2"]
func (m *ThreadSafeSet[K]) SymmetricDifference(other *ThreadSafeSet[K]) *ThreadSafeSet[K] {
	left := m.CopyRaw()
	right := other.CopyRaw()

	result := NewThreadSafeSet[K]()

	for k := range left {
		if _, ok := right[k]; !ok {
			result.data[k] = struct{}{}
		}
	}

	for k := range right {
		if _, ok := left[k]; !ok {
			result.data[k] = struct{}{}
		}
	}

	return result
}
