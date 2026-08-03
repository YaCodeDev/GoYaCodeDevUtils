package redisstore

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
	"github.com/redis/go-redis/v9"
)

// CreateExecution materializes one occurrence of a job. A repeat of an
// already materialized occurrence returns the stored execution and reports
// false, so a replayed schedule pass never double-runs a job.
func (s *Store) CreateExecution(
	ctx context.Context,
	jobID protocol.JobUUID,
	scheduledAt time.Time,
	state store.ExecutionState,
	backfilled store.Backfilled,
) (execution *store.Execution, created bool, err yaerrors.Error) {
	const action = "create execution"

	scheduled := scheduledAt.UTC()
	wire := executionWire{
		JobID:         jobID,
		ScheduledAt:   scheduled,
		State:         state,
		NextAttemptAt: scheduled,
		Backfilled:    backfilled,
	}

	blob, err := yaencoding.EncodeMessagePack(wire)
	if err != nil {
		return nil, false, err.Wrap(logTag + " failed to " + action)
	}

	now := s.now()
	idHex := uuidHex(jobID)
	wake, wakes := executionWake(state, scheduled, scheduled)
	leased := executionLeased(state)

	reply, runErr := createExecutionScript.Run(
		ctx,
		s.client,
		[]string{
			s.jobKey(idHex),
			s.keys.occurrences,
			s.keys.executionCounter,
			s.keys.wake,
			s.keys.lease,
			s.stateKey(state),
			s.jobExecutionsKey(idHex),
			s.jobActiveKey(idHex),
			s.jobPendingKey(idHex),
		},
		occurrenceField(jobID, scheduled),
		blob,
		nanoString(now),
		s.keys.executionPrefix,
		boolFlag(wakes),
		microScore(wake),
		boolFlag(leased),
		microScore(time.Time{}),
		boolFlag(leased),
		boolFlag(!state.Terminal()),
	).Result()
	if runErr != nil {
		return nil, false, transportError(runErr, action)
	}

	values, isSlice := reply.([]any)
	if !isSlice || len(values) == 0 {
		return nil, false, scriptReplyError(action)
	}

	code, isCode := asInt64(values[0])
	if !isCode {
		return nil, false, scriptReplyError(action)
	}

	switch code {
	case replyRefused:
		return nil, false, yaerrors.FromError(
			http.StatusNotFound,
			store.ErrJobNotFound,
			logTag+" failed to "+action,
		)
	case replyExisting, replyCreated:
		if len(values) < pairReplyLength {
			return nil, false, scriptReplyError(action)
		}

		id, isID := asUint64(values[1])
		if !isID {
			return nil, false, scriptReplyError(action)
		}

		if code == replyExisting {
			execution, err = s.GetExecution(ctx, protocol.ExecutionID(id))
			if err != nil {
				return nil, false, err.Wrap(logTag + " failed to " + action)
			}

			return execution, false, nil
		}

		return &store.Execution{
			ID:            protocol.ExecutionID(id),
			JobID:         jobID,
			ScheduledAt:   scheduled,
			State:         state,
			NextAttemptAt: scheduled,
			Backfilled:    backfilled,
			Version:       1,
			CreatedAt:     now,
			UpdatedAt:     now,
		}, true, nil
	default:
		return nil, false, scriptReplyError(action)
	}
}

// GetExecution returns the execution with the given identifier.
func (s *Store) GetExecution(
	ctx context.Context,
	id protocol.ExecutionID,
) (execution *store.Execution, err yaerrors.Error) {
	const action = "fetch execution"

	fields, getErr := s.client.HGetAll(ctx, s.executionKey(id)).Result()
	if getErr != nil {
		return nil, transportError(getErr, action)
	}

	if len(fields) == 0 {
		return nil, yaerrors.FromError(
			http.StatusNotFound,
			store.ErrExecutionNotFound,
			logTag+" failed to "+action,
		)
	}

	return executionFromHash(id, fields)
}

// UpdateExecution applies a partial update under an optimistic version
// check, refusing a state change out of a terminal state or one the
// transition table does not allow. The version check re-runs atomically
// inside the script, so racing updates serialize and exactly one wins.
func (s *Store) UpdateExecution(
	ctx context.Context,
	id protocol.ExecutionID,
	expectedVersion store.Version,
	update store.ExecutionUpdate,
) (execution *store.Execution, err yaerrors.Error) {
	const action = "update execution"

	current, err := s.GetExecution(ctx, id)
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	if current.Version != expectedVersion {
		return nil, yaerrors.FromError(
			http.StatusConflict,
			store.ErrVersionConflict,
			logTag+" failed to "+action,
		)
	}

	next := *current

	if update.State != nil && *update.State != current.State {
		if current.State.Terminal() {
			return nil, yaerrors.FromError(
				http.StatusConflict,
				store.ErrTerminalState,
				logTag+" failed to "+action,
			)
		}

		if !store.CanTransition(current.State, *update.State) {
			return nil, yaerrors.FromError(
				http.StatusConflict,
				store.ErrIllegalTransition,
				logTag+" failed to "+action,
			)
		}

		next.State = *update.State
	}

	applyExecutionUpdate(&next, update)

	next.Version++
	next.UpdatedAt = s.now()

	return s.writeExecutionUpdate(ctx, current, &next, expectedVersion)
}

func applyExecutionUpdate(next *store.Execution, update store.ExecutionUpdate) {
	if update.FunctionAttempts != nil {
		next.FunctionAttempts = *update.FunctionAttempts
	}

	if update.CurrentAttemptID != nil {
		next.CurrentAttemptID = *update.CurrentAttemptID
	}

	if update.NextAttemptAt != nil {
		next.NextAttemptAt = update.NextAttemptAt.UTC()
	}

	if update.LeaseExpiresAt != nil {
		next.LeaseExpiresAt = update.LeaseExpiresAt.UTC()
	}

	if update.LastError != nil {
		next.LastError = *update.LastError
	}

	if update.WaitReason != nil {
		next.WaitReason = *update.WaitReason
	}
}

func (s *Store) writeExecutionUpdate(
	ctx context.Context,
	current *store.Execution,
	next *store.Execution,
	expectedVersion store.Version,
) (execution *store.Execution, err yaerrors.Error) {
	const action = "update execution"

	blob, err := yaencoding.EncodeMessagePack(executionWire{
		JobID:            next.JobID,
		ScheduledAt:      next.ScheduledAt,
		State:            next.State,
		FunctionAttempts: next.FunctionAttempts,
		CurrentAttemptID: next.CurrentAttemptID,
		NextAttemptAt:    next.NextAttemptAt,
		LeaseExpiresAt:   next.LeaseExpiresAt,
		Backfilled:       next.Backfilled,
		LastError:        next.LastError,
		WaitReason:       next.WaitReason,
	})
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	idHex := uuidHex(next.JobID)
	wake, wakes := executionWake(next.State, next.ScheduledAt, next.NextAttemptAt)
	leased := executionLeased(next.State)

	reply, runErr := updateExecutionScript.Run(
		ctx,
		s.client,
		[]string{
			s.executionKey(next.ID),
			s.stateKey(current.State),
			s.stateKey(next.State),
			s.keys.wake,
			s.keys.lease,
			s.jobActiveKey(idHex),
			s.jobPendingKey(idHex),
		},
		strconv.FormatUint(uint64(expectedVersion), decimalBase),
		blob,
		nanoString(next.UpdatedAt),
		boolFlag(wakes),
		microScore(wake),
		boolFlag(leased),
		microScore(next.LeaseExpiresAt),
		boolFlag(leased),
		boolFlag(!next.State.Terminal()),
		strconv.FormatUint(uint64(next.ID), decimalBase),
	).Result()
	if runErr != nil {
		return nil, transportError(runErr, action)
	}

	code, isCode := asInt64(reply)
	if !isCode {
		return nil, scriptReplyError(action)
	}

	switch code {
	case replyNotFound:
		return nil, yaerrors.FromError(
			http.StatusNotFound,
			store.ErrExecutionNotFound,
			logTag+" failed to "+action,
		)
	case replyConflict:
		return nil, yaerrors.FromError(
			http.StatusConflict,
			store.ErrVersionConflict,
			logTag+" failed to "+action,
		)
	case replyUpdated:
		return next, nil
	default:
		return nil, scriptReplyError(action)
	}
}

// DueExecutions returns executions whose time has come, ordered by
// schedule time then identifier, capped by a positive limit.
func (s *Store) DueExecutions(
	ctx context.Context,
	now time.Time,
	limit store.BatchLimit,
) (due []*store.Execution, err yaerrors.Error) {
	const action = "list due executions"

	members, rangeErr := s.client.ZRangeByScore(ctx, s.keys.wake, &redis.ZRangeBy{
		Min: scoreFloor,
		Max: microScore(now),
	}).Result()
	if rangeErr != nil {
		return nil, transportError(rangeErr, action)
	}

	candidates, err := s.executionsByMembers(ctx, members, action)
	if err != nil {
		return nil, err
	}

	due = make([]*store.Execution, 0, len(candidates))

	for _, candidate := range candidates {
		wake, wakes := executionWake(
			candidate.State,
			candidate.ScheduledAt,
			candidate.NextAttemptAt,
		)
		if wakes && !wake.After(now) {
			due = append(due, candidate)
		}
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
	ctx context.Context,
) (wakeAt time.Time, found bool, err yaerrors.Error) {
	const action = "find next wake time"

	scored, rangeErr := s.client.ZRangeWithScores(ctx, s.keys.wake, 0, 0).Result()
	if rangeErr != nil {
		return time.Time{}, false, transportError(rangeErr, action)
	}

	if len(scored) == 0 {
		return time.Time{}, false, nil
	}

	minScore := strconv.FormatFloat(scored[0].Score, 'f', -1, uintBitSize)

	members, tiedErr := s.client.ZRangeByScore(ctx, s.keys.wake, &redis.ZRangeBy{
		Min: minScore,
		Max: minScore,
	}).Result()
	if tiedErr != nil {
		return time.Time{}, false, transportError(tiedErr, action)
	}

	candidates, err := s.executionsByMembers(ctx, members, action)
	if err != nil {
		return time.Time{}, false, err
	}

	for _, candidate := range candidates {
		wake, wakes := executionWake(
			candidate.State,
			candidate.ScheduledAt,
			candidate.NextAttemptAt,
		)
		if !wakes {
			continue
		}

		if !found || wake.Before(wakeAt) {
			wakeAt = wake
			found = true
		}
	}

	return wakeAt, found, nil
}

// ExecutionsInStates returns every execution in any of the given states,
// in creation order.
func (s *Store) ExecutionsInStates(
	ctx context.Context,
	states ...store.ExecutionState,
) (matching []*store.Execution, err yaerrors.Error) {
	const action = "list executions by state"

	matching = make([]*store.Execution, 0)

	if len(states) == 0 {
		return matching, nil
	}

	stateKeys := make([]string, 0, len(states))
	for _, state := range states {
		stateKeys = append(stateKeys, s.stateKey(state))
	}

	members, unionErr := s.client.SUnion(ctx, stateKeys...).Result()
	if unionErr != nil {
		return nil, transportError(unionErr, action)
	}

	candidates, err := s.executionsByMembers(ctx, members, action)
	if err != nil {
		return nil, err
	}

	wanted := make(map[store.ExecutionState]struct{}, len(states))
	for _, state := range states {
		wanted[state] = struct{}{}
	}

	for _, candidate := range candidates {
		if _, matches := wanted[candidate.State]; matches {
			matching = append(matching, candidate)
		}
	}

	sortExecutionsByID(matching)

	return matching, nil
}

// ExecutionsForJob returns every execution of one job in creation order.
func (s *Store) ExecutionsForJob(
	ctx context.Context,
	jobID protocol.JobUUID,
) (matching []*store.Execution, err yaerrors.Error) {
	const action = "list job executions"

	members, membersErr := s.client.SMembers(
		ctx,
		s.jobExecutionsKey(uuidHex(jobID)),
	).Result()
	if membersErr != nil {
		return nil, transportError(membersErr, action)
	}

	matching, err = s.executionsByMembers(ctx, members, action)
	if err != nil {
		return nil, err
	}

	sortExecutionsByID(matching)

	return matching, nil
}

// HasActiveExecution reports whether any execution of one job besides the
// excluded one holds a dispatch or run lease.
func (s *Store) HasActiveExecution(
	ctx context.Context,
	jobID protocol.JobUUID,
	exclude protocol.ExecutionID,
) (active bool, err yaerrors.Error) {
	const action = "check job activity"

	members, membersErr := s.client.SMembers(
		ctx,
		s.jobActiveKey(uuidHex(jobID)),
	).Result()
	if membersErr != nil {
		return false, transportError(membersErr, action)
	}

	excluded := strconv.FormatUint(uint64(exclude), decimalBase)

	for _, member := range members {
		if member != excluded {
			return true, nil
		}
	}

	return false, nil
}

// HasPendingOccurrence reports whether any occurrence of one job has yet
// to settle.
func (s *Store) HasPendingOccurrence(
	ctx context.Context,
	jobID protocol.JobUUID,
) (pending bool, err yaerrors.Error) {
	const action = "check pending occurrences"

	count, cardErr := s.client.SCard(ctx, s.jobPendingKey(uuidHex(jobID))).Result()
	if cardErr != nil {
		return false, transportError(cardErr, action)
	}

	return count > 0, nil
}

// ExpiredLeases returns leased executions whose lease has elapsed, in
// creation order.
func (s *Store) ExpiredLeases(
	ctx context.Context,
	now time.Time,
) (expired []*store.Execution, err yaerrors.Error) {
	const action = "list expired leases"

	members, rangeErr := s.client.ZRangeByScore(ctx, s.keys.lease, &redis.ZRangeBy{
		Min: scoreFloor,
		Max: microScore(now),
	}).Result()
	if rangeErr != nil {
		return nil, transportError(rangeErr, action)
	}

	candidates, err := s.executionsByMembers(ctx, members, action)
	if err != nil {
		return nil, err
	}

	expired = make([]*store.Execution, 0, len(candidates))

	for _, candidate := range candidates {
		if executionLeased(candidate.State) && !candidate.LeaseExpiresAt.After(now) {
			expired = append(expired, candidate)
		}
	}

	sortExecutionsByID(expired)

	return expired, nil
}

func (s *Store) executionsByMembers(
	ctx context.Context,
	members []string,
	action string,
) (executions []*store.Execution, err yaerrors.Error) {
	ids := make([]protocol.ExecutionID, 0, len(members))
	keys := make([]string, 0, len(members))

	for _, member := range members {
		parsed, parseErr := strconv.ParseUint(member, decimalBase, uintBitSize)
		if parseErr != nil {
			return nil, transportError(parseErr, action)
		}

		ids = append(ids, protocol.ExecutionID(parsed))
		keys = append(keys, s.executionKey(protocol.ExecutionID(parsed)))
	}

	hashes, err := s.fetchHashes(ctx, keys, action)
	if err != nil {
		return nil, err
	}

	executions = make([]*store.Execution, 0, len(hashes))

	for index, fields := range hashes {
		if len(fields) == 0 {
			continue
		}

		execution, decodeErr := executionFromHash(ids[index], fields)
		if decodeErr != nil {
			return nil, decodeErr.Wrap(logTag + " failed to " + action)
		}

		executions = append(executions, execution)
	}

	return executions, nil
}

func sortExecutionsByID(executions []*store.Execution) {
	sort.Slice(executions, func(i, j int) bool {
		return executions[i].ID < executions[j].ID
	})
}

func executionFromHash(
	id protocol.ExecutionID,
	fields map[string]string,
) (execution *store.Execution, err yaerrors.Error) {
	const action = "decode execution record"

	wire, err := yaencoding.DecodeMessagePack[executionWire]([]byte(fields[fieldBlob]))
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	version, err := parseUintField(fields, fieldVersion)
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	createdAt, err := parseTimeField(fields, fieldCreatedAt)
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	updatedAt, err := parseTimeField(fields, fieldUpdatedAt)
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	return &store.Execution{
		ID:               id,
		JobID:            wire.JobID,
		ScheduledAt:      wire.ScheduledAt,
		State:            wire.State,
		FunctionAttempts: wire.FunctionAttempts,
		CurrentAttemptID: wire.CurrentAttemptID,
		NextAttemptAt:    wire.NextAttemptAt,
		LeaseExpiresAt:   wire.LeaseExpiresAt,
		Backfilled:       wire.Backfilled,
		LastError:        wire.LastError,
		WaitReason:       wire.WaitReason,
		Version:          store.Version(version),
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
	}, nil
}
