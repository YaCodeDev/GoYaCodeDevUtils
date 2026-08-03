package storetest

import (
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
)

type (
	// Factory builds one fresh, empty store under test. The suite calls it
	// once per subtest, so no state leaks from one subtest into another.
	Factory func(t *testing.T) store.Store

	// BoundedFactory builds one fresh store whose pending-result storage
	// honors the given caps, the way memstore wires Caps fields into its
	// own Config. A zero cap field keeps the implementation default.
	BoundedFactory func(t *testing.T, caps Caps) store.Store
)

// Caps bounds the pending-result storage of a store the cap subtests
// drive. MaxResults caps the results held in total; MaxResultsPerInstance
// caps the results held for one submitting instance.
type Caps struct {
	MaxResults            store.OccurrenceCount
	MaxResultsPerInstance store.OccurrenceCount
}
