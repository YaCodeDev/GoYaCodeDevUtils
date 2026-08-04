package redisstore

import (
	"context"
	"math"
	"net/http"
	"sort"
	"strconv"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

// CreateAttempt records one delivery of an execution to one instance.
func (s *Store) CreateAttempt(
	ctx context.Context,
	executionID protocol.ExecutionID,
	number store.AttemptNumber,
	instanceID protocol.InstanceID,
) (attempt *store.Attempt, err yaerrors.Error) {
	const action = "create attempt"

	blob, err := yaencoding.EncodeMessagePack(attemptWire{
		ExecutionID: executionID,
		Number:      number,
		InstanceID:  instanceID,
	})
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	now := s.now()

	minted, incrErr := s.client.Incr(ctx, s.keys.attemptCounter).Result()
	if incrErr != nil {
		return nil, transportError(incrErr, action)
	}

	if minted < 0 {
		return nil, scriptReplyError(action)
	}

	mintedID := protocol.AttemptID(minted)

	reply, runErr := createAttemptScript.Run(
		ctx,
		s.client,
		[]string{
			s.executionKey(executionID),
			s.attemptKey(mintedID),
			s.executionAttemptsKey(executionID),
			s.instanceAttemptsKey(instanceID),
		},
		blob,
		nanoString(now),
		strconv.FormatUint(uint64(mintedID), decimalBase),
		strconv.FormatUint(uint64(store.AttemptDispatched), decimalBase),
	).Result()
	if runErr != nil {
		return nil, transportError(runErr, action)
	}

	values, isSlice := reply.([]any)
	if !isSlice || len(values) == 0 {
		return nil, scriptReplyError(action)
	}

	code, isCode := asInt64(values[0])
	if !isCode {
		return nil, scriptReplyError(action)
	}

	if code == replyRefused {
		return nil, yaerrors.FromError(
			http.StatusNotFound,
			store.ErrExecutionNotFound,
			logTag+" failed to "+action,
		)
	}

	if code != replyCreatedAttempt || len(values) < pairReplyLength {
		return nil, scriptReplyError(action)
	}

	id, isID := asUint64(values[1])
	if !isID {
		return nil, scriptReplyError(action)
	}

	return &store.Attempt{
		ID:          protocol.AttemptID(id),
		ExecutionID: executionID,
		Number:      number,
		InstanceID:  instanceID,
		State:       store.AttemptDispatched,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// GetAttempt returns the attempt with the given identifier.
func (s *Store) GetAttempt(
	ctx context.Context,
	id protocol.AttemptID,
) (attempt *store.Attempt, err yaerrors.Error) {
	const action = "fetch attempt"

	fields, getErr := s.client.HGetAll(ctx, s.attemptKey(id)).Result()
	if getErr != nil {
		return nil, transportError(getErr, action)
	}

	if len(fields) == 0 {
		return nil, yaerrors.FromError(
			http.StatusNotFound,
			store.ErrAttemptNotFound,
			logTag+" failed to "+action,
		)
	}

	return attemptFromHash(id, fields)
}

// UpdateAttemptState moves an attempt to a new state, optionally only from
// one of the given states. It reports false when the guard did not match.
func (s *Store) UpdateAttemptState(
	ctx context.Context,
	id protocol.AttemptID,
	from []store.AttemptState,
	to store.AttemptState,
	errorText store.ErrorText,
) (updated bool, err yaerrors.Error) {
	const action = "update attempt state"

	args := make([]any, 0, len(from)+updateAttemptFixedArgs)
	args = append(
		args,
		strconv.FormatUint(uint64(to), decimalBase),
		string(errorText),
		boolFlag(errorText != ""),
		nanoString(s.now()),
		strconv.Itoa(len(from)),
	)

	for _, state := range from {
		args = append(args, strconv.FormatUint(uint64(state), decimalBase))
	}

	reply, runErr := updateAttemptStateScript.Run(
		ctx,
		s.client,
		[]string{s.attemptKey(id)},
		args...,
	).Result()
	if runErr != nil {
		return false, transportError(runErr, action)
	}

	code, isCode := asInt64(reply)
	if !isCode {
		return false, scriptReplyError(action)
	}

	switch code {
	case replyNotFound:
		return false, yaerrors.FromError(
			http.StatusNotFound,
			store.ErrAttemptNotFound,
			logTag+" failed to "+action,
		)
	case replyNoMatch:
		return false, nil
	case replyUpdated:
		return true, nil
	default:
		return false, scriptReplyError(action)
	}
}

// AttemptsForExecution returns every attempt of one execution in creation
// order.
func (s *Store) AttemptsForExecution(
	ctx context.Context,
	executionID protocol.ExecutionID,
) (attempts []*store.Attempt, err yaerrors.Error) {
	const action = "list execution attempts"

	members, membersErr := s.client.SMembers(
		ctx,
		s.executionAttemptsKey(executionID),
	).Result()
	if membersErr != nil {
		return nil, transportError(membersErr, action)
	}

	attempts, err = s.attemptsByMembers(ctx, members, action)
	if err != nil {
		return nil, err
	}

	return attempts, nil
}

// AttemptsOnInstance returns attempts delivered to one instance, in
// creation order, optionally filtered to the given states.
func (s *Store) AttemptsOnInstance(
	ctx context.Context,
	instanceID protocol.InstanceID,
	states ...store.AttemptState,
) (attempts []*store.Attempt, err yaerrors.Error) {
	const action = "list instance attempts"

	members, membersErr := s.client.SMembers(
		ctx,
		s.instanceAttemptsKey(instanceID),
	).Result()
	if membersErr != nil {
		return nil, transportError(membersErr, action)
	}

	candidates, err := s.attemptsByMembers(ctx, members, action)
	if err != nil {
		return nil, err
	}

	if len(states) == 0 {
		return candidates, nil
	}

	wanted := make(map[store.AttemptState]struct{}, len(states))
	for _, state := range states {
		wanted[state] = struct{}{}
	}

	attempts = make([]*store.Attempt, 0, len(candidates))

	for _, candidate := range candidates {
		if _, matches := wanted[candidate.State]; matches {
			attempts = append(attempts, candidate)
		}
	}

	return attempts, nil
}

func (s *Store) attemptsByMembers(
	ctx context.Context,
	members []string,
	action string,
) (attempts []*store.Attempt, err yaerrors.Error) {
	ids := make([]protocol.AttemptID, 0, len(members))
	keys := make([]string, 0, len(members))

	for _, member := range members {
		parsed, parseErr := strconv.ParseUint(member, decimalBase, uintBitSize)
		if parseErr != nil {
			return nil, transportError(parseErr, action)
		}

		ids = append(ids, protocol.AttemptID(parsed))
		keys = append(keys, s.attemptKey(protocol.AttemptID(parsed)))
	}

	hashes, err := s.fetchHashes(ctx, keys, action)
	if err != nil {
		return nil, err
	}

	attempts = make([]*store.Attempt, 0, len(hashes))

	for index, fields := range hashes {
		if len(fields) == 0 {
			continue
		}

		attempt, decodeErr := attemptFromHash(ids[index], fields)
		if decodeErr != nil {
			return nil, decodeErr.Wrap(logTag + " failed to " + action)
		}

		attempts = append(attempts, attempt)
	}

	sort.Slice(attempts, func(i, j int) bool {
		return attempts[i].ID < attempts[j].ID
	})

	return attempts, nil
}

func attemptFromHash(
	id protocol.AttemptID,
	fields map[string]string,
) (attempt *store.Attempt, err yaerrors.Error) {
	const action = "decode attempt record"

	wire, err := yaencoding.DecodeMessagePack[attemptWire]([]byte(fields[fieldBlob]))
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	state, err := parseUintField(fields, fieldState)
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	if state > math.MaxUint8 {
		return nil, scriptReplyError(action)
	}

	createdAt, err := parseTimeField(fields, fieldCreatedAt)
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	updatedAt, err := parseTimeField(fields, fieldUpdatedAt)
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	return &store.Attempt{
		ID:          id,
		ExecutionID: wire.ExecutionID,
		Number:      wire.Number,
		InstanceID:  wire.InstanceID,
		State:       store.AttemptState(state),
		Error:       store.ErrorText(fields[fieldError]),
		CreatedAt:   createdAt,
		UpdatedAt:   updatedAt,
	}, nil
}
