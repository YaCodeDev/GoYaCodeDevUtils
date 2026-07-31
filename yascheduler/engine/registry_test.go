package engine_test

import (
	"sync"
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/engine"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

const (
	poolExecutorType      = protocol.ExecutorType("worker")
	poolFunctionName      = protocol.FunctionName("report")
	firstInstance         = protocol.InstanceID("exec-1")
	secondInstance        = protocol.InstanceID("exec-2")
	thirdInstance         = protocol.InstanceID("exec-3")
	poolUnlimitedCapacity = store.Capacity(0)
	singleSlot            = store.Capacity(1)
	occupyingAttempt      = protocol.AttemptID(1)
)

type closeCount int

type stubSender struct {
	mu     sync.Mutex
	sent   []protocol.Message
	closes closeCount
}

func (s *stubSender) EnqueueMessage(msg protocol.Message) yaerrors.Error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.sent = append(s.sent, msg)

	return nil
}

func (s *stubSender) CloseConnection() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closes++
}

func (s *stubSender) closeCalls() closeCount {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.closes
}

func poolFunction() protocol.FunctionSpec {
	return protocol.FunctionSpec{
		Name:            poolFunctionName,
		InputSignature:  "in",
		OutputSignature: "out",
	}
}

func register(
	registry engine.ExecutorRegistry,
	instanceID protocol.InstanceID,
	capacity store.Capacity,
	functions ...protocol.FunctionSpec,
) (*engine.ExecutorEntry, *engine.ExecutorEntry) {
	return registry.Register(instanceID, poolExecutorType, capacity, functions, &stubSender{})
}

func TestExecutorRegistryRegister(t *testing.T) {
	t.Parallel()

	t.Run(
		"when the same instance registers again / then the old entry returns closed",
		func(t *testing.T) {
			t.Parallel()

			registry := engine.NewExecutorRegistry()

			first, replacedOnFirst := register(
				registry,
				firstInstance,
				poolUnlimitedCapacity,
				poolFunction(),
			)
			if replacedOnFirst != nil {
				t.Fatal("a first registration should not replace anything")
			}

			second, replaced := register(
				registry,
				firstInstance,
				poolUnlimitedCapacity,
				poolFunction(),
			)
			if replaced == nil {
				t.Fatal("a re-registration should return the replaced entry")
			}

			if replaced.Generation() != first.Generation() {
				t.Errorf(
					"the replaced entry should be the first registration: got %d, want %d",
					replaced.Generation(),
					first.Generation(),
				)
			}

			if second.Generation() <= first.Generation() {
				t.Errorf(
					"a re-registration should mint a newer generation: got %d after %d",
					second.Generation(),
					first.Generation(),
				)
			}

			if replaced.Alive() {
				t.Error("a replaced entry should be marked closed")
			}
		},
	)

	t.Run(
		"when a notify hook is set / then registering fires it with the executor type",
		func(t *testing.T) {
			t.Parallel()

			registry := engine.NewExecutorRegistry()

			var notified []protocol.ExecutorType

			registry.SetNotify(func(executorType protocol.ExecutorType) {
				notified = append(notified, executorType)
			})

			register(registry, firstInstance, poolUnlimitedCapacity, poolFunction())

			if len(notified) != 1 || notified[0] != poolExecutorType {
				t.Errorf("registration should notify once with the executor type: got %v", notified)
			}
		},
	)
}

func TestExecutorRegistryDeregister(t *testing.T) {
	t.Parallel()

	t.Run(
		"when the generation is stale / then the newer entry survives",
		func(t *testing.T) {
			t.Parallel()

			registry := engine.NewExecutorRegistry()

			first, _ := register(registry, firstInstance, poolUnlimitedCapacity, poolFunction())
			second, _ := register(registry, firstInstance, poolUnlimitedCapacity, poolFunction())

			if registry.Deregister(firstInstance, first.Generation()) {
				t.Error("a stale generation should not deregister")
			}

			entry, found := registry.Get(firstInstance)
			if !found {
				t.Fatal("the newer entry should survive a stale deregistration")
			}

			if entry.Generation() != second.Generation() {
				t.Errorf(
					"the surviving entry should be the newest: got %d, want %d",
					entry.Generation(),
					second.Generation(),
				)
			}
		},
	)

	t.Run(
		"when the generation is current / then the entry is removed",
		func(t *testing.T) {
			t.Parallel()

			registry := engine.NewExecutorRegistry()

			entry, _ := register(registry, firstInstance, poolUnlimitedCapacity, poolFunction())

			if !registry.Deregister(firstInstance, entry.Generation()) {
				t.Fatal("the current generation should deregister")
			}

			if _, found := registry.Get(firstInstance); found {
				t.Error("a deregistered instance should not be found")
			}

			if !entry.Alive() {
				return
			}

			t.Error("a deregistered entry should be marked closed")
		},
	)
}

func TestExecutorRegistrySelect(t *testing.T) {
	t.Parallel()

	t.Run(
		"when three compatible executors register / then selection rotates fairly",
		func(t *testing.T) {
			t.Parallel()

			const laps = 2

			registry := engine.NewExecutorRegistry()

			register(registry, firstInstance, poolUnlimitedCapacity, poolFunction())
			register(registry, secondInstance, poolUnlimitedCapacity, poolFunction())
			register(registry, thirdInstance, poolUnlimitedCapacity, poolFunction())

			function := poolFunction()

			for lap := range laps {
				seen := make(map[protocol.InstanceID]int, 3)

				for range 3 {
					entry, found := registry.Select(poolExecutorType, &function)
					if !found {
						t.Fatalf("lap %d should select an executor", lap)
					}

					seen[entry.InstanceID()]++
				}

				for _, instance := range []protocol.InstanceID{
					firstInstance,
					secondInstance,
					thirdInstance,
				} {
					if seen[instance] != 1 {
						t.Errorf(
							"lap %d should pick %s exactly once: got %d",
							lap,
							instance,
							seen[instance],
						)
					}
				}
			}
		},
	)

	t.Run(
		"when an entry is closed / then selection skips it",
		func(t *testing.T) {
			t.Parallel()

			const selections = 4

			registry := engine.NewExecutorRegistry()

			closedEntry, _ := register(
				registry,
				firstInstance,
				poolUnlimitedCapacity,
				poolFunction(),
			)
			register(registry, secondInstance, poolUnlimitedCapacity, poolFunction())

			closedEntry.MarkClosed()

			function := poolFunction()

			for range selections {
				entry, found := registry.Select(poolExecutorType, &function)
				if !found {
					t.Fatal("an alive executor should be selectable")
				}

				if entry.InstanceID() == firstInstance {
					t.Fatal("a closed entry should never be selected")
				}
			}
		},
	)

	t.Run(
		"when the function is missing or incompatible / then nothing is selected",
		func(t *testing.T) {
			t.Parallel()

			const unknownFunction = protocol.FunctionName("unknown")

			registry := engine.NewExecutorRegistry()

			register(registry, firstInstance, poolUnlimitedCapacity, poolFunction())

			missing := protocol.FunctionSpec{Name: unknownFunction}
			if _, found := registry.Select(poolExecutorType, &missing); found {
				t.Error("an unregistered function should not select an executor")
			}

			wrongInput := poolFunction()
			wrongInput.InputSignature = "other-in"

			if _, found := registry.Select(poolExecutorType, &wrongInput); found {
				t.Error("a mismatched input signature should not select an executor")
			}

			wrongOutput := poolFunction()
			wrongOutput.OutputSignature = "other-out"

			if _, found := registry.Select(poolExecutorType, &wrongOutput); found {
				t.Error("a mismatched output signature should not select an executor")
			}
		},
	)

	t.Run(
		"when an executor is at capacity / then selection skips it until load drops",
		func(t *testing.T) {
			t.Parallel()

			registry := engine.NewExecutorRegistry()

			entry, _ := register(registry, firstInstance, singleSlot, poolFunction())
			entry.AddInFlight(occupyingAttempt)

			function := poolFunction()

			if _, found := registry.Select(poolExecutorType, &function); found {
				t.Error("a saturated executor should not be selected")
			}

			entry.ReleaseInFlight(occupyingAttempt)

			selected, found := registry.Select(poolExecutorType, &function)
			if !found {
				t.Fatal("a drained executor should be selectable again")
			}

			if selected.InstanceID() != firstInstance {
				t.Errorf("the drained executor should be selected: got %s", selected.InstanceID())
			}
		},
	)
}

func TestExecutorRegistryPoolSize(t *testing.T) {
	t.Parallel()

	t.Run(
		"when an entry closes / then the pool counts only alive entries",
		func(t *testing.T) {
			t.Parallel()

			const alivePool = store.PoolSize(2)

			registry := engine.NewExecutorRegistry()

			closedEntry, _ := register(
				registry,
				firstInstance,
				poolUnlimitedCapacity,
				poolFunction(),
			)
			register(registry, secondInstance, poolUnlimitedCapacity, poolFunction())
			register(registry, thirdInstance, poolUnlimitedCapacity, poolFunction())

			closedEntry.MarkClosed()

			if got := registry.PoolSize(poolExecutorType); got != alivePool {
				t.Errorf(
					"the pool should count alive entries only: got %d, want %d",
					got,
					alivePool,
				)
			}
		},
	)
}

func TestExecutorRegistryCloseAll(t *testing.T) {
	t.Parallel()

	t.Run(
		"when the registry closes / then every entry closes its connection",
		func(t *testing.T) {
			t.Parallel()

			const singleClose = closeCount(1)

			registry := engine.NewExecutorRegistry()

			firstSender := &stubSender{}
			secondSender := &stubSender{}

			firstEntry, _ := registry.Register(
				firstInstance,
				poolExecutorType,
				poolUnlimitedCapacity,
				[]protocol.FunctionSpec{poolFunction()},
				firstSender,
			)
			secondEntry, _ := registry.Register(
				secondInstance,
				poolExecutorType,
				poolUnlimitedCapacity,
				[]protocol.FunctionSpec{poolFunction()},
				secondSender,
			)

			registry.CloseAll()

			if firstEntry.Alive() || secondEntry.Alive() {
				t.Error("closing the registry should mark every entry closed")
			}

			if firstSender.closeCalls() != singleClose || secondSender.closeCalls() != singleClose {
				t.Errorf(
					"closing the registry should close each connection once: got %d and %d",
					firstSender.closeCalls(),
					secondSender.closeCalls(),
				)
			}
		},
	)
}
