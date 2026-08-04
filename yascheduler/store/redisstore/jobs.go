package redisstore

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"sort"
	"strconv"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
	"github.com/redis/go-redis/v9"
)

// UpsertJob creates or replaces the job addressed by its executor type and
// key, keeping the stored identity, creation time, and skipped-occurrence
// counter.
func (s *Store) UpsertJob(
	ctx context.Context,
	job *store.Job,
) (upserted *store.Job, err yaerrors.Error) {
	const action = "upsert job"

	if job == nil {
		return nil, yaerrors.FromError(
			http.StatusBadRequest,
			store.ErrNilJob,
			logTag+" failed to "+action,
		)
	}

	blob, err := yaencoding.EncodeMessagePack(jobWire{
		Key:                 job.Key,
		ExecutorType:        job.ExecutorType,
		Function:            job.Function,
		Args:                job.Args,
		Schedule:            job.Schedule,
		Backfill:            job.Backfill,
		Retry:               job.Retry,
		Overlap:             job.Overlap,
		Pin:                 job.Pin,
		ResultMode:          job.ResultMode,
		SubmitterInstanceID: job.SubmitterInstanceID,
	})
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	for range conflictRetryLimit {
		tried, retry, tryErr := s.tryUpsertJob(ctx, job, blob)
		if tryErr != nil {
			return nil, tryErr
		}

		if retry {
			continue
		}

		return tried, nil
	}

	return nil, yaerrors.FromError(
		http.StatusConflict,
		ErrConcurrentUpdate,
		logTag+" failed to "+action,
	)
}

func (s *Store) tryUpsertJob(
	ctx context.Context,
	job *store.Job,
	blob []byte,
) (upserted *store.Job, retry bool, err yaerrors.Error) {
	const action = "upsert job"

	field := scopedField(job.ExecutorType, job.Key)

	expectedHex, getErr := s.client.HGet(ctx, s.keys.jobKeys, field).Result()
	if getErr != nil {
		if !errors.Is(getErr, redis.Nil) {
			return nil, false, transportError(getErr, action)
		}

		expectedHex = ""
	}

	now := s.now()
	idHex := uuidHex(job.ID)

	existingKey := s.jobKey(idHex)
	if expectedHex != "" {
		existingKey = s.jobKey(expectedHex)
	}

	reply, runErr := upsertJobScript.Run(
		ctx,
		s.client,
		[]string{s.keys.jobKeys, s.keys.jobsEnabled, s.jobKey(idHex), existingKey},
		field,
		blob,
		boolFlag(bool(job.Enabled)),
		nanoString(now),
		boolFlag(job.ID.IsZero()),
		idHex,
		expectedHex,
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

	result := *job
	result.UpdatedAt = now

	switch code {
	case replyRefused:
		return nil, false, yaerrors.FromError(
			http.StatusBadRequest,
			store.ErrZeroJobUUID,
			logTag+" failed to "+action,
		)
	case replyCreated:
		result.Version = 1
		result.CreatedAt = now
		result.SkippedOccurrences = 0

		return &result, false, nil
	case replyReplaced:
		upserted, err = s.replacedJob(&result, values)
		if err != nil {
			return nil, false, err
		}

		return upserted, false, nil
	case replyRetry:
		return nil, true, nil
	default:
		return nil, false, scriptReplyError(action)
	}
}

func (s *Store) replacedJob(
	result *store.Job,
	values []any,
) (upserted *store.Job, err yaerrors.Error) {
	const (
		action      = "upsert job"
		replyLength = 5
	)

	if len(values) != replyLength {
		return nil, scriptReplyError(action)
	}

	existingHex, hexValid := asString(values[1])
	version, versionValid := asUint64(values[2])
	createdEncoded, createdValid := asString(values[3])
	skippedEncoded, skippedValid := asString(values[4])

	if !hexValid || !versionValid || !createdValid || !skippedValid {
		return nil, scriptReplyError(action)
	}

	id, err := uuidFromHex(existingHex)
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	createdAt, err := timeFromNano(createdEncoded)
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	skipped, parseErr := strconv.ParseUint(skippedEncoded, decimalBase, uintBitSize)
	if parseErr != nil {
		return nil, transportError(parseErr, action)
	}

	result.ID = id
	result.Version = store.Version(version)
	result.CreatedAt = createdAt
	result.SkippedOccurrences = store.OccurrenceCount(skipped)

	return result, nil
}

// GetJob returns the job with the given identifier.
func (s *Store) GetJob(
	ctx context.Context,
	id protocol.JobUUID,
) (fetched *store.Job, err yaerrors.Error) {
	const action = "fetch job"

	fields, getErr := s.client.HGetAll(ctx, s.jobKey(uuidHex(id))).Result()
	if getErr != nil {
		return nil, transportError(getErr, action)
	}

	if len(fields) == 0 {
		return nil, yaerrors.FromError(
			http.StatusNotFound,
			store.ErrJobNotFound,
			logTag+" failed to "+action,
		)
	}

	return jobFromHash(id, fields)
}

// GetJobByKey returns the job addressed by the given executor type and
// key.
func (s *Store) GetJobByKey(
	ctx context.Context,
	executorType protocol.ExecutorType,
	key store.JobKey,
) (fetched *store.Job, err yaerrors.Error) {
	const action = "fetch job by key"

	idHex, getErr := s.client.HGet(
		ctx,
		s.keys.jobKeys,
		scopedField(executorType, key),
	).Result()
	if getErr != nil {
		if errors.Is(getErr, redis.Nil) {
			return nil, yaerrors.FromError(
				http.StatusNotFound,
				store.ErrJobNotFound,
				logTag+" failed to "+action,
			)
		}

		return nil, transportError(getErr, action)
	}

	id, err := uuidFromHex(idHex)
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	fetched, err = s.GetJob(ctx, id)
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	return fetched, nil
}

// DeleteJob removes the job with the given identifier and frees its
// executor-scoped key, so a later upsert of the key materializes a fresh
// job. It reports false when no job was stored, so a replayed delete is
// not an error. Executions and pending results of the job stay untouched:
// cleaning them is the engine's job, not the store's.
func (s *Store) DeleteJob(
	ctx context.Context,
	id protocol.JobUUID,
) (deleted bool, err yaerrors.Error) {
	const action = "delete job"

	idHex := uuidHex(id)

	reply, runErr := deleteJobScript.Run(
		ctx,
		s.client,
		[]string{s.jobKey(idHex), s.keys.jobKeys, s.keys.jobsEnabled},
		idHex,
	).Result()
	if runErr != nil {
		return false, transportError(runErr, action)
	}

	code, isCode := asInt64(reply)
	if !isCode {
		return false, scriptReplyError(action)
	}

	return code == replyDeleted, nil
}

// SetJobEnabled flips the scheduling eligibility of one job.
func (s *Store) SetJobEnabled(
	ctx context.Context,
	id protocol.JobUUID,
	enabled store.Enabled,
) (err yaerrors.Error) {
	const action = "set job enabled state"

	idHex := uuidHex(id)

	reply, runErr := setJobEnabledScript.Run(
		ctx,
		s.client,
		[]string{s.jobKey(idHex), s.keys.jobsEnabled},
		boolFlag(bool(enabled)),
		nanoString(s.now()),
		idHex,
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
			store.ErrJobNotFound,
			logTag+" failed to "+action,
		)
	}

	return nil
}

// AddSkippedOccurrences records occurrences dropped without dispatch.
func (s *Store) AddSkippedOccurrences(
	ctx context.Context,
	id protocol.JobUUID,
	count store.OccurrenceCount,
) (err yaerrors.Error) {
	const action = "record skipped occurrences"

	reply, runErr := addSkippedOccurrencesScript.Run(
		ctx,
		s.client,
		[]string{s.jobKey(uuidHex(id))},
		strconv.FormatUint(uint64(count), decimalBase),
		nanoString(s.now()),
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
			store.ErrJobNotFound,
			logTag+" failed to "+action,
		)
	}

	return nil
}

// ListEnabledJobs returns every schedulable job, ordered by identifier.
func (s *Store) ListEnabledJobs(
	ctx context.Context,
) (jobs []*store.Job, err yaerrors.Error) {
	const action = "list enabled jobs"

	idHexes, membersErr := s.client.SMembers(ctx, s.keys.jobsEnabled).Result()
	if membersErr != nil {
		return nil, transportError(membersErr, action)
	}

	keys := make([]string, 0, len(idHexes))
	for _, idHex := range idHexes {
		keys = append(keys, s.jobKey(idHex))
	}

	hashes, err := s.fetchHashes(ctx, keys, action)
	if err != nil {
		return nil, err
	}

	jobs = make([]*store.Job, 0, len(hashes))

	for index, fields := range hashes {
		if len(fields) == 0 || fields[fieldEnabled] != flagTrue {
			continue
		}

		id, idErr := uuidFromHex(idHexes[index])
		if idErr != nil {
			return nil, idErr.Wrap(logTag + " failed to " + action)
		}

		job, jobErr := jobFromHash(id, fields)
		if jobErr != nil {
			return nil, jobErr.Wrap(logTag + " failed to " + action)
		}

		jobs = append(jobs, job)
	}

	sort.Slice(jobs, func(i, j int) bool {
		return bytes.Compare(jobs[i].ID[:], jobs[j].ID[:]) < 0
	})

	return jobs, nil
}

func jobFromHash(
	id protocol.JobUUID,
	fields map[string]string,
) (job *store.Job, err yaerrors.Error) {
	const action = "decode job record"

	wire, err := yaencoding.DecodeMessagePack[jobWire]([]byte(fields[fieldBlob]))
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	version, err := parseUintField(fields, fieldVersion)
	if err != nil {
		return nil, err.Wrap(logTag + " failed to " + action)
	}

	skipped, err := parseUintField(fields, fieldSkipped)
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

	return &store.Job{
		ID:                  id,
		Key:                 wire.Key,
		ExecutorType:        wire.ExecutorType,
		Function:            wire.Function,
		Args:                wire.Args,
		Schedule:            wire.Schedule,
		Enabled:             store.Enabled(fields[fieldEnabled] == flagTrue),
		Backfill:            wire.Backfill,
		Retry:               wire.Retry,
		Overlap:             wire.Overlap,
		Pin:                 wire.Pin,
		ResultMode:          wire.ResultMode,
		SubmitterInstanceID: wire.SubmitterInstanceID,
		SkippedOccurrences:  store.OccurrenceCount(skipped),
		Version:             store.Version(version),
		CreatedAt:           createdAt,
		UpdatedAt:           updatedAt,
	}, nil
}
