package yathreadsafeset

// safetyCheck ensures that the internal set is initialized before any operations are performed.
func (m *ThreadSafeSet[K]) safetyCheck() {
	if m.data == nil {
		m.data = make(map[K]struct{})
	}
}
