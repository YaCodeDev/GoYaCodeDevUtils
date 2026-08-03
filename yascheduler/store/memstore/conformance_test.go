package memstore_test

import (
	"testing"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store/memstore"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yascheduler/store/storetest"
)

func TestConformance(t *testing.T) {
	t.Parallel()

	storetest.TestStore(t, func(t *testing.T) store.Store {
		t.Helper()

		return memstore.NewStore(memstore.Config{})
	})
}

func TestConformanceResultCaps(t *testing.T) {
	t.Parallel()

	storetest.TestResultCaps(t, func(t *testing.T, caps storetest.Caps) store.Store {
		t.Helper()

		return memstore.NewStore(memstore.Config{
			MaxResults:            caps.MaxResults,
			MaxResultsPerInstance: caps.MaxResultsPerInstance,
		})
	})
}
