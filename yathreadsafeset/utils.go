package yathreadsafeset

// safetyCheck ensures that the internal set is initialized before any operations are performed.
func (m *ThreadSafeSet[K]) safetyCheck() {
	m.mu.RLock()
	initialized := m.data != nil
	m.mu.RUnlock()

	if initialized {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.data == nil {
		m.data = make(map[K]struct{})
	}
}
