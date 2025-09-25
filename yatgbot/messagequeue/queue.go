package messagequeue

import (
	"context"
	"math/rand"
	"sync"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
	"github.com/YaCodeDev/GoYaCodeDevUtils/yatgclient"
	"github.com/gotd/td/bin"
	"github.com/gotd/td/tg"
)

// Dispatcher handles message sending with priority and concurrency control.
type Dispatcher struct {
	Client               yatgclient.Client
	priorityQueueChannel chan MessageJob
	heap                 messageHeap
	cond                 sync.Cond
	log                  yalogger.Logger
}

// NewDispatcher creates a new Dispatcher with the given number of workers.
// Each worker processes jobs from the priority queue channel.
// The dispatcher uses a condition variable to signal when new jobs are added to the heap.
// It also initializes the message heap and starts the worker goroutines.
//
// Example of usage:
//
//	dispatcher := NewDispatcher(ctx, 5, log)
func NewDispatcher(
	ctx context.Context,
	workerCount uint,
	log yalogger.Logger,
) *Dispatcher {
	dispatcher := &Dispatcher{
		priorityQueueChannel: make(chan MessageJob),
		log:                  log,
		heap:                 newMessageHeap(),
	}

	go dispatcher.proccessMessagesQueue()

	for i := range workerCount {
		go dispatcher.worker(ctx, i)
	}

	return dispatcher
}

// proccessMessagesQueue continuously processes jobs from the heap and sends them to the priority queue channel.
// It waits for new jobs if the heap is empty.
func (d *Dispatcher) proccessMessagesQueue() {
	for {
		if d.heap.Len() == 0 {
			d.cond.L.Lock()
			d.cond.Wait()
			d.cond.L.Unlock()
		}

		job, ok := d.heap.Pop()
		if !ok {
			continue
		}

		d.priorityQueueChannel <- job
	}
}

// worker processes jobs from the priority queue channel.
// It executes each job and sends the result back through the job's ResultCh.
func (d *Dispatcher) worker(ctx context.Context, id uint) {
	for {
		select {
		case job := <-d.priorityQueueChannel:
			start := time.Now()

			err := job.Execute(ctx, d, id)

			select {
			case job.ResultCh <- err:
			case <-ctx.Done():
				return
			}

			time.Sleep(time.Second - time.Since(start))

		case <-ctx.Done():
			return
		}
	}
}

// DeleteJob removes a job from the heap by its ID.
// Returns true if the job was found and deleted, false otherwise.
//
// Example of usage:
//
// deleted := dispatcher.DeleteJob(jobID)
//
//	if !deleted {
//	    // Handle job not found
//	}
func (d *Dispatcher) DeleteJob(id uint64) bool {
	return d.heap.Delete(id)
}

// DeleteJobFunc removes jobs from the heap that satisfy the given condition.
//
// Example of usage:
//
//	deletedIDs := dispatcher.DeleteJobFunc(func(job MessageJob) bool {
//		    return job.Priority < 10
//		})
//
//		for _, id := range deletedIDs {
//		    // Handle deleted job ID
//		}
func (d *Dispatcher) DeleteJobFunc(deleteFunc func(MessageJob) bool) []uint64 {
	return d.heap.DeleteFunc(deleteFunc)
}

// AddRawJob adds a raw job to the dispatcher with the specified request, priority, and task count.
// It returns the job ID and a channel to receive the job result.
//
// Example of usage:
//
// jobID, resultCh := dispatcher.AddRawJob(request, priority, taskCount)
//
// // Wait for the job result
// result := <-resultCh
//
//	if result.Err != nil {
//	    // Handle job error
//	}
func (d *Dispatcher) AddRawJob(
	request bin.Encoder,
	priority uint16,
	taskCount uint,
) (uint64, <-chan JobResult) {
	job := MessageJob{
		ID:        rand.Uint64(),
		Priority:  priority,
		Request:   request,
		ResultCh:  make(chan JobResult, 1),
		Timestamp: time.Now(),
		TaskCount: taskCount,
	}

	d.heap.Push(job)

	d.cond.Signal()

	return job.ID, job.ResultCh
}

// AddEmptyJob adds the specified number of placeholder jobs to the dispatcher.
//
// Example of usage:
//
// dispatcher.AddEmptyJob(5) // Adds 5 placeholder jobs
func (d *Dispatcher) AddEmptyJob(count uint) {
	for range count {
		d.heap.Push(MessageJob{
			IsPlaceholder: true,
		})
	}
}

// AddMessagesForfard adds a message forwarding job to the dispatcher.
//
// Example of usage:
//
// jobID, resultCh := dispatcher.AddMessagesForward(messagesForwardMessagesRequest, priority)
//
// // Wait for the job result
// result := <-resultCh
//
//	if result.Err != nil {
//	    // Handle job error
//	}
func (d *Dispatcher) AddMessagesForward(
	req *tg.MessagesForwardMessagesRequest,
	priority uint16,
) (uint64, <-chan JobResult) {
	req.RandomID = make([]int64, len(req.ID))
	for i := range req.RandomID {
		req.RandomID[i] = rand.Int63()
	}

	return d.AddRawJob(req, priority, uint(len(req.RandomID)))
}

// SendMessage adds a message sending job to the dispatcher.
//
// Example of usage:
//
// jobID, resultCh := dispatcher.SendMessage(messagesSendMessageRequest, priority)
//
// // Wait for the job result
// result := <-resultCh
//
//	if result.Err != nil {
//	    // Handle job error
//	}
func (d *Dispatcher) SendMessage(
	req *tg.MessagesSendMessageRequest,
	priority uint16,
) (uint64, <-chan JobResult) {
	if req.RandomID == 0 {
		req.RandomID = rand.Int63()
	}

	return d.AddRawJob(req, priority, SingleMessage)
}

// SendMedia adds a media sending job to the dispatcher.
//
// Example of usage:
//
// jobID, resultCh := dispatcher.SendMedia(messagesSendMediaRequest, priority)
//
// // Wait for the job result
// result := <-resultCh
//
//	if result.Err != nil {
//	    // Handle job error
//	}
func (d *Dispatcher) SendMultiMedia(
	req *tg.MessagesSendMultiMediaRequest,
	priority uint16,
) (uint64, <-chan JobResult) {
	for i := range req.MultiMedia {
		req.MultiMedia[i].RandomID = rand.Int63()
	}

	return d.AddRawJob(req, priority, uint(len(req.MultiMedia)))
}
