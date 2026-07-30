package yaringbuffer

const (
	// DefaultInitialCapacity is the backing-storage capacity applied when New
	// receives a non-positive initial capacity, or when a zero-value
	// RingBuffer lazily initializes itself on first use.
	DefaultInitialCapacity = 8

	// GrowthFactor multiplies the backing-storage capacity each time the ring
	// buffer runs out of room for a new entry.
	GrowthFactor = 2
)
