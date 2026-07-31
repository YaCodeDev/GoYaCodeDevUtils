package store_test

import (
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

const unknownState = store.ExecutionState(99)

var allExecutionStates = []store.ExecutionState{
	store.StateScheduled,
	store.StateReady,
	store.StateWaitingExecutor,
	store.StateWaitingCompatible,
	store.StateWaitingLabel,
	store.StateDispatching,
	store.StateRunning,
	store.StateRetryWait,
	store.StateSucceeded,
	store.StateFailed,
	store.StateCancelled,
	store.StateSkipped,
}

var legalTransitions = map[store.ExecutionState][]store.ExecutionState{
	store.StateScheduled: {
		store.StateReady,
		store.StateDispatching,
		store.StateWaitingExecutor,
		store.StateWaitingCompatible,
		store.StateWaitingLabel,
		store.StateCancelled,
		store.StateSkipped,
	},
	store.StateReady: {
		store.StateDispatching,
		store.StateWaitingExecutor,
		store.StateWaitingCompatible,
		store.StateWaitingLabel,
		store.StateCancelled,
		store.StateSkipped,
	},
	store.StateWaitingExecutor: {
		store.StateReady,
		store.StateDispatching,
		store.StateWaitingCompatible,
		store.StateWaitingLabel,
		store.StateCancelled,
		store.StateSkipped,
	},
	store.StateWaitingCompatible: {
		store.StateReady,
		store.StateDispatching,
		store.StateWaitingExecutor,
		store.StateWaitingLabel,
		store.StateCancelled,
		store.StateSkipped,
	},
	store.StateWaitingLabel: {
		store.StateReady,
		store.StateDispatching,
		store.StateWaitingExecutor,
		store.StateWaitingCompatible,
		store.StateCancelled,
		store.StateSkipped,
	},
	store.StateDispatching: {
		store.StateRunning,
		store.StateReady,
		store.StateWaitingExecutor,
		store.StateWaitingCompatible,
		store.StateWaitingLabel,
		store.StateCancelled,
	},
	store.StateRunning: {
		store.StateSucceeded,
		store.StateFailed,
		store.StateRetryWait,
		store.StateReady,
		store.StateCancelled,
	},
	store.StateRetryWait: {
		store.StateDispatching,
		store.StateReady,
		store.StateWaitingExecutor,
		store.StateWaitingCompatible,
		store.StateWaitingLabel,
		store.StateCancelled,
	},
	store.StateSucceeded: {},
	store.StateFailed:    {},
	store.StateCancelled: {},
	store.StateSkipped:   {},
}

func isLegalTransition(from, to store.ExecutionState) bool {
	for _, target := range legalTransitions[from] {
		if target == to {
			return true
		}
	}

	return false
}

func TestCanTransition(t *testing.T) {
	t.Parallel()

	t.Run(
		"when every state pair is checked / then exactly the table transitions are allowed",
		func(t *testing.T) {
			t.Parallel()

			for _, from := range allExecutionStates {
				for _, to := range allExecutionStates {
					want := isLegalTransition(from, to)

					if got := store.CanTransition(from, to); got != want {
						t.Errorf(
							"transition %s -> %s should report %t, got %t",
							from,
							to,
							want,
							got,
						)
					}
				}
			}
		},
	)

	t.Run(
		"when the from state is unknown / then no transition is allowed",
		func(t *testing.T) {
			t.Parallel()

			for _, to := range allExecutionStates {
				if store.CanTransition(unknownState, to) {
					t.Errorf("an unknown from state should never transition to %s", to)
				}
			}
		},
	)

	t.Run(
		"when the target state is unknown / then the transition is rejected",
		func(t *testing.T) {
			t.Parallel()

			for _, from := range allExecutionStates {
				if store.CanTransition(from, unknownState) {
					t.Errorf("%s should never transition to an unknown state", from)
				}
			}
		},
	)
}

func TestWaitingLabelTransitions(t *testing.T) {
	t.Parallel()

	t.Run(
		"when a state may reach waiting executor / then it may also reach waiting label",
		func(t *testing.T) {
			t.Parallel()

			for _, from := range allExecutionStates {
				if from == store.StateWaitingLabel || from == store.StateWaitingExecutor {
					continue
				}

				toExecutor := store.CanTransition(from, store.StateWaitingExecutor)

				if got := store.CanTransition(from, store.StateWaitingLabel); got != toExecutor {
					t.Errorf(
						"%s -> waiting_label should mirror %s -> waiting_executor (%t), got %t",
						from,
						from,
						toExecutor,
						got,
					)
				}
			}
		},
	)

	t.Run(
		"when waiting label leaves / then it mirrors the waiting executor targets",
		func(t *testing.T) {
			t.Parallel()

			for _, to := range allExecutionStates {
				if to == store.StateWaitingLabel || to == store.StateWaitingExecutor {
					continue
				}

				fromExecutor := store.CanTransition(store.StateWaitingExecutor, to)

				got := store.CanTransition(store.StateWaitingLabel, to)
				if got != fromExecutor {
					t.Errorf(
						"waiting_label -> %s should mirror waiting_executor -> %s (%t), got %t",
						to,
						to,
						fromExecutor,
						got,
					)
				}
			}
		},
	)

	t.Run(
		"when waiting label and its siblings are paired / then both directions are legal",
		func(t *testing.T) {
			t.Parallel()

			pairs := [][2]store.ExecutionState{
				{store.StateWaitingLabel, store.StateWaitingExecutor},
				{store.StateWaitingExecutor, store.StateWaitingLabel},
				{store.StateWaitingLabel, store.StateWaitingCompatible},
				{store.StateWaitingCompatible, store.StateWaitingLabel},
			}

			for _, pair := range pairs {
				if !store.CanTransition(pair[0], pair[1]) {
					t.Errorf("%s -> %s should be legal", pair[0], pair[1])
				}
			}
		},
	)

	t.Run(
		"when waiting label is settled / then it is neither terminal nor self-directed",
		func(t *testing.T) {
			t.Parallel()

			if store.StateWaitingLabel.Terminal() {
				t.Error("waiting_label should not be terminal")
			}

			if store.CanTransition(store.StateWaitingLabel, store.StateWaitingLabel) {
				t.Error("waiting_label should not transition to itself")
			}

			if store.CanTransition(store.StateWaitingLabel, store.StateRunning) {
				t.Error("waiting_label should not jump straight to running")
			}
		},
	)
}

func TestExecutionStateTerminal(t *testing.T) {
	t.Parallel()

	t.Run(
		"when each state is checked / then only settled states are terminal",
		func(t *testing.T) {
			t.Parallel()

			terminalStates := map[store.ExecutionState]bool{
				store.StateSucceeded: true,
				store.StateFailed:    true,
				store.StateCancelled: true,
				store.StateSkipped:   true,
			}

			for _, state := range allExecutionStates {
				want := terminalStates[state]

				if got := state.Terminal(); got != want {
					t.Errorf("%s terminality should be %t, got %t", state, want, got)
				}
			}
		},
	)

	t.Run(
		"when the state is unknown / then it is not terminal",
		func(t *testing.T) {
			t.Parallel()

			if unknownState.Terminal() {
				t.Error("an unknown state should not be terminal")
			}
		},
	)
}

func TestExecutionStateString(t *testing.T) {
	t.Parallel()

	t.Run(
		"when each state is rendered / then every known state has its own name",
		func(t *testing.T) {
			t.Parallel()

			const unknownName = "unknown"

			names := map[store.ExecutionState]string{
				store.StateScheduled:         "scheduled",
				store.StateReady:             "ready",
				store.StateWaitingExecutor:   "waiting_executor",
				store.StateWaitingCompatible: "waiting_compatible",
				store.StateWaitingLabel:      "waiting_label",
				store.StateDispatching:       "dispatching",
				store.StateRunning:           "running",
				store.StateRetryWait:         "retry_wait",
				store.StateSucceeded:         "succeeded",
				store.StateFailed:            "failed",
				store.StateCancelled:         "cancelled",
				store.StateSkipped:           "skipped",
			}

			for _, state := range allExecutionStates {
				want, known := names[state]
				if !known {
					t.Fatalf("state %d is missing an expected name", uint8(state))
				}

				if got := state.String(); got != want {
					t.Errorf("state name should be %q, got %q", want, got)
				}

				if state.String() == unknownName {
					t.Errorf("a known state should not render as %q", unknownName)
				}
			}
		},
	)

	t.Run(
		"when the state is unknown / then it renders as unknown",
		func(t *testing.T) {
			t.Parallel()

			const unknownName = "unknown"

			if got := unknownState.String(); got != unknownName {
				t.Errorf("an unknown state should render as %q, got %q", unknownName, got)
			}
		},
	)
}
