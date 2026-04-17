package yatgbot

// FeatureFlags toggles optional YaTgBot behaviors without changing defaults.
type FeatureFlags uint8

const (
	// FeatureSequentialUpdates ensures async updates that share a user or chat/channel
	// are processed in arrival order, one-by-one.
	FeatureSequentialUpdates FeatureFlags = 1 << iota
)

// Has reports whether the provided feature flag is enabled.
func (f FeatureFlags) Has(flag FeatureFlags) bool {
	return f&flag == flag
}
