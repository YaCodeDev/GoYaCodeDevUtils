package redisstore

import (
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

// KeyPrefix namespaces every redis key one store instance touches, so two
// schedulers share one redis database without seeing each other's records.
type KeyPrefix string

type jobWire struct {
	Key                 store.JobKey
	ExecutorType        protocol.ExecutorType
	Function            protocol.FunctionSpec
	Args                store.Payload
	Schedule            protocol.ScheduleSpec
	Backfill            protocol.BackfillSpec
	Retry               protocol.RetrySpec
	Overlap             protocol.OverlapPolicy
	Pin                 protocol.PinSpec
	ResultMode          protocol.ResultMode
	SubmitterInstanceID protocol.InstanceID
}

type executionWire struct {
	JobID            protocol.JobUUID
	ScheduledAt      time.Time
	State            store.ExecutionState
	FunctionAttempts store.FunctionAttempts
	CurrentAttemptID protocol.AttemptID
	NextAttemptAt    time.Time
	LeaseExpiresAt   time.Time
	Backfilled       store.Backfilled
	LastError        store.ErrorText
	WaitReason       store.WaitReason
}

type attemptWire struct {
	ExecutionID protocol.ExecutionID
	Number      store.AttemptNumber
	InstanceID  protocol.InstanceID
}

type resultWire struct {
	InstanceID  protocol.InstanceID
	ExecutionID protocol.ExecutionID
	Success     store.Delivered
	HasValue    store.HasValue
	Payload     store.Payload
	Cause       *protocol.WireError
}

type keySet struct {
	jobKeys          string
	jobsEnabled      string
	executionCounter string
	occurrences      string
	wake             string
	lease            string
	attemptCounter   string
	resultsCreated   string

	jobPrefix             string
	executionPrefix       string
	statePrefix           string
	attemptPrefix         string
	instanceAttemptPrefix string
	resultPrefix          string
	instanceResultPrefix  string
}
