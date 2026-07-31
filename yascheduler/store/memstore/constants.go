package memstore

import "github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"

const (
	// DefaultMaxResults caps how many pending results the store holds in
	// total. At protocol.DefaultMaxResultBytes per result this bounds the
	// pending-result memory at 64 MiB.
	DefaultMaxResults store.OccurrenceCount = 1024

	// DefaultMaxResultsPerInstance caps how many pending results the store
	// holds for one submitting instance, so a single disconnected
	// submitter cannot consume the whole budget.
	DefaultMaxResultsPerInstance store.OccurrenceCount = 256
)

const logTag = "[SCHEDULERMEMSTORE]"
