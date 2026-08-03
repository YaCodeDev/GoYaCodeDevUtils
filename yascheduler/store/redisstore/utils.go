package redisstore

import (
	"context"
	"encoding/hex"
	"net/http"
	"strconv"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaencoding"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
	"github.com/redis/go-redis/v9"
)

func (s *Store) jobKey(idHex string) (key string) {
	return s.keys.jobPrefix + idHex
}

func (s *Store) jobExecutionsKey(idHex string) (key string) {
	return s.keys.jobPrefix + idHex + keyPartJobExecutions
}

func (s *Store) jobActiveKey(idHex string) (key string) {
	return s.keys.jobPrefix + idHex + keyPartJobActive
}

func (s *Store) jobPendingKey(idHex string) (key string) {
	return s.keys.jobPrefix + idHex + keyPartJobPending
}

func (s *Store) executionKey(id protocol.ExecutionID) (key string) {
	return s.keys.executionPrefix + strconv.FormatUint(uint64(id), decimalBase)
}

func (s *Store) executionAttemptsKey(id protocol.ExecutionID) (key string) {
	return s.executionKey(id) + keyPartAttempts
}

func (s *Store) stateKey(state store.ExecutionState) (key string) {
	return s.keys.statePrefix + strconv.FormatUint(uint64(state), decimalBase)
}

func (s *Store) attemptKey(id protocol.AttemptID) (key string) {
	return s.keys.attemptPrefix + strconv.FormatUint(uint64(id), decimalBase)
}

func (s *Store) instanceAttemptsKey(id protocol.InstanceID) (key string) {
	return s.keys.instanceAttemptPrefix + yaencoding.ToString([]byte(id))
}

func (s *Store) resultKey(idHex string) (key string) {
	return s.keys.resultPrefix + idHex
}

func (s *Store) instanceResultsKey(id protocol.InstanceID) (key string) {
	return s.keys.instanceResultPrefix + yaencoding.ToString([]byte(id))
}

func scopedField(executorType protocol.ExecutorType, key store.JobKey) (field string) {
	return strconv.Itoa(len(executorType)) + fieldSeparator + string(executorType) + string(key)
}

func occurrenceField(jobID protocol.JobUUID, scheduledAt time.Time) (field string) {
	return uuidHex(jobID) + fieldSeparator +
		strconv.FormatInt(scheduledAt.UTC().UnixNano(), decimalBase)
}

func uuidHex(id protocol.JobUUID) (encoded string) {
	return hex.EncodeToString(id[:])
}

func uuidFromHex(encoded string) (id protocol.JobUUID, err yaerrors.Error) {
	raw, decodeErr := hex.DecodeString(encoded)
	if decodeErr != nil {
		return protocol.JobUUID{}, yaerrors.FromError(
			http.StatusInternalServerError,
			decodeErr,
			logTag+" failed to decode job identifier",
		)
	}

	if len(raw) != uuidByteLength {
		return protocol.JobUUID{}, yaerrors.FromError(
			http.StatusInternalServerError,
			ErrUnexpectedScriptReply,
			logTag+" failed to decode job identifier",
		)
	}

	copy(id[:], raw)

	return id, nil
}

func nanoString(instant time.Time) (encoded string) {
	return strconv.FormatInt(instant.UTC().UnixNano(), decimalBase)
}

func timeFromNano(encoded string) (instant time.Time, err yaerrors.Error) {
	nanos, parseErr := strconv.ParseInt(encoded, decimalBase, uintBitSize)
	if parseErr != nil {
		return time.Time{}, yaerrors.FromError(
			http.StatusInternalServerError,
			parseErr,
			logTag+" failed to decode stored instant",
		)
	}

	return time.Unix(0, nanos).UTC(), nil
}

func microScore(instant time.Time) (encoded string) {
	return strconv.FormatInt(instant.UTC().UnixMicro(), decimalBase)
}

func boolFlag(value bool) (flag string) {
	if value {
		return flagTrue
	}

	return flagFalse
}

func executionWake(
	state store.ExecutionState,
	scheduledAt time.Time,
	nextAttemptAt time.Time,
) (wake time.Time, wakes bool) {
	switch state {
	case store.StateScheduled:
		return scheduledAt, true
	case store.StateReady, store.StateRetryWait:
		return nextAttemptAt, true
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

func executionLeased(state store.ExecutionState) (leased bool) {
	return state == store.StateDispatching || state == store.StateRunning
}

func parseUintField(
	fields map[string]string,
	field string,
) (value uint64, err yaerrors.Error) {
	parsed, parseErr := strconv.ParseUint(fields[field], decimalBase, uintBitSize)
	if parseErr != nil {
		return 0, yaerrors.FromError(
			http.StatusInternalServerError,
			parseErr,
			logTag+" failed to decode stored counter",
		)
	}

	return parsed, nil
}

func parseTimeField(
	fields map[string]string,
	field string,
) (instant time.Time, err yaerrors.Error) {
	encoded, found := fields[field]
	if !found || encoded == "" {
		return time.Time{}, nil
	}

	return timeFromNano(encoded)
}

func asInt64(value any) (number int64, valid bool) {
	switch typed := value.(type) {
	case int64:
		return typed, true
	case string:
		parsed, parseErr := strconv.ParseInt(typed, decimalBase, uintBitSize)
		if parseErr != nil {
			return 0, false
		}

		return parsed, true
	default:
		return 0, false
	}
}

func asUint64(value any) (number uint64, valid bool) {
	signed, valid := asInt64(value)
	if !valid || signed < 0 {
		return 0, false
	}

	return uint64(signed), true
}

func asString(value any) (text string, valid bool) {
	typed, valid := value.(string)

	return typed, valid
}

func scriptReplyError(action string) (err yaerrors.Error) {
	return yaerrors.FromError(
		http.StatusInternalServerError,
		ErrUnexpectedScriptReply,
		logTag+" failed to "+action,
	)
}

func transportError(cause error, action string) (err yaerrors.Error) {
	return yaerrors.FromError(
		http.StatusInternalServerError,
		cause,
		logTag+" failed to "+action,
	)
}

func (s *Store) fetchHashes(
	ctx context.Context,
	keys []string,
	action string,
) (hashes []map[string]string, err yaerrors.Error) {
	hashes = make([]map[string]string, 0, len(keys))

	if len(keys) == 0 {
		return hashes, nil
	}

	pipe := s.client.Pipeline()

	commands := make([]*redis.MapStringStringCmd, 0, len(keys))
	for _, key := range keys {
		commands = append(commands, pipe.HGetAll(ctx, key))
	}

	if _, execErr := pipe.Exec(ctx); execErr != nil {
		return nil, transportError(execErr, action)
	}

	for _, command := range commands {
		hashes = append(hashes, command.Val())
	}

	return hashes, nil
}
