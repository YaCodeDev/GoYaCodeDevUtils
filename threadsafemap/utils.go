package threadsafemap

// safetyCheck ensures that the internal map is initialized before any operations are performed.
func (m *ThreadSafeMap[K, V]) safetyCheck() {
	m.mu.RLock()
	initialized := m.data != nil
	m.mu.RUnlock()

	if initialized {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.data == nil {
		m.data = make(map[K]V)
	}
}
