package storetest

import "testing"

// TestStore runs the whole conformance suite short of the cap subtests
// against stores the factory builds. Run TestResultCaps beside it with a
// BoundedFactory to cover the pending-result caps.
func TestStore(t *testing.T, factory Factory) {
	t.Helper()

	TestJobRepository(t, factory)
	TestExecutionRepository(t, factory)
	TestAttemptRepository(t, factory)
	TestResultRepository(t, factory)
}
