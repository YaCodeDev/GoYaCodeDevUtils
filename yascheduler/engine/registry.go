package engine

import (
	"net/http"
	"slices"
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

// RegistryChange describes what just became routable: the pool an executor
// joined, and the routing labels that arrived with it. One shape covers both
// wake-up causes, so a parked execution waiting for a pool and one waiting
// for a label are reconsidered through the same callback.
type RegistryChange struct {
	ExecutorType protocol.ExecutorType
	Labels       []protocol.Label
}

// RegistryNotify is called after an executor joins or announces labels, so
// work parked for want of either is reconsidered at once.
type RegistryNotify func(change RegistryChange)

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
	labels        *yathreadsafeset.ThreadSafeSet[protocol.Label]
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

// Labels reports the routing labels this executor currently announces.
func (e *ExecutorEntry) Labels() (labels []protocol.Label) {
	return e.labels.Values()
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
	// returns the new entry plus the entry it displaced, if any. Labels are
	// client-owned state replayed on every registration, so a reconnect
	// restores the pins a connection held rather than silently dropping
	// them. Empty labels are dropped; the label count of one registration
	// is already bounded by the wire decoder, which is where an oversized
	// list can still be refused with a reason.
	Register(
		instanceID protocol.InstanceID,
		executorType protocol.ExecutorType,
		capacity store.Capacity,
		functions []protocol.FunctionSpec,
		labels []protocol.Label,
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

	// SelectLabeled picks the next alive executor that announces the label,
	// belongs to the given type, supports the function, and has capacity.
	// A label refines routing inside a pool; it never replaces the type and
	// function match.
	SelectLabeled(
		executorType protocol.ExecutorType,
		function *protocol.FunctionSpec,
		label protocol.Label,
	) (*ExecutorEntry, bool)

	// PoolSize counts the alive executors of one type.
	PoolSize(executorType protocol.ExecutorType) store.PoolSize

	// LabelPoolSize counts the alive executors announcing one label.
	LabelPoolSize(label protocol.Label) store.PoolSize

	// UpdateLabels revises the labels of one live connection, applying
	// withdrawals last, and reports how many labels the connection carries
	// afterwards. A refused update leaves the label set untouched and
	// reports the count it still holds.
	UpdateLabels(
		instanceID protocol.InstanceID,
		announce []protocol.Label,
		withdraw []protocol.Label,
	) (store.LabelCount, yaerrors.Error)

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
	labelPools map[protocol.Label]*yaringbuffer.RingBuffer[protocol.InstanceID, *ExecutorEntry]
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
		labelPools: make(
			map[protocol.Label]*yaringbuffer.RingBuffer[protocol.InstanceID, *ExecutorEntry],
		),
		entries: make(map[protocol.InstanceID]*ExecutorEntry),
	}
}

// attachLabels puts one instance into the ring of every given label,
// creating rings on demand. The caller holds the registry lock.
func (r *executorRegistry) attachLabels(
	entry *ExecutorEntry,
	labels []protocol.Label,
) {
	for _, label := range labels {
		ring, found := r.labelPools[label]
		if !found {
			ring = yaringbuffer.New[protocol.InstanceID, *ExecutorEntry](0)
			r.labelPools[label] = ring
		}

		ring.Upsert(entry.instanceID, entry)
	}
}

// detachLabels takes one instance out of the ring of every given label and
// drops rings that empty out, so a departed executor stops being reachable
// from a label it no longer announces. A missed call here leaks a dead entry
// into a ring that only registration churn ever surfaces. The caller holds
// the registry lock.
func (r *executorRegistry) detachLabels(
	instanceID protocol.InstanceID,
	labels []protocol.Label,
) {
	for _, label := range labels {
		ring, found := r.labelPools[label]
		if !found {
			continue
		}

		ring.Remove(instanceID)

		if ring.Len() == 0 {
			delete(r.labelPools, label)
		}
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
	labels []protocol.Label,
	sender Sender,
) (*ExecutorEntry, *ExecutorEntry) {
	functionIndex := make(map[functionKey]protocol.FunctionSpec, len(functions))
	for _, spec := range functions {
		functionIndex[functionKey{name: spec.Name, version: spec.Version}] = spec
	}

	labelSet := yathreadsafeset.NewThreadSafeSet[protocol.Label]()

	for _, label := range labels {
		if label == "" {
			continue
		}

		labelSet.Set(label)
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
		labels:       labelSet,
		inFlight:     yathreadsafeset.NewThreadSafeSet[protocol.AttemptID](),
	}

	replaced := r.entries[instanceID]

	if replaced != nil {
		if replaced.executorType != executorType {
			if oldPool, found := r.pools[replaced.executorType]; found {
				oldPool.Remove(instanceID)
			}
		}

		r.detachLabels(instanceID, replaced.labels.Difference(labelSet).Values())
	}

	r.entries[instanceID] = entry

	pool, found := r.pools[executorType]
	if !found {
		pool = yaringbuffer.New[protocol.InstanceID, *ExecutorEntry](0)
		r.pools[executorType] = pool
	}

	pool.Upsert(instanceID, entry)

	announced := labelSet.Values()

	r.attachLabels(entry, announced)

	notify := r.notify

	r.mu.Unlock()

	if replaced != nil {
		replaced.MarkClosed()
	}

	if notify != nil {
		notify(RegistryChange{ExecutorType: executorType, Labels: announced})
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

	r.detachLabels(instanceID, entry.Labels())

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

func (r *executorRegistry) SelectLabeled(
	executorType protocol.ExecutorType,
	function *protocol.FunctionSpec,
	label protocol.Label,
) (*ExecutorEntry, bool) {
	r.mu.RLock()
	ring, found := r.labelPools[label]
	r.mu.RUnlock()

	if !found {
		return nil, false
	}

	entry, matched := ring.NextMatch(func(candidate *ExecutorEntry) bool {
		return candidate.Alive() &&
			candidate.executorType == executorType &&
			candidate.Supports(function) &&
			candidate.HasCapacity()
	})

	return entry, matched
}

func (r *executorRegistry) UpdateLabels(
	instanceID protocol.InstanceID,
	announce []protocol.Label,
	withdraw []protocol.Label,
) (store.LabelCount, yaerrors.Error) {
	if slices.Contains(announce, "") || slices.Contains(withdraw, "") {
		return 0, yaerrors.FromError(
			http.StatusBadRequest,
			ErrEmptyLabel,
			logTag+" failed to update labels",
		)
	}

	r.mu.Lock()

	entry, found := r.entries[instanceID]
	if !found {
		r.mu.Unlock()

		return 0, yaerrors.FromError(
			http.StatusNotFound,
			ErrUnknownInstance,
			logTag+" failed to update labels",
		)
	}

	desired := entry.labels.Copy()

	for _, label := range announce {
		desired.Set(label)
	}

	desired.DeleteMultiple(withdraw)

	if desired.Length() > int(maxInstanceLabels) {
		//nolint:gosec // a live label set is admitted through this same cap
		held := store.LabelCount(entry.labels.Length())

		r.mu.Unlock()

		return held, yaerrors.FromError(
			http.StatusBadRequest,
			ErrLabelLimitExceeded,
			logTag+" failed to update labels",
		)
	}

	added := desired.Difference(entry.labels).Values()

	r.detachLabels(instanceID, entry.labels.Difference(desired).Values())

	entry.labels = desired

	r.attachLabels(entry, desired.Values())

	//nolint:gosec // the cap check above bounds the desired set
	active := store.LabelCount(desired.Length())
	executorType := entry.executorType
	notify := r.notify

	r.mu.Unlock()

	if notify != nil && len(added) > 0 {
		notify(RegistryChange{ExecutorType: executorType, Labels: added})
	}

	return active, nil
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

func (r *executorRegistry) LabelPoolSize(label protocol.Label) store.PoolSize {
	r.mu.RLock()
	ring, found := r.labelPools[label]
	r.mu.RUnlock()

	if !found {
		return 0
	}

	alive := store.PoolSize(0)

	for _, entry := range ring.Values() {
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
