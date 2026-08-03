package yascheduler

import (
	"context"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
)

// invocationContextKey keys the Invocation the runtime stores on every
// execution context; an unexported type keeps the entry collision-free
// with values other packages carry.
type invocationContextKey struct{}

// Invocation identifies one running function invocation: the stable
// identities the scheduler dispatched it under and the function reference
// the request carried. Handlers that cause external effects use
// ExecutionID (stable across redispatches of the same occurrence) as an
// idempotency key.
type Invocation struct {
	JobUUID       protocol.JobUUID
	ExecutionID   protocol.ExecutionID
	AttemptID     protocol.AttemptID
	AttemptNumber uint32
	Function      protocol.FunctionSpec
}

// InvocationFromContext returns the Invocation of the running function
// and reports whether ctx carries one. Only a context handed to a
// registered function by this package's runtime answers found; any other
// context answers false.
func InvocationFromContext(ctx context.Context) (invocation *Invocation, found bool) {
	invocation, found = ctx.Value(invocationContextKey{}).(*Invocation)

	return invocation, found
}
