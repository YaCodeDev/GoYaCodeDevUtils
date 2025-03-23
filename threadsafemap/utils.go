package threadsafemap

// safetyCheck ensures that the internal map is initialized before any operations are performed.
func (m *ThreadSafeMap[K, V]) safetyCheck() {
	if m.data == nil {
		m.data = make(map[K]V)
	}
}
