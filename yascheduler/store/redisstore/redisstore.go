// Package redisstore persists the scheduler store on a redis-protocol
// backend, targeting Dragonfly compatibility: hashes, sorted sets, sets,
// strings, INCR, and EVAL lua only, with no modules, no keyspace
// notifications, and no key expiry, since the engine drives retention
// itself through ExpiredResults.
//
// Caller-controlled strings never shape a redis key name directly: job
// identifiers appear as fixed-width hex, minted execution and attempt
// identifiers as decimals, and instance identifiers as base64, so a
// hostile value carrying separators or extending another value as a
// prefix cannot collide with or escape the namespace. Executor-scoped job
// keys and occurrence identities live as fields inside one hash each,
// encoded with an unambiguous length or fixed-width prefix.
package redisstore

import (
	"sync"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
	"github.com/redis/go-redis/v9"
)

var _ store.Store = (*Store)(nil)

// Config shapes a redis store. A zero field applies its package default.
type Config struct {
	// KeyPrefix namespaces every redis key the store touches.
	KeyPrefix KeyPrefix

	// MaxResults caps how many pending results the store holds in total.
	MaxResults store.OccurrenceCount

	// MaxResultsPerInstance caps how many pending results the store holds
	// for one submitting instance.
	MaxResultsPerInstance store.OccurrenceCount
}

// Store is a redis-backed store.Store. Every multi-step invariant runs as
// one lua script, so concurrent schedulers over one backend never observe
// a half-applied write.
type Store struct {
	client *redis.Client
	keys   keySet

	maxResults            store.OccurrenceCount
	maxResultsPerInstance store.OccurrenceCount

	mu    sync.RWMutex
	clock func() time.Time
}

// NewStore builds a store over an already-configured redis client,
// namespaced and bounded by the given config.
func NewStore(client *redis.Client, config Config) (created *Store) {
	prefix := string(config.KeyPrefix)
	if prefix == "" {
		prefix = string(DefaultKeyPrefix)
	}

	maxResults := config.MaxResults
	if maxResults == 0 {
		maxResults = DefaultMaxResults
	}

	maxResultsPerInstance := config.MaxResultsPerInstance
	if maxResultsPerInstance == 0 {
		maxResultsPerInstance = DefaultMaxResultsPerInstance
	}

	return &Store{
		client: client,
		keys: keySet{
			jobKeys:          prefix + keyPartJobKeys,
			jobsEnabled:      prefix + keyPartJobsEnabled,
			executionCounter: prefix + keyPartExecutionCounter,
			occurrences:      prefix + keyPartOccurrences,
			wake:             prefix + keyPartWake,
			lease:            prefix + keyPartLease,
			attemptCounter:   prefix + keyPartAttemptCounter,
			resultsCreated:   prefix + keyPartResultsCreated,

			jobPrefix:             prefix + keyPartJob,
			executionPrefix:       prefix + keyPartExecution,
			statePrefix:           prefix + keyPartState,
			attemptPrefix:         prefix + keyPartAttempt,
			instanceAttemptPrefix: prefix + keyPartInstanceAttempts,
			resultPrefix:          prefix + keyPartResult,
			instanceResultPrefix:  prefix + keyPartInstanceResults,
		},
		maxResults:            maxResults,
		maxResultsPerInstance: maxResultsPerInstance,
		clock:                 func() time.Time { return time.Now().UTC() },
	}
}

// SetClock replaces the time source every stored timestamp is read from.
func (s *Store) SetClock(clock func() time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.clock = clock
}

func (s *Store) now() (instant time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.clock()
}
