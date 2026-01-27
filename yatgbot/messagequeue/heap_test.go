package messagequeue

import (
	"sync"
	"testing"
	"time"
)

func mustPop(t *testing.T, h *messageHeap) MessageJob {
	t.Helper()

	job, ok := h.Pop()
	if !ok {
		t.Fatalf("expected job, heap is empty")
	}

	return job
}

func TestHeap_PushPopOrdering(t *testing.T) {
	h := newMessageHeap(&sync.Mutex{})
	now := time.Now()

	// ID 3: highest priority (1) and *oldest* timestamp
	h.Push(MessageJob{ID: 3, Priority: 1, Timestamp: now.Add(-2 * time.Minute)})
	// ID 2: same priority 1 but newer than ID 3
	h.Push(MessageJob{ID: 2, Priority: 1, Timestamp: now})
	// ID 1: lower priority (2)
	h.Push(MessageJob{ID: 1, Priority: 2, Timestamp: now})
	// ID 4: placeholder – always first
	h.Push(MessageJob{ID: 4, IsPlaceholder: true})

	if h.Len() != 4 {
		t.Fatalf("expected heap len 4, got %d", h.Len())
	}

	wantOrder := []uint64{4, 3, 2, 1}
	for i, wantID := range wantOrder {
		got := mustPop(t, &h)
		if got.ID != wantID {
			t.Fatalf("pop #%d: want ID %d, got %d", i+1, wantID, got.ID)
		}
	}

	if h.Len() != 0 {
		t.Fatalf("expected empty heap, len=%d", h.Len())
	}
}

func TestHeap_DeleteByID(t *testing.T) {
	h := newMessageHeap(&sync.Mutex{})
	h.Push(MessageJob{ID: 10})
	h.Push(MessageJob{ID: 20})

	if !h.Delete(10) {
		t.Fatalf("Delete should return true for existing ID")
	}

	if h.Len() != 1 {
		t.Fatalf("expected len 1 after delete, got %d", h.Len())
	}

	if h.Delete(42) {
		t.Fatalf("Delete should return false for missing ID")
	}
}

func TestHeap_DeleteFunc(t *testing.T) {
	h := newMessageHeap(&sync.Mutex{})

	h.Push(MessageJob{ID: 1, Priority: 5})
	h.Push(MessageJob{ID: 2, Priority: 3})
	h.Push(MessageJob{ID: 3, Priority: 1})

	deleted := h.DeleteFunc(func(j MessageJob) bool { return j.Priority < 4 })
	if len(deleted) != 2 {
		t.Fatalf("expected 2 jobs deleted, got %d", len(deleted))
	}

	if h.Len() != 1 {
		t.Fatalf("expected heap len 1 after DeleteFunc, got %d", h.Len())
	}

	if deleted[0] == deleted[1] {
		t.Fatalf("deleted IDs should be unique, got %+v", deleted)
	}
}

func TestHeap_PopOnEmpty(t *testing.T) {
	h := newMessageHeap(&sync.Mutex{})
	if _, ok := h.Pop(); ok {
		t.Fatalf("expected ok==false on empty Pop")
	}
}
