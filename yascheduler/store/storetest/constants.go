package storetest

import (
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

const (
	baseUnixSeconds = 1_700_000_000
	keyLengthCap    = 255

	suiteExecutorType = protocol.ExecutorType("worker")
	otherExecutorType = protocol.ExecutorType("mailer")
	suiteFunctionName = protocol.FunctionName("report")
	suiteInstanceID   = protocol.InstanceID("exec-1")
	otherInstanceID   = protocol.InstanceID("exec-2")

	noExclusion  = protocol.ExecutionID(0)
	firstAttempt = store.AttemptNumber(1)
	unlimited    = store.BatchLimit(0)

	firstVersion  = store.Version(1)
	secondVersion = store.Version(2)
)
