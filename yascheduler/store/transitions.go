package store

var executionTransitions = map[ExecutionState]map[ExecutionState]struct{}{
	StateScheduled: {
		StateReady:             {},
		StateDispatching:       {},
		StateWaitingExecutor:   {},
		StateWaitingCompatible: {},
		StateWaitingLabel:      {},
		StateCancelled:         {},
		StateSkipped:           {},
	},
	StateReady: {
		StateDispatching:       {},
		StateWaitingExecutor:   {},
		StateWaitingCompatible: {},
		StateWaitingLabel:      {},
		StateCancelled:         {},
		StateSkipped:           {},
	},
	StateWaitingExecutor: {
		StateReady:             {},
		StateDispatching:       {},
		StateWaitingCompatible: {},
		StateWaitingLabel:      {},
		StateCancelled:         {},
		StateSkipped:           {},
	},
	StateWaitingCompatible: {
		StateReady:           {},
		StateDispatching:     {},
		StateWaitingExecutor: {},
		StateWaitingLabel:    {},
		StateCancelled:       {},
		StateSkipped:         {},
	},
	StateWaitingLabel: {
		StateReady:             {},
		StateDispatching:       {},
		StateWaitingExecutor:   {},
		StateWaitingCompatible: {},
		StateCancelled:         {},
		StateSkipped:           {},
	},
	StateDispatching: {
		StateRunning:           {},
		StateReady:             {},
		StateWaitingExecutor:   {},
		StateWaitingCompatible: {},
		StateWaitingLabel:      {},
		StateCancelled:         {},
	},
	StateRunning: {
		StateSucceeded: {},
		StateFailed:    {},
		StateRetryWait: {},
		StateReady:     {},
		StateCancelled: {},
	},
	StateRetryWait: {
		StateDispatching:       {},
		StateReady:             {},
		StateWaitingExecutor:   {},
		StateWaitingCompatible: {},
		StateWaitingLabel:      {},
		StateCancelled:         {},
	},
	StateSucceeded: {},
	StateFailed:    {},
	StateCancelled: {},
	StateSkipped:   {},
}

// CanTransition reports whether an execution may move directly from one
// state to another. An unknown source or target state is never allowed.
func CanTransition(from ExecutionState, to ExecutionState) (allowed bool) {
	targets, known := executionTransitions[from]
	if !known {
		return false
	}

	_, allowed = targets[to]

	return allowed
}
