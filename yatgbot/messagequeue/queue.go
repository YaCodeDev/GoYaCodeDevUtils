package messagequeue

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"github.com/YaCodeDev/GoYaCodeDevUtils/yalogger"
)

type Dispatcher struct {
	mu                   sync.Mutex // maybe delete
	inputChannel         chan MessageJob
	priorityQueueChannel chan MessageJob
	log                  yalogger.Logger
}

func NewDispatcher(
	ctx context.Context,
	workerCount uint,
	inputChannelCap uint,
	priorityChannelCap uint,
	log yalogger.Logger,
) *Dispatcher {
	dispatcher := &Dispatcher{
		inputChannel:         make(chan MessageJob, inputChannelCap),
		priorityQueueChannel: make(chan MessageJob, priorityChannelCap),
		log:                  log,
	}

	go dispatcher.reorderMessages(ctx)

	for i := uint(0); i < workerCount; i++ {
		go dispatcher.worker(ctx, i)
	}

	return dispatcher
}

func (d *Dispatcher) reorderMessages(ctx context.Context) {

	var pq messageHeap

	heap.Init(&pq)

	for {
		select {
		case job := <-d.inputChannel:
			heap.Push(&pq, job)
		case <-ctx.Done():
			return
		}

		if pq.Len() > 0 {
			job := heap.Pop(&pq).(MessageJob)
			d.priorityQueueChannel <- job
		}
	}
}

func (d *Dispatcher) worker(ctx context.Context, id uint) {
	for {
		select {
		case job := <-d.priorityQueueChannel:
			start := time.Now()

			if job.Markup == nil {
				if _, err := job.Sender.To(job.To).Text(ctx, job.Text); err != nil {
					d.log.Infof("[worker %d] error sending message without markup: %v", id, err)
				}
			} else {
				if _, err := job.Sender.To(job.To).Markup(job.Markup).Text(ctx, job.Text); err != nil {
					d.log.Infof("[worker %d] error sending message with markup: %v", id, err)
				}
			}

			time.Sleep(time.Second - time.Since(start))

		case <-ctx.Done():
			return
		}
	}
}

func (d *Dispatcher) Add(job MessageJob) {
	d.inputChannel <- job
}
