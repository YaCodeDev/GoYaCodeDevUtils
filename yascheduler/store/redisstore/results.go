package redisstore

import (
	"bytes"
	"context"
	"errors"
	"math"
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

// StoreResult holds one settled result for later delivery, keyed by its
// job. It reports false without an error when a storage cap refuses the
// result, so a full store never fails the execution that produced it.
// Re-storing a job's result keeps the send counters of the stored one.
func (s *Store) StoreResult(
	ctx context.Context,
	result *store.PendingResult,
) (stored bool, err yaerrors.Error) {
	const action = "store result"

	if result == nil {
		return false, yaerrors.FromError(
			http.StatusBadRequest,
			store.ErrNilResult,
			logTag+" failed to "+action,
		)
	}

	blob, err := yaencoding.EncodeMessagePack(resultWire{
		InstanceID:  result.InstanceID,
		ExecutionID: result.ExecutionID,
		Success:     result.Success,
		HasValue:    result.HasValue,
		Payload:     result.Payload,
		Cause:       result.Cause,
	})
	if err != nil {
		return false, err.Wrap(logTag + " failed to " + action)
	}

	idHex := uuidHex(result.JobUUID)
	instanceList := s.instanceResultsKey(result.InstanceID)

	for range conflictRetryLimit {
		accepted, retry, tryErr := s.tryStoreResult(ctx, blob, idHex, instanceList)
		if tryErr != nil {
			return false, tryErr
		}

		if retry {
			continue
		}

		return accepted, nil
	}

	return false, yaerrors.FromError(
		http.StatusConflict,
		ErrConcurrentUpdate,
		logTag+" failed to "+action,
	)
}

func (s *Store) tryStoreResult(
	ctx context.Context,
	blob []byte,
	idHex string,
	instanceList string,
) (stored bool, retry bool, err yaerrors.Error) {
	const action = "store result"

	expectedList, getErr := s.client.HGet(ctx, s.resultKey(idHex), fieldInstanceKey).Result()
	if getErr != nil {
		if !errors.Is(getErr, redis.Nil) {
			return false, false, transportError(getErr, action)
		}

		expectedList = ""
	}

	previousList := expectedList
	if previousList == "" {
		previousList = instanceList
	}

	now := s.now()

	reply, runErr := storeResultScript.Run(
		ctx,
		s.client,
		[]string{s.resultKey(idHex), s.keys.resultsCreated, instanceList, previousList},
		blob,
		instanceList,
		nanoString(now),
		microScore(now),
		strconv.FormatUint(uint64(s.maxResults), decimalBase),
		strconv.FormatUint(uint64(s.maxResultsPerInstance), decimalBase),
		idHex,
		expectedList,
	).Result()
	if runErr != nil {
		return false, false, transportError(runErr, action)
	}

	code, isCode := asInt64(reply)
	if !isCode {
		return false, false, scriptReplyError(action)
	}

	if code == replyRetry {
		return false, true, nil
	}

	return code == replyStored, false, nil
}

// DeleteResult drops the pending result of one job. It reports false when
// no result was held, so an acknowledgement replay is not an error.
func (s *Store) DeleteResult(
	ctx context.Context,
	jobUUID protocol.JobUUID,
) (deleted bool, err yaerrors.Error) {
	const action = "delete result"

	idHex := uuidHex(jobUUID)

	for range conflictRetryLimit {
		currentList, getErr := s.client.HGet(ctx, s.resultKey(idHex), fieldInstanceKey).Result()
		if getErr != nil {
			if errors.Is(getErr, redis.Nil) {
				return false, nil
			}

			return false, transportError(getErr, action)
		}

		reply, runErr := deleteResultScript.Run(
			ctx,
			s.client,
			[]string{s.resultKey(idHex), s.keys.resultsCreated, currentList},
			idHex,
			currentList,
		).Result()
		if runErr != nil {
			return false, transportError(runErr, action)
		}

		code, isCode := asInt64(reply)
		if !isCode {
			return false, scriptReplyError(action)
		}

		if code == replyRetry {
			continue
		}

		return code == replyDeleted, nil
	}

	return false, yaerrors.FromError(
		http.StatusConflict,
		ErrConcurrentUpdate,
		logTag+" failed to "+action,
	)
}

// ResultsForInstance returns pending results submitted by one instance in
// storage order, capped by a positive limit.
func (s *Store) ResultsForInstance(
	ctx context.Context,
	id protocol.InstanceID,
	limit store.BatchLimit,
) (results []*store.PendingResult, err yaerrors.Error) {
	const action = "list instance results"

	stop := int64(-1)
	if limit > 0 {
		stop = int64(limit) - 1
	}

	members, rangeErr := s.client.LRange(
		ctx,
		s.instanceResultsKey(id),
		0,
		stop,
	).Result()
	if rangeErr != nil {
		return nil, transportError(rangeErr, action)
	}

	results, err = s.resultsByMembers(ctx, members, action)
	if err != nil {
		return nil, err
	}

	return results, nil
}

// MarkResultSent records one delivery attempt of a pending result.
func (s *Store) MarkResultSent(
	ctx context.Context,
	jobUUID protocol.JobUUID,
	at time.Time,
) (err yaerrors.Error) {
	const action = "mark result sent"

	reply, runErr := markResultSentScript.Run(
		ctx,
		s.client,
		[]string{s.resultKey(uuidHex(jobUUID))},
		nanoString(at),
	).Result()
	if runErr != nil {
		return transportError(runErr, action)
	}

	code, isCode := asInt64(reply)
	if !isCode {
		return scriptReplyError(action)
	}

	if code == replyUntouched {
		return yaerrors.FromError(
			http.StatusNotFound,
			store.ErrResultNotFound,
			logTag+" failed to "+action,
		)
	}

	return nil
}

// ExpiredResults returns pending results stored before the given instant,
// ordered by storage time then job identifier, capped by a positive limit.
func (s *Store) ExpiredResults(
	ctx context.Context,
	before time.Time,
	limit store.BatchLimit,
) (expired []*store.PendingResult, err yaerrors.Error) {
	const action = "list expired results"

	members, rangeErr := s.client.ZRangeByScore(
		ctx,
		s.keys.resultsCreated,
		&redis.ZRangeBy{
			Min: scoreFloor,
			Max: microScore(before),
		},
	).Result()
	if rangeErr != nil {
		return nil, transportError(rangeErr, action)
	}

	candidates, err := s.resultsByMembers(ctx, members, action)
	if err != nil {
		return nil, err
	}

	expired = make([]*store.PendingResult, 0, len(candidates))

	for _, candidate := range candidates {
		if candidate.CreatedAt.Before(before) {
			expired = append(expired, candidate)
		}
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
	ctx context.Context,
) (count store.OccurrenceCount, err yaerrors.Error) {
	const action = "count results"

	total, cardErr := s.client.ZCard(ctx, s.keys.resultsCreated).Result()
	if cardErr != nil {
		return 0, transportError(cardErr, action)
	}

	if total < 0 {
		return 0, scriptReplyError(action)
	}

	return store.OccurrenceCount(total), nil
}

func (s *Store) resultsByMembers(
	ctx context.Context,
	members []string,
	action string,
) (results []*store.PendingResult, err yaerrors.Error) {
	keys := make([]string, 0, len(members))
	for _, member := range members {
		keys = append(keys, s.resultKey(member))
	}

	hashes, err := s.fetchHashes(ctx, keys, action)
	if err != nil {
		return nil, err
	}

	results = make([]*store.PendingResult, 0, len(hashes))

	for index, fields := range hashes {
		if len(fields) == 0 {
			continue
		}

		id, idErr := uuidFromHex(members[index])
		if idErr != nil {
			return nil, idErr.Wrap(logTag + " failed to " + action)
		}

		result, decodeErr := resultFromHash(id, fields)
		if decodeErr != nil {
			return nil, decodeErr.Wrap(logTag + " failed to " + action)
		}

		results = append(results, result)
	}

	return results, nil
}

func resultFromHash(
	id protocol.JobUUID,
	fields map[string]string,
) (result *store.PendingResult, err yaerrors.Error) {
	const action = "decode result record"

	wire, err := yaencoding.DecodeMessagePack[resultWire]([]byte(fields[fieldBlob]))
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	attempts, err := parseUintField(fields, fieldAttempts)
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	if attempts > math.MaxUint32 {
		return nil, scriptReplyError(action)
	}

	createdAt, err := parseTimeField(fields, fieldCreatedAt)
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	lastSentAt, err := parseTimeField(fields, fieldLastSentAt)
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	return &store.PendingResult{
		JobUUID:     id,
		InstanceID:  wire.InstanceID,
		ExecutionID: wire.ExecutionID,
		Success:     wire.Success,
		HasValue:    wire.HasValue,
		Payload:     wire.Payload,
		Cause:       wire.Cause,
		Attempts:    store.ResultAttempts(attempts),
		CreatedAt:   createdAt,
		LastSentAt:  lastSentAt,
	}, nil
}
