package redisstore

import "github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"

const (
	// DefaultKeyPrefix namespaces every redis key the store touches when
	// the config names no prefix of its own.
	DefaultKeyPrefix KeyPrefix = "yascheduler"

	// DefaultMaxResults caps how many pending results the store holds in
	// total, matching the memory store's default budget.
	DefaultMaxResults store.OccurrenceCount = 1024

	// DefaultMaxResultsPerInstance caps how many pending results the store
	// holds for one submitting instance, so a single disconnected
	// submitter cannot consume the whole budget.
	DefaultMaxResultsPerInstance store.OccurrenceCount = 256
)

const logTag = "[SCHEDULERREDISSTORE]"

const (
	keyPartJobKeys          = ":jobs:keys"
	keyPartJobsEnabled      = ":jobs:enabled"
	keyPartJob              = ":job:"
	keyPartJobExecutions    = ":executions"
	keyPartJobActive        = ":active"
	keyPartJobPending       = ":pending"
	keyPartExecutionCounter = ":executions:counter"
	keyPartOccurrences      = ":executions:occurrences"
	keyPartWake             = ":executions:wake"
	keyPartLease            = ":executions:lease"
	keyPartState            = ":executions:state:"
	keyPartExecution        = ":execution:"
	keyPartAttempts         = ":attempts"
	keyPartAttemptCounter   = ":attempts:counter"
	keyPartAttempt          = ":attempt:"
	keyPartInstanceAttempts = ":attempts:instance:"
	keyPartResult           = ":result:"
	keyPartResultsCreated   = ":results:created"
	keyPartInstanceResults  = ":results:instance:"
)

const (
	fieldBlob        = "blob"
	fieldVersion     = "version"
	fieldSkipped     = "skipped"
	fieldEnabled     = "enabled"
	fieldKeyField    = "keyfield"
	fieldCreatedAt   = "created_at"
	fieldUpdatedAt   = "updated_at"
	fieldState       = "state"
	fieldError       = "error"
	fieldInstanceKey = "instkey"
	fieldAttempts    = "attempts"
	fieldLastSentAt  = "last_sent_at"
)

const (
	flagTrue  = "1"
	flagFalse = "0"

	fieldSeparator = ":"

	scoreFloor = "-inf"
)

const (
	replyRefused  int64 = 0
	replyReplaced int64 = 1
	replyCreated  int64 = 2

	replyNotFound  int64 = 0
	replyConflict  int64 = 1
	replyUpdated   int64 = 2
	replyNoMatch   int64 = 1
	replyExisting  int64 = 1
	replyStored    int64 = 1
	replyDeleted   int64 = 1
	replyUntouched int64 = 0
)

const (
	replyCreatedAttempt int64 = 1

	updateAttemptFixedArgs = 5
	pairReplyLength        = 2
)

const (
	decimalBase    = 10
	uintBitSize    = 64
	uuidByteLength = 16
)
