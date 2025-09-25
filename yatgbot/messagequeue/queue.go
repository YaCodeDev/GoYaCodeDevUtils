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

type Dispatcher struct {
	Client               yatgclient.Client
	priorityQueueChannel chan MessageJob
	heap                 messageHeap
	cond                 sync.Cond
	log                  yalogger.Logger
}

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

func (d *Dispatcher) DeleteJob(id uint64) bool {
	return d.heap.Delete(id)
}

func (d *Dispatcher) DeleteJobFunc(deleteFunc func(MessageJob) bool) []uint64 {
	return d.heap.DeleteFunc(deleteFunc)
}

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

func (d *Dispatcher) AddEmptyJob(count uint) {
	for range count {
		d.heap.Push(MessageJob{
			IsPlaceholder: true,
		})
	}
}

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

func (d *Dispatcher) SendMessage(
	req *tg.MessagesSendMessageRequest,
	priority uint16,
) (uint64, <-chan JobResult) {
	if req.RandomID == 0 {
		req.RandomID = rand.Int63()
	}

	return d.AddRawJob(req, priority, SingleMessage)
}

func (d *Dispatcher) SendMultiMedia(
	req *tg.MessagesSendMultiMediaRequest,
	priority uint16,
) (uint64, <-chan JobResult) {
	for i := range req.MultiMedia {
		req.MultiMedia[i].RandomID = rand.Int63()
	}

	return d.AddRawJob(req, priority, uint(len(req.MultiMedia)))
}
