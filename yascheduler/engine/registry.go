package engine

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaringbuffer"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yathreadsafeset"
)

// Sender is the seam an engine reaches an executor through. A network
// transport implements it over a connection; an in-process executor
// implements it over a channel, which is what lets one engine drive local
// and remote executors alike.
type Sender interface {
	// EnqueueMessage hands one message to the executor, refusing rather
	// than blocking when the outgoing queue is full.
	EnqueueMessage(msg protocol.Message) yaerrors.Error

	// CloseConnection tears the executor link down.
	CloseConnection()
}

// RegistryNotify is called after an executor of the given type joins, so
// work parked for want of an executor is reconsidered at once.
type RegistryNotify func(executorType protocol.ExecutorType)

type functionKey struct {
	name    protocol.FunctionName
	version protocol.FunctionVersion
}

// ExecutorEntry is one registered executor instance: what it runs, how much
// of it is in flight, and how to reach it.
type ExecutorEntry struct {
	instanceID    protocol.InstanceID
	executorType  protocol.ExecutorType
	generation    store.Generation
	capacity      store.Capacity
	functions     map[functionKey]protocol.FunctionSpec
	sender        Sender
	inFlight      *yathreadsafeset.ThreadSafeSet[protocol.AttemptID]
	closed        atomic.Bool
	lastHeartbeat atomic.Int64
}

// InstanceID reports which executor process this entry stands for.
func (e *ExecutorEntry) InstanceID() (id protocol.InstanceID) {
	return e.instanceID
}

// ExecutorType reports which pool this entry belongs to.
func (e *ExecutorEntry) ExecutorType() (executorType protocol.ExecutorType) {
	return e.executorType
}

// Generation reports the registration counter of this entry. A deregister
// carrying an older generation is a late message and is refused.
func (e *ExecutorEntry) Generation() (generation store.Generation) {
	return e.generation
}

// Supports reports whether this executor registered a function the given
// spec can run on. An empty signature on the spec matches any signature.
func (e *ExecutorEntry) Supports(spec *protocol.FunctionSpec) (supported bool) {
	local, found := e.functions[functionKey{name: spec.Name, version: spec.Version}]
	if !found {
		return false
	}

	if spec.InputSignature != "" && spec.InputSignature != local.InputSignature {
		return false
	}

	if spec.OutputSignature != "" && spec.OutputSignature != local.OutputSignature {
		return false
	}

	return true
}

// HasCapacity reports whether this executor accepts one more concurrent
// execution. Zero capacity means unbounded.
func (e *ExecutorEntry) HasCapacity() (available bool) {
	if e.capacity == 0 {
		return true
	}

	return e.inFlight.Length() < int(e.capacity)
}

// Alive reports whether this entry is still the live registration of its
// instance.
func (e *ExecutorEntry) Alive() (alive bool) {
	return !e.closed.Load()
}

// MarkClosed retires this entry, so selection skips it from now on.
func (e *ExecutorEntry) MarkClosed() {
	e.closed.Store(true)
}

// AddInFlight books one attempt against this executor's capacity.
func (e *ExecutorEntry) AddInFlight(attemptID protocol.AttemptID) {
	e.inFlight.Set(attemptID)
}

// ReleaseInFlight returns the capacity slot one attempt held.
func (e *ExecutorEntry) ReleaseInFlight(attemptID protocol.AttemptID) {
	e.inFlight.Pop(attemptID)
}

// InFlight counts the attempts currently booked against this executor.
func (e *ExecutorEntry) InFlight() (count store.InFlight) {
	return store.InFlight(e.inFlight.Length())
}

// Heartbeat records the last time this executor reported liveness.
func (e *ExecutorEntry) Heartbeat(now time.Time) {
	e.lastHeartbeat.Store(now.UnixNano())
}

// Enqueue hands one message to this executor's sender.
func (e *ExecutorEntry) Enqueue(msg protocol.Message) yaerrors.Error {
	if err := e.sender.EnqueueMessage(msg); err != nil {
		return err.Wrap(logTag + " failed to enqueue executor message")
	}

	return nil
}

// CloseConnection tears down this executor's link.
func (e *ExecutorEntry) CloseConnection() {
	e.sender.CloseConnection()
}

// ExecutorRegistry holds the connected executors and hands out the one that
// should run a given function next.
type ExecutorRegistry interface {
	// Register adds or replaces the registration of one instance and
	// returns the new entry plus the entry it displaced, if any.
	Register(
		instanceID protocol.InstanceID,
		executorType protocol.ExecutorType,
		capacity store.Capacity,
		functions []protocol.FunctionSpec,
		sender Sender,
	) (*ExecutorEntry, *ExecutorEntry)

	// Deregister removes one instance when the stated generation is still
	// the live one, and reports whether it did.
	Deregister(instanceID protocol.InstanceID, generation store.Generation) bool

	// Get returns the live entry of one instance.
	Get(instanceID protocol.InstanceID) (*ExecutorEntry, bool)

	// Select picks the next alive executor of the given type that supports
	// the function and has capacity, rotating fairly across the pool.
	Select(
		executorType protocol.ExecutorType,
		function *protocol.FunctionSpec,
	) (*ExecutorEntry, bool)

	// PoolSize counts the alive executors of one type.
	PoolSize(executorType protocol.ExecutorType) store.PoolSize

	// SupportsFunction reports whether any alive executor of the given type
	// registered a compatible function, whatever its current load.
	SupportsFunction(
		executorType protocol.ExecutorType,
		function *protocol.FunctionSpec,
	) bool

	// ConnectedCounts reports the alive pool size of every non-empty type.
	ConnectedCounts() map[protocol.ExecutorType]store.PoolSize

	// SetNotify installs the hook fired after an executor joins.
	SetNotify(notify RegistryNotify)

	// CloseAll retires every entry and closes every connection.
	CloseAll()
}

type executorRegistry struct {
	mu         sync.RWMutex
	pools      map[protocol.ExecutorType]*yaringbuffer.RingBuffer[protocol.InstanceID, *ExecutorEntry]
	entries    map[protocol.InstanceID]*ExecutorEntry
	generation store.Generation
	notify     RegistryNotify
}

// NewExecutorRegistry builds an empty registry.
func NewExecutorRegistry() (registry ExecutorRegistry) {
	return &executorRegistry{
		pools: make(
			map[protocol.ExecutorType]*yaringbuffer.RingBuffer[protocol.InstanceID, *ExecutorEntry],
		),
		entries: make(map[protocol.InstanceID]*ExecutorEntry),
	}
}

func (r *executorRegistry) SetNotify(notify RegistryNotify) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.notify = notify
}

func (r *executorRegistry) Register(
	instanceID protocol.InstanceID,
	executorType protocol.ExecutorType,
	capacity store.Capacity,
	functions []protocol.FunctionSpec,
	sender Sender,
) (*ExecutorEntry, *ExecutorEntry) {
	functionIndex := make(map[functionKey]protocol.FunctionSpec, len(functions))
	for _, spec := range functions {
		functionIndex[functionKey{name: spec.Name, version: spec.Version}] = spec
	}

	r.mu.Lock()

	r.generation++

	entry := &ExecutorEntry{
		instanceID:   instanceID,
		executorType: executorType,
		generation:   r.generation,
		capacity:     capacity,
		functions:    functionIndex,
		sender:       sender,
		inFlight:     yathreadsafeset.NewThreadSafeSet[protocol.AttemptID](),
	}

	replaced := r.entries[instanceID]

	if replaced != nil && replaced.executorType != executorType {
		if oldPool, found := r.pools[replaced.executorType]; found {
			oldPool.Remove(instanceID)
		}
	}

	r.entries[instanceID] = entry

	pool, found := r.pools[executorType]
	if !found {
		pool = yaringbuffer.New[protocol.InstanceID, *ExecutorEntry](0)
		r.pools[executorType] = pool
	}

	pool.Upsert(instanceID, entry)

	notify := r.notify

	r.mu.Unlock()

	if replaced != nil {
		replaced.MarkClosed()
	}

	if notify != nil {
		notify(executorType)
	}

	return entry, replaced
}

func (r *executorRegistry) Deregister(
	instanceID protocol.InstanceID,
	generation store.Generation,
) bool {
	r.mu.Lock()

	entry, found := r.entries[instanceID]
	if !found || entry.generation != generation {
		r.mu.Unlock()

		return false
	}

	delete(r.entries, instanceID)

	if pool, poolFound := r.pools[entry.executorType]; poolFound {
		pool.Remove(instanceID)
	}

	r.mu.Unlock()

	entry.MarkClosed()

	return true
}

func (r *executorRegistry) Get(
	instanceID protocol.InstanceID,
) (*ExecutorEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, found := r.entries[instanceID]

	return entry, found
}

func (r *executorRegistry) Select(
	executorType protocol.ExecutorType,
	function *protocol.FunctionSpec,
) (*ExecutorEntry, bool) {
	r.mu.RLock()
	pool, found := r.pools[executorType]
	r.mu.RUnlock()

	if !found {
		return nil, false
	}

	entry, matched := pool.NextMatch(func(candidate *ExecutorEntry) bool {
		return candidate.Alive() &&
			candidate.Supports(function) &&
			candidate.HasCapacity()
	})

	return entry, matched
}

func (r *executorRegistry) SupportsFunction(
	executorType protocol.ExecutorType,
	function *protocol.FunctionSpec,
) bool {
	r.mu.RLock()
	pool, found := r.pools[executorType]
	r.mu.RUnlock()

	if !found {
		return false
	}

	for _, entry := range pool.Values() {
		if entry.Alive() && entry.Supports(function) {
			return true
		}
	}

	return false
}

func (r *executorRegistry) PoolSize(
	executorType protocol.ExecutorType,
) store.PoolSize {
	r.mu.RLock()
	pool, found := r.pools[executorType]
	r.mu.RUnlock()

	if !found {
		return 0
	}

	alive := store.PoolSize(0)

	for _, entry := range pool.Values() {
		if entry.Alive() {
			alive++
		}
	}

	return alive
}

func (r *executorRegistry) ConnectedCounts() map[protocol.ExecutorType]store.PoolSize {
	r.mu.RLock()
	defer r.mu.RUnlock()

	counts := make(map[protocol.ExecutorType]store.PoolSize, len(r.pools))

	for executorType, pool := range r.pools {
		size := store.PoolSize(0)

		for _, entry := range pool.Values() {
			if entry.Alive() {
				size++
			}
		}

		if size > 0 {
			counts[executorType] = size
		}
	}

	return counts
}

func (r *executorRegistry) CloseAll() {
	r.mu.Lock()
	entries := make([]*ExecutorEntry, 0, len(r.entries))

	for _, entry := range r.entries {
		entries = append(entries, entry)
	}

	r.mu.Unlock()

	for _, entry := range entries {
		entry.MarkClosed()
		entry.CloseConnection()
	}
}
