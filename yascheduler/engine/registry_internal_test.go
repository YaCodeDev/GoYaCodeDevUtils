package engine

import (
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/protocol"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

const (
	ringInstance      = protocol.InstanceID("ring-exec-1")
	ringOtherInstance = protocol.InstanceID("ring-exec-2")
	ringExecutorType  = protocol.ExecutorType("worker")
	ringFunctionName  = protocol.FunctionName("report")
	ringGPULabel      = protocol.Label("gpu")
	ringShardLabel    = protocol.Label("shard-a")
	ringUnlimited     = store.Capacity(0)
)

type silentSender struct{}

func (s *silentSender) EnqueueMessage(_ protocol.Message) yaerrors.Error {
	return nil
}

func (s *silentSender) CloseConnection() {}

func newInternalRegistry(t *testing.T) *executorRegistry {
	t.Helper()

	registry, ok := NewExecutorRegistry().(*executorRegistry)
	if !ok {
		t.Fatal("the registry constructor should return the concrete registry")
	}

	return registry
}

func registerRing(
	t *testing.T,
	registry *executorRegistry,
	instanceID protocol.InstanceID,
	labels []protocol.Label,
) *ExecutorEntry {
	t.Helper()

	entry, _ := registry.Register(
		instanceID,
		ringExecutorType,
		ringUnlimited,
		[]protocol.FunctionSpec{{Name: ringFunctionName}},
		labels,
		&silentSender{},
	)

	return entry
}

// TestExecutorRegistryLabelRingsCleanedOnDeregister proves a departing or
// replaced instance stops being reachable from the label rings it announced.
// Alive() already hides a dead entry from selection and from LabelPoolSize,
// so a missed cleanup is invisible from outside the package and shows up
// only as unbounded ring growth under registration churn; this reads the
// rings directly instead.
func TestExecutorRegistryLabelRingsCleanedOnDeregister(t *testing.T) {
	t.Parallel()

	t.Run(
		"when the only holder deregisters / then the label ring is dropped",
		func(t *testing.T) {
			t.Parallel()

			registry := newInternalRegistry(t)

			entry := registerRing(
				t,
				registry,
				ringInstance,
				[]protocol.Label{ringGPULabel, ringShardLabel},
			)

			if len(registry.labelPools) != 2 {
				t.Fatalf(
					"both announced labels should hold a ring: got %d",
					len(registry.labelPools),
				)
			}

			if !registry.Deregister(ringInstance, entry.Generation()) {
				t.Fatal("the current generation should deregister")
			}

			if len(registry.labelPools) != 0 {
				t.Errorf(
					"deregistration should drop every emptied label ring: got %v",
					registry.labelPools,
				)
			}
		},
	)

	t.Run(
		"when one of two holders deregisters / then only it leaves the ring",
		func(t *testing.T) {
			t.Parallel()

			labels := []protocol.Label{ringGPULabel}

			registry := newInternalRegistry(t)

			departing := registerRing(t, registry, ringInstance, labels)
			registerRing(t, registry, ringOtherInstance, labels)

			if !registry.Deregister(ringInstance, departing.Generation()) {
				t.Fatal("the current generation should deregister")
			}

			ring, found := registry.labelPools[ringGPULabel]
			if !found {
				t.Fatal("a ring with a surviving holder should not be dropped")
			}

			if ring.Len() != 1 {
				t.Errorf("the departed instance should leave the ring: got %d entries", ring.Len())
			}

			if _, present := ring.Get(ringInstance); present {
				t.Error("the departed instance should not be reachable from the ring")
			}
		},
	)

	t.Run(
		"when an instance re-registers with fewer labels / then the stale ring is dropped",
		func(t *testing.T) {
			t.Parallel()

			registry := newInternalRegistry(t)

			registerRing(t, registry, ringInstance, []protocol.Label{ringGPULabel, ringShardLabel})
			registerRing(t, registry, ringInstance, []protocol.Label{ringGPULabel})

			if _, found := registry.labelPools[ringShardLabel]; found {
				t.Error("a label the re-registration dropped should leave no ring behind")
			}

			ring, found := registry.labelPools[ringGPULabel]
			if !found {
				t.Fatal("a replayed label should keep its ring")
			}

			if ring.Len() != 1 {
				t.Errorf("a re-registration should not duplicate the instance: got %d", ring.Len())
			}
		},
	)

	t.Run(
		"when labels are withdrawn at runtime / then the emptied ring is dropped",
		func(t *testing.T) {
			t.Parallel()

			registry := newInternalRegistry(t)

			registerRing(t, registry, ringInstance, []protocol.Label{ringGPULabel})

			if _, err := registry.UpdateLabels(
				ringInstance,
				nil,
				[]protocol.Label{ringGPULabel},
			); err != nil {
				t.Fatalf("a withdrawal should be admitted: %v", err)
			}

			if len(registry.labelPools) != 0 {
				t.Errorf(
					"a withdrawal should drop every emptied label ring: got %v",
					registry.labelPools,
				)
			}
		},
	)
}
