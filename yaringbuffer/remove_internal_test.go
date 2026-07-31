package yaringbuffer

import "testing"

const (
	removeCapacity = 4
	headKey        = "head"
	tailKey        = "tail"
)

// TestRemoveReleasesVacatedSlot proves Remove stops the backing array from
// holding the entry it just dropped. Reslicing alone shortens the length
// while the vacated slot keeps its key and value, and the garbage collector
// scans the whole array, so a removed executor connection would stay
// reachable until a later Upsert happened to overwrite that slot.
func TestRemoveReleasesVacatedSlot(t *testing.T) {
	t.Parallel()

	ring := New[string, *int](removeCapacity)

	head, tail := new(int), new(int)

	ring.Upsert(headKey, head)
	ring.Upsert(tailKey, tail)

	if !ring.Remove(tailKey) {
		t.Fatal("remove should report the entry it dropped")
	}

	vacated := ring.entries[:len(ring.entries)+1][len(ring.entries)]

	if vacated.value != nil {
		t.Fatal("removed value stays reachable from the backing array")
	}

	if vacated.key != "" {
		t.Fatalf("removed key %q stays reachable from the backing array", vacated.key)
	}

	if ring.Len() != 1 {
		t.Fatalf("length = %d, want 1", ring.Len())
	}

	value, found := ring.Get(headKey)
	if !found || value != head {
		t.Fatal("remove disturbed the entry that stayed in the ring")
	}
}
