package yacache

// safetyCheck ensures that the internal maps are initialized before any operation touches them.
func (m *Memory) safetyCheck() {
	m.mutex.RLock()
	initialized := m.inner.HMap != nil && m.inner.Map != nil
	m.mutex.RUnlock()

	if initialized {
		return
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.inner.HMap == nil {
		m.inner.HMap = make(map[string]childMemoryContainer)
	}

	if m.inner.Map == nil {
		m.inner.Map = make(map[string]*memoryCacheItem)
	}
}
