package memstore

import (
	"bytes"
	"context"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

var _ store.Store = (*Store)(nil)

// Config bounds the pending-result storage of a memory store. A zero field
// applies its package default.
type Config struct {
	// MaxResults caps how many pending results the store holds in total.
	MaxResults store.OccurrenceCount

	// MaxResultsPerInstance caps how many pending results the store holds
	// for one submitting instance.
	MaxResultsPerInstance store.OccurrenceCount
}

type occurrenceKey struct {
	jobID       protocol.JobUUID
	scheduledAt store.UnixNano
}

// Store is an in-memory store.Store. Every read returns a copy, so a
// caller can never mutate stored state through a returned record.
type Store struct {
	mu sync.RWMutex

	jobs    map[protocol.JobUUID]*store.Job
	jobKeys map[store.JobKey]protocol.JobUUID

	executions      map[protocol.ExecutionID]*store.Execution
	occurrences     map[occurrenceKey]protocol.ExecutionID
	nextExecutionID protocol.ExecutionID

	attempts       map[protocol.AttemptID]*store.Attempt
	attemptsByExec map[protocol.ExecutionID][]protocol.AttemptID
	attemptsByHost map[protocol.InstanceID][]protocol.AttemptID
	nextAttemptID  protocol.AttemptID

	results           map[protocol.JobUUID]*store.PendingResult
	resultsByInstance map[protocol.InstanceID][]protocol.JobUUID

	maxResults            store.OccurrenceCount
	maxResultsPerInstance store.OccurrenceCount

	clock             func() time.Time
	executionsOrdered []protocol.ExecutionID
}

// NewStore builds an empty memory store bounded by the given config.
func NewStore(config Config) (created *Store) {
	maxResults := config.MaxResults
	if maxResults == 0 {
		maxResults = DefaultMaxResults
	}

	maxResultsPerInstance := config.MaxResultsPerInstance
	if maxResultsPerInstance == 0 {
		maxResultsPerInstance = DefaultMaxResultsPerInstance
	}

	return &Store{
		jobs:                  make(map[protocol.JobUUID]*store.Job),
		jobKeys:               make(map[store.JobKey]protocol.JobUUID),
		executions:            make(map[protocol.ExecutionID]*store.Execution),
		occurrences:           make(map[occurrenceKey]protocol.ExecutionID),
		attempts:              make(map[protocol.AttemptID]*store.Attempt),
		attemptsByExec:        make(map[protocol.ExecutionID][]protocol.AttemptID),
		attemptsByHost:        make(map[protocol.InstanceID][]protocol.AttemptID),
		results:               make(map[protocol.JobUUID]*store.PendingResult),
		resultsByInstance:     make(map[protocol.InstanceID][]protocol.JobUUID),
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

// UpsertJob creates or replaces the job addressed by its key, keeping the
// stored identity, creation time, and skipped-occurrence counter.
func (s *Store) UpsertJob(
	_ context.Context,
	job *store.Job,
) (*store.Job, yaerrors.Error) {
	if job == nil {
		return nil, yaerrors.FromError(
			http.StatusBadRequest,
			store.ErrNilJob,
			logTag+" failed to upsert job",
		)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	now := s.clock()

	if existingID, found := s.jobKeys[job.Key]; found {
		existing := s.jobs[existingID]

		updated := *job
		updated.ID = existing.ID
		updated.CreatedAt = existing.CreatedAt
		updated.SkippedOccurrences = existing.SkippedOccurrences
		updated.Version = existing.Version + 1
		updated.UpdatedAt = now

		s.jobs[existing.ID] = &updated

		result := updated

		return &result, nil
	}

	if job.ID.IsZero() {
		return nil, yaerrors.FromError(
			http.StatusBadRequest,
			store.ErrZeroJobUUID,
			logTag+" failed to upsert job",
		)
	}

	created := *job
	created.Version = 1
	created.CreatedAt = now
	created.UpdatedAt = now

	s.jobs[created.ID] = &created
	s.jobKeys[created.Key] = created.ID

	result := created

	return &result, nil
}

// GetJob returns the job with the given identifier.
func (s *Store) GetJob(
	_ context.Context,
	id protocol.JobUUID,
) (*store.Job, yaerrors.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	job, found := s.jobs[id]
	if !found {
		return nil, yaerrors.FromError(
			http.StatusNotFound,
			store.ErrJobNotFound,
			logTag+" failed to fetch job",
		)
	}

	copied := *job

	return &copied, nil
}

// GetJobByKey returns the job addressed by the given key.
func (s *Store) GetJobByKey(
	_ context.Context,
	key store.JobKey,
) (*store.Job, yaerrors.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	id, found := s.jobKeys[key]
	if !found {
		return nil, yaerrors.FromError(
			http.StatusNotFound,
			store.ErrJobNotFound,
			logTag+" failed to fetch job by key",
		)
	}

	copied := *s.jobs[id]

	return &copied, nil
}

// SetJobEnabled flips the scheduling eligibility of one job.
func (s *Store) SetJobEnabled(
	_ context.Context,
	id protocol.JobUUID,
	enabled store.Enabled,
) yaerrors.Error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, found := s.jobs[id]
	if !found {
		return yaerrors.FromError(
			http.StatusNotFound,
			store.ErrJobNotFound,
			logTag+" failed to set job enabled state",
		)
	}

	job.Enabled = enabled
	job.Version++
	job.UpdatedAt = s.clock()

	return nil
}

// AddSkippedOccurrences records occurrences dropped without dispatch.
func (s *Store) AddSkippedOccurrences(
	_ context.Context,
	id protocol.JobUUID,
	count store.OccurrenceCount,
) yaerrors.Error {
	s.mu.Lock()
	defer s.mu.Unlock()

	job, found := s.jobs[id]
	if !found {
		return yaerrors.FromError(
			http.StatusNotFound,
			store.ErrJobNotFound,
			logTag+" failed to record skipped occurrences",
		)
	}

	job.SkippedOccurrences += count
	job.UpdatedAt = s.clock()

	return nil
}

// ListEnabledJobs returns every schedulable job, ordered by identifier.
func (s *Store) ListEnabledJobs(
	_ context.Context,
) ([]*store.Job, yaerrors.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	jobs := make([]*store.Job, 0, len(s.jobs))

	for _, job := range s.jobs {
		if job.Enabled {
			copied := *job
			jobs = append(jobs, &copied)
		}
	}

	sort.Slice(jobs, func(i, j int) bool {
		return bytes.Compare(jobs[i].ID[:], jobs[j].ID[:]) < 0
	})

	return jobs, nil
}

// CreateExecution materializes one occurrence of a job. A repeat of an
// already materialized occurrence returns the stored execution and reports
// false, so a replayed schedule pass never double-runs a job.
func (s *Store) CreateExecution(
	_ context.Context,
	jobID protocol.JobUUID,
	scheduledAt time.Time,
	state store.ExecutionState,
	backfilled store.Backfilled,
) (*store.Execution, bool, yaerrors.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, found := s.jobs[jobID]; !found {
		return nil, false, yaerrors.FromError(
			http.StatusNotFound,
			store.ErrJobNotFound,
			logTag+" failed to create execution",
		)
	}

	key := occurrenceKey{
		jobID:       jobID,
		scheduledAt: store.UnixNano(scheduledAt.UTC().UnixNano()),
	}

	if existingID, found := s.occurrences[key]; found {
		copied := *s.executions[existingID]

		return &copied, false, nil
	}

	s.nextExecutionID++

	now := s.clock()
	execution := &store.Execution{
		ID:            s.nextExecutionID,
		JobID:         jobID,
		ScheduledAt:   scheduledAt.UTC(),
		State:         state,
		NextAttemptAt: scheduledAt.UTC(),
		Backfilled:    backfilled,
		Version:       1,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	s.executions[execution.ID] = execution
	s.occurrences[key] = execution.ID
	s.executionsOrdered = append(s.executionsOrdered, execution.ID)

	copied := *execution

	return &copied, true, nil
}

// GetExecution returns the execution with the given identifier.
func (s *Store) GetExecution(
	_ context.Context,
	id protocol.ExecutionID,
) (*store.Execution, yaerrors.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	execution, found := s.executions[id]
	if !found {
		return nil, yaerrors.FromError(
			http.StatusNotFound,
			store.ErrExecutionNotFound,
			logTag+" failed to fetch execution",
		)
	}

	copied := *execution

	return &copied, nil
}

// UpdateExecution applies a partial update under an optimistic version
// check, refusing a state change out of a terminal state or one the
// transition table does not allow.
func (s *Store) UpdateExecution(
	_ context.Context,
	id protocol.ExecutionID,
	expectedVersion store.Version,
	update store.ExecutionUpdate,
) (*store.Execution, yaerrors.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	execution, found := s.executions[id]
	if !found {
		return nil, yaerrors.FromError(
			http.StatusNotFound,
			store.ErrExecutionNotFound,
			logTag+" failed to update execution",
		)
	}

	if execution.Version != expectedVersion {
		return nil, yaerrors.FromError(
			http.StatusConflict,
			store.ErrVersionConflict,
			logTag+" failed to update execution",
		)
	}

	if update.State != nil && *update.State != execution.State {
		if execution.State.Terminal() {
			return nil, yaerrors.FromError(
				http.StatusConflict,
				store.ErrTerminalState,
				logTag+" failed to update execution",
			)
		}

		if !store.CanTransition(execution.State, *update.State) {
			return nil, yaerrors.FromError(
				http.StatusConflict,
				store.ErrIllegalTransition,
				logTag+" failed to update execution",
			)
		}

		execution.State = *update.State
	}

	if update.FunctionAttempts != nil {
		execution.FunctionAttempts = *update.FunctionAttempts
	}

	if update.CurrentAttemptID != nil {
		execution.CurrentAttemptID = *update.CurrentAttemptID
	}

	if update.NextAttemptAt != nil {
		execution.NextAttemptAt = update.NextAttemptAt.UTC()
	}

	if update.LeaseExpiresAt != nil {
		execution.LeaseExpiresAt = update.LeaseExpiresAt.UTC()
	}

	if update.LastError != nil {
		execution.LastError = *update.LastError
	}

	if update.WaitReason != nil {
		execution.WaitReason = *update.WaitReason
	}

	execution.Version++
	execution.UpdatedAt = s.clock()

	copied := *execution

	return &copied, nil
}

// DueExecutions returns executions whose time has come, ordered by
// schedule time then identifier, capped by a positive limit.
func (s *Store) DueExecutions(
	_ context.Context,
	now time.Time,
	limit store.BatchLimit,
) ([]*store.Execution, yaerrors.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	due := make([]*store.Execution, 0)

	for _, id := range s.executionsOrdered {
		execution := s.executions[id]

		if !executionDue(execution, now) {
			continue
		}

		copied := *execution
		due = append(due, &copied)
	}

	sort.Slice(due, func(i, j int) bool {
		if due[i].ScheduledAt.Equal(due[j].ScheduledAt) {
			return due[i].ID < due[j].ID
		}

		return due[i].ScheduledAt.Before(due[j].ScheduledAt)
	})

	if limit > 0 && store.BatchLimit(len(due)) > limit {
		due = due[:limit]
	}

	return due, nil
}

// NextWakeAt returns the earliest instant at which any execution becomes
// due, and whether any execution is waiting for one.
func (s *Store) NextWakeAt(
	_ context.Context,
) (time.Time, bool, yaerrors.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var (
		earliest time.Time
		found    bool
	)

	for _, execution := range s.executions {
		wake, wakes := executionWakeTime(execution)
		if !wakes {
			continue
		}

		if !found || wake.Before(earliest) {
			earliest = wake
			found = true
		}
	}

	return earliest, found, nil
}

// ExecutionsInStates returns every execution in any of the given states,
// in creation order.
func (s *Store) ExecutionsInStates(
	_ context.Context,
	states ...store.ExecutionState,
) ([]*store.Execution, yaerrors.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wanted := make(map[store.ExecutionState]struct{}, len(states))
	for _, state := range states {
		wanted[state] = struct{}{}
	}

	matching := make([]*store.Execution, 0)

	for _, id := range s.executionsOrdered {
		execution := s.executions[id]

		if _, matches := wanted[execution.State]; matches {
			copied := *execution
			matching = append(matching, &copied)
		}
	}

	return matching, nil
}

// ExecutionsForJob returns every execution of one job in creation order.
func (s *Store) ExecutionsForJob(
	_ context.Context,
	jobID protocol.JobUUID,
) ([]*store.Execution, yaerrors.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	matching := make([]*store.Execution, 0)

	for _, id := range s.executionsOrdered {
		execution := s.executions[id]

		if execution.JobID == jobID {
			copied := *execution
			matching = append(matching, &copied)
		}
	}

	return matching, nil
}

// HasActiveExecution reports whether any execution of one job besides the
// excluded one holds a dispatch or run lease.
func (s *Store) HasActiveExecution(
	_ context.Context,
	jobID protocol.JobUUID,
	exclude protocol.ExecutionID,
) (bool, yaerrors.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, execution := range s.executions {
		if execution.JobID != jobID || execution.ID == exclude {
			continue
		}

		if execution.State == store.StateDispatching ||
			execution.State == store.StateRunning {
			return true, nil
		}
	}

	return false, nil
}

// HasPendingOccurrence reports whether any occurrence of one job has yet
// to settle.
func (s *Store) HasPendingOccurrence(
	_ context.Context,
	jobID protocol.JobUUID,
) (bool, yaerrors.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, execution := range s.executions {
		if execution.JobID != jobID {
			continue
		}

		if !execution.State.Terminal() {
			return true, nil
		}
	}

	return false, nil
}

// ExpiredLeases returns leased executions whose lease has elapsed.
func (s *Store) ExpiredLeases(
	_ context.Context,
	now time.Time,
) ([]*store.Execution, yaerrors.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	expired := make([]*store.Execution, 0)

	for _, id := range s.executionsOrdered {
		execution := s.executions[id]

		inLease := execution.State == store.StateDispatching ||
			execution.State == store.StateRunning
		if inLease && !execution.LeaseExpiresAt.After(now) {
			copied := *execution
			expired = append(expired, &copied)
		}
	}

	return expired, nil
}

// CreateAttempt records one delivery of an execution to one instance.
func (s *Store) CreateAttempt(
	_ context.Context,
	executionID protocol.ExecutionID,
	number store.AttemptNumber,
	instanceID protocol.InstanceID,
) (*store.Attempt, yaerrors.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, found := s.executions[executionID]; !found {
		return nil, yaerrors.FromError(
			http.StatusNotFound,
			store.ErrExecutionNotFound,
			logTag+" failed to create attempt",
		)
	}

	s.nextAttemptID++

	now := s.clock()
	attempt := &store.Attempt{
		ID:          s.nextAttemptID,
		ExecutionID: executionID,
		Number:      number,
		InstanceID:  instanceID,
		State:       store.AttemptDispatched,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	s.attempts[attempt.ID] = attempt
	s.attemptsByExec[executionID] = append(s.attemptsByExec[executionID], attempt.ID)
	s.attemptsByHost[instanceID] = append(s.attemptsByHost[instanceID], attempt.ID)

	copied := *attempt

	return &copied, nil
}

// GetAttempt returns the attempt with the given identifier.
func (s *Store) GetAttempt(
	_ context.Context,
	id protocol.AttemptID,
) (*store.Attempt, yaerrors.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	attempt, found := s.attempts[id]
	if !found {
		return nil, yaerrors.FromError(
			http.StatusNotFound,
			store.ErrAttemptNotFound,
			logTag+" failed to fetch attempt",
		)
	}

	copied := *attempt

	return &copied, nil
}

// UpdateAttemptState moves an attempt to a new state, optionally only from
// one of the given states. It reports false when the guard did not match.
func (s *Store) UpdateAttemptState(
	_ context.Context,
	id protocol.AttemptID,
	from []store.AttemptState,
	to store.AttemptState,
	errorText store.ErrorText,
) (bool, yaerrors.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	attempt, found := s.attempts[id]
	if !found {
		return false, yaerrors.FromError(
			http.StatusNotFound,
			store.ErrAttemptNotFound,
			logTag+" failed to update attempt state",
		)
	}

	if len(from) > 0 {
		matched := false

		for _, state := range from {
			if attempt.State == state {
				matched = true

				break
			}
		}

		if !matched {
			return false, nil
		}
	}

	attempt.State = to

	if errorText != "" {
		attempt.Error = errorText
	}

	attempt.UpdatedAt = s.clock()

	return true, nil
}

// AttemptsForExecution returns every attempt of one execution in creation
// order.
func (s *Store) AttemptsForExecution(
	_ context.Context,
	executionID protocol.ExecutionID,
) ([]*store.Attempt, yaerrors.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.attemptsByExec[executionID]
	attempts := make([]*store.Attempt, 0, len(ids))

	for _, id := range ids {
		copied := *s.attempts[id]
		attempts = append(attempts, &copied)
	}

	return attempts, nil
}

// AttemptsOnInstance returns attempts delivered to one instance, in
// creation order, optionally filtered to the given states.
func (s *Store) AttemptsOnInstance(
	_ context.Context,
	instanceID protocol.InstanceID,
	states ...store.AttemptState,
) ([]*store.Attempt, yaerrors.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	wanted := make(map[store.AttemptState]struct{}, len(states))
	for _, state := range states {
		wanted[state] = struct{}{}
	}

	ids := s.attemptsByHost[instanceID]
	attempts := make([]*store.Attempt, 0, len(ids))

	for _, id := range ids {
		attempt := s.attempts[id]

		if len(states) > 0 {
			if _, matches := wanted[attempt.State]; !matches {
				continue
			}
		}

		copied := *attempt
		attempts = append(attempts, &copied)
	}

	return attempts, nil
}

// StoreResult holds one settled result for later delivery, keyed by its
// job. It reports false without an error when a storage cap refuses the
// result, so a full store never fails the execution that produced it.
// Re-storing a job's result keeps the send counters of the stored one.
func (s *Store) StoreResult(
	_ context.Context,
	result *store.PendingResult,
) (bool, yaerrors.Error) {
	if result == nil {
		return false, yaerrors.FromError(
			http.StatusBadRequest,
			store.ErrNilResult,
			logTag+" failed to store result",
		)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, found := s.results[result.JobUUID]

	if !found && store.OccurrenceCount(len(s.results)) >= s.maxResults {
		return false, nil
	}

	moved := found && existing.InstanceID != result.InstanceID

	if !found || moved {
		held := store.OccurrenceCount(len(s.resultsByInstance[result.InstanceID]))
		if held >= s.maxResultsPerInstance {
			return false, nil
		}
	}

	stored := *result

	if found {
		stored.Attempts = existing.Attempts
		stored.CreatedAt = existing.CreatedAt
		stored.LastSentAt = existing.LastSentAt
	} else {
		stored.Attempts = 0
		stored.CreatedAt = s.clock()
		stored.LastSentAt = time.Time{}
	}

	if moved {
		s.detachResult(existing.InstanceID, result.JobUUID)
	}

	if !found || moved {
		s.resultsByInstance[result.InstanceID] = append(
			s.resultsByInstance[result.InstanceID],
			result.JobUUID,
		)
	}

	s.results[result.JobUUID] = &stored

	return true, nil
}

// DeleteResult drops the pending result of one job. It reports false when
// no result was held, so an acknowledgement replay is not an error.
func (s *Store) DeleteResult(
	_ context.Context,
	jobUUID protocol.JobUUID,
) (bool, yaerrors.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing, found := s.results[jobUUID]
	if !found {
		return false, nil
	}

	delete(s.results, jobUUID)
	s.detachResult(existing.InstanceID, jobUUID)

	return true, nil
}

// ResultsForInstance returns pending results submitted by one instance in
// storage order, capped by a positive limit.
func (s *Store) ResultsForInstance(
	_ context.Context,
	id protocol.InstanceID,
	limit store.BatchLimit,
) ([]*store.PendingResult, yaerrors.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := s.resultsByInstance[id]
	results := make([]*store.PendingResult, 0, len(ids))

	for _, jobUUID := range ids {
		if limit > 0 && store.BatchLimit(len(results)) >= limit {
			break
		}

		copied := *s.results[jobUUID]
		results = append(results, &copied)
	}

	return results, nil
}

// MarkResultSent records one delivery attempt of a pending result.
func (s *Store) MarkResultSent(
	_ context.Context,
	jobUUID protocol.JobUUID,
	at time.Time,
) yaerrors.Error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result, found := s.results[jobUUID]
	if !found {
		return yaerrors.FromError(
			http.StatusNotFound,
			store.ErrResultNotFound,
			logTag+" failed to mark result sent",
		)
	}

	result.Attempts++
	result.LastSentAt = at.UTC()

	return nil
}

// ExpiredResults returns pending results stored before the given instant,
// ordered by storage time then job identifier, capped by a positive limit.
func (s *Store) ExpiredResults(
	_ context.Context,
	before time.Time,
	limit store.BatchLimit,
) ([]*store.PendingResult, yaerrors.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	expired := make([]*store.PendingResult, 0)

	for _, result := range s.results {
		if !result.CreatedAt.Before(before) {
			continue
		}

		copied := *result
		expired = append(expired, &copied)
	}

	sort.Slice(expired, func(i, j int) bool {
		if expired[i].CreatedAt.Equal(expired[j].CreatedAt) {
			return bytes.Compare(
				expired[i].JobUUID[:],
				expired[j].JobUUID[:],
			) < 0
		}

		return expired[i].CreatedAt.Before(expired[j].CreatedAt)
	})

	if limit > 0 && store.BatchLimit(len(expired)) > limit {
		expired = expired[:limit]
	}

	return expired, nil
}

// CountResults returns how many pending results the store holds.
func (s *Store) CountResults(
	_ context.Context,
) (store.OccurrenceCount, yaerrors.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return store.OccurrenceCount(len(s.results)), nil
}

func (s *Store) detachResult(
	instanceID protocol.InstanceID,
	jobUUID protocol.JobUUID,
) {
	ids := s.resultsByInstance[instanceID]

	for index, id := range ids {
		if id != jobUUID {
			continue
		}

		s.resultsByInstance[instanceID] = append(ids[:index:index], ids[index+1:]...)

		break
	}

	if len(s.resultsByInstance[instanceID]) == 0 {
		delete(s.resultsByInstance, instanceID)
	}
}

func executionDue(execution *store.Execution, now time.Time) (due bool) {
	switch execution.State {
	case store.StateScheduled:
		return !execution.ScheduledAt.After(now)
	case store.StateReady, store.StateRetryWait:
		return !execution.NextAttemptAt.After(now)
	case store.StateWaitingExecutor,
		store.StateWaitingCompatible,
		store.StateWaitingLabel,
		store.StateDispatching,
		store.StateRunning,
		store.StateSucceeded,
		store.StateFailed,
		store.StateCancelled,
		store.StateSkipped:
		return false
	default:
		return false
	}
}

func executionWakeTime(execution *store.Execution) (wake time.Time, wakes bool) {
	switch execution.State {
	case store.StateScheduled:
		return execution.ScheduledAt, true
	case store.StateReady, store.StateRetryWait:
		return execution.NextAttemptAt, true
	case store.StateWaitingExecutor,
		store.StateWaitingCompatible,
		store.StateWaitingLabel,
		store.StateDispatching,
		store.StateRunning,
		store.StateSucceeded,
		store.StateFailed,
		store.StateCancelled,
		store.StateSkipped:
		return time.Time{}, false
	default:
		return time.Time{}, false
	}
}
