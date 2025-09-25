package messagequeue

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yaerrors"
	"github.com/gotd/td/bin"
	"github.com/gotd/td/tg"
)

// MessageJob represents a job to send a message with a certain priority.
type MessageJob struct {
	ID            uint64
	Priority      uint16
	Request       bin.Encoder
	ResultCh      chan JobResult
	Timestamp     time.Time
	IsPlaceholder bool
	TaskCount     uint
}

// JobResult represents the result of a message job execution.
type JobResult struct {
	Updates tg.UpdatesClass
	Err     yaerrors.Error
}

// Execute performs the message sending operation.
// If the job is a placeholder, it returns an empty result.
// If the job has multiple tasks, it adds empty jobs to the dispatcher.
func (j MessageJob) Execute(
	ctx context.Context,
	dispatcher *Dispatcher,
	workerID uint,
) JobResult {
	if j.IsPlaceholder {
		return JobResult{}
	}

	if j.TaskCount > 1 {
		dispatcher.AddEmptyJob(j.TaskCount - 1)
	}

	var result tg.UpdatesBox

	err := dispatcher.Client.Invoke(ctx, j.Request, &result)

	return JobResult{
		Updates: result.Updates,
		Err: yaerrors.FromError(
			http.StatusInternalServerError,
			err,
			fmt.Sprintf("worker %d: failed to send message", workerID),
		),
	}
}

// messageHeap is a thread-safe priority queue for MessageJob.
type messageHeap struct {
	jobs []MessageJob
	mu   sync.Mutex
}

// newMessageHeap creates a new instance of messageHeap.
func newMessageHeap() messageHeap {
	return messageHeap{
		jobs: make([]MessageJob, 0, HighPriorityQueueSize),
	}
}

// sort sorts the jobs in the heap based on priority and timestamp.
// Placeholders are always sorted to the end.
// Higher priority jobs come first, and for equal priority, older jobs come first.
func (h *messageHeap) sort() {
	slices.SortFunc(h.jobs, func(a, b MessageJob) int {
		if a.IsPlaceholder && b.IsPlaceholder {
			return 0
		}

		if a.IsPlaceholder {
			return 1
		}

		if a.Priority != b.Priority {
			return cmp.Compare(b.Priority, a.Priority)
		}

		switch {
		case a.Timestamp.Before(b.Timestamp):
			return 1
		case a.Timestamp.After(b.Timestamp):
			return -1
		default:
			return 0
		}
	})
}

// Push adds a new job to the heap and sorts it.
func (h *messageHeap) Push(job MessageJob) {
	h.mu.Lock()

	h.jobs = append(h.jobs, job)
	h.sort()

	h.mu.Unlock()
}

// Len returns the number of jobs in the heap.
func (h *messageHeap) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()

	return len(h.jobs)
}

// Pop removes and returns the highest priority job from the heap.
func (h *messageHeap) Pop() (MessageJob, bool) {
	if h.Len() == 0 {
		return MessageJob{}, false
	}

	h.mu.Lock()

	last := len(h.jobs) - 1
	job := h.jobs[last]
	h.jobs = h.jobs[:last]

	h.mu.Unlock()

	return job, true
}

// Delete removes a job with the specified ID from the heap.
func (h *messageHeap) Delete(id uint64) bool {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i, job := range h.jobs {
		if job.ID == id {
			h.jobs = append(h.jobs[:i], h.jobs[i+1:]...)

			return true
		}
	}

	return false
}

// DeleteFunc removes jobs that satisfy the given condition from the heap.
func (h *messageHeap) DeleteFunc(deleteFunc func(MessageJob) bool) []uint64 {
	var deletedEntries []uint64

	h.mu.Lock()

	newJobs := make([]MessageJob, 0, len(h.jobs))

	for _, job := range h.jobs {
		if deleteFunc(job) {
			deletedEntries = append(deletedEntries, job.ID)

			continue
		}

		newJobs = append(newJobs, job)
	}

	h.jobs = newJobs

	h.mu.Unlock()

	return deletedEntries
}
